package main

import (
	"context"
	"embed"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

const (
	petWidth  = 300
	petHeight = 380
)

func main() {
	store := NewConfigStore()
	svc := &PetService{store: store}
	svc.tracker = NewActivityTracker(store, func(r Remark) { go svc.say(r) })

	app := application.New(application.Options{
		Name:        "pcserve",
		Description: "桌面三维宠物",
		Services:    []application.Service{application.NewService(svc)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.pcserve.pet",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				svc.pet.Show()
			},
		},
	})
	svc.app = app

	svc.pet = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "pet",
		Title:            "桌面宠物",
		Width:            petWidth,
		Height:           petHeight,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		Hidden:           true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		URL:              "/?view=pet",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
		},
	})

	svc.settings = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "settings",
		Title:            "宠物设置",
		Width:            520,
		Height:           720,
		Hidden:           true,
		BackgroundColour: application.NewRGB(250, 248, 245),
		URL:              "/?view=settings",
	})

	for _, w := range []*application.WebviewWindow{svc.pet, svc.settings} {
		w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			w.Hide()
			e.Cancel()
		})
	}

	var saveTimer *time.Timer
	svc.pet.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) {
		if saveTimer != nil {
			saveTimer.Stop()
		}
		saveTimer = time.AfterFunc(500*time.Millisecond, func() { _ = svc.SavePetPosition() })
	})

	svc.onChange = setupTray(app, svc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		placePet(svc)
		svc.pet.Show()
		go svc.tracker.Run(ctx)
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func placePet(svc *PetService) {
	cfg := svc.store.Get()
	if cfg.PetX != nil && cfg.PetY != nil && onAnyScreen(svc.app, *cfg.PetX, *cfg.PetY) {
		svc.pet.SetPosition(*cfg.PetX, *cfg.PetY)
		return
	}
	wa := svc.GetWorkArea()
	svc.pet.SetPosition(wa.X+wa.Width-petWidth-40, wa.Y+wa.Height-petHeight)
}

func onAnyScreen(app *application.App, x, y int) bool {
	cx, cy := x+petWidth/2, y+petHeight/2
	for _, s := range app.Screen.GetAll() {
		wa := s.WorkArea
		if cx >= wa.X && cx < wa.X+wa.Width && cy >= wa.Y && cy < wa.Y+wa.Height {
			return true
		}
	}
	return false
}

func setupTray(app *application.App, svc *PetService) func(Config) {
	cfg := svc.store.Get()
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("桌面宠物 · " + cfg.Name)

	menu := app.NewMenu()
	menu.Add("显示 / 隐藏宠物").OnClick(func(*application.Context) {
		if svc.pet.IsVisible() {
			svc.pet.Hide()
		} else {
			svc.pet.Show()
		}
	})
	menu.Add("和它聊天").OnClick(func(*application.Context) {
		svc.pet.Show()
		svc.pet.Focus()
		app.Event.Emit(EventOpenChat)
	})
	menu.AddSeparator()

	petMenu := menu.AddSubmenu("换宠物")
	petItems := map[string]*application.MenuItem{}
	for _, id := range []string{"cat", "dog", "parrot"} {
		petItems[id] = petMenu.AddRadio(petNames[id], cfg.Pet == id).OnClick(func(*application.Context) {
			_ = svc.SetPet(id)
		})
	}
	muteItem := menu.AddCheckbox("静音", cfg.Muted).OnClick(func(ctx *application.Context) {
		_ = svc.SetMuted(ctx.ClickedMenuItem().Checked())
	})
	menu.Add("设置…").OnClick(func(*application.Context) { svc.OpenSettings() })
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) {
		_ = svc.SavePetPosition()
		app.Quit()
	})

	tray.SetMenu(menu)
	tray.OnClick(func() {
		svc.pet.Show()
		svc.pet.Focus()
	})

	return func(c Config) {
		for id, item := range petItems {
			item.SetChecked(c.Pet == id)
		}
		muteItem.SetChecked(c.Muted)
		menu.Update()
		tray.SetTooltip("桌面宠物 · " + c.Name)
	}
}
