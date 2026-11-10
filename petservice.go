package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	EventConfigChanged = "config:changed"
	EventSay           = "pet:say"
	EventOpenChat      = "pet:chat"
)

type Say struct {
	Text string `json:"text"`
	Kind string `json:"kind"`
}

// HitRect is in physical pixels relative to the pet window's top-left corner.
type HitRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type WorkArea struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// PetService is bound to the frontend; every exported method is callable from Vue.
type PetService struct {
	store    *ConfigStore
	tracker  *ActivityTracker
	app      *application.App
	pet      *application.WebviewWindow
	settings *application.WebviewWindow
	onChange func(Config)
	walkGen  atomic.Int64
}

const walkSpeed = 70.0 // pixels per second

func (s *PetService) GetConfig() Config {
	return s.store.Get()
}

func (s *PetService) SaveConfig(next Config) (Config, error) {
	prev := s.store.Get()
	cfg, err := s.store.Update(func(c *Config) {
		petX, petY := c.PetX, c.PetY
		*c = next
		c.PetX, c.PetY = petX, petY
	})
	if err != nil {
		return cfg, err
	}
	if cfg.AutoStart != prev.AutoStart {
		if err := setAutoStart(cfg.AutoStart); err != nil {
			return cfg, fmt.Errorf("开机自启设置失败：%w", err)
		}
	}
	s.broadcast(cfg)
	return cfg, nil
}

func (s *PetService) SetPet(pet string) error {
	if _, ok := petNames[pet]; !ok {
		return fmt.Errorf("未知宠物：%s", pet)
	}
	cfg, err := s.store.Update(func(c *Config) { c.Pet = pet })
	if err != nil {
		return err
	}
	s.broadcast(cfg)
	return nil
}

func (s *PetService) SetMuted(muted bool) error {
	cfg, err := s.store.Update(func(c *Config) { c.Muted = muted })
	if err != nil {
		return err
	}
	s.broadcast(cfg)
	return nil
}

func (s *PetService) Chat(text string) string {
	cfg := s.store.Get()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	reply, err := askLLM(ctx, cfg, text)
	if errors.Is(err, errNoLLM) {
		return fallbackFor(cfg, "chat", 0)
	}
	if err != nil {
		return "呜，我脑子卡住了：" + truncate(err.Error(), 40)
	}
	return reply
}

func (s *PetService) TestLLM(cfg Config) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cfg = normalize(cfg)
	return askLLM(ctx, cfg, userTitle(cfg)+"刚给你接上了大脑，打个招呼吧。")
}

func (s *PetService) PetLine() string {
	return fallbackFor(s.store.Get(), "pet", 0)
}

// Speak returns base64 mp3 for text, or "" when voice is off or muted.
func (s *PetService) Speak(text string) (string, error) {
	cfg := s.store.Get()
	if !cfg.Voice.Enabled || cfg.Muted {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	audio, err := synthesize(ctx, cfg.Voice, text)
	if err != nil || len(audio) == 0 {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(audio), nil
}

// TestVoice synthesizes a sample with unsaved settings so the user can preview the voice.
func (s *PetService) TestVoice(cfg Config) (string, error) {
	cfg = normalize(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	text := "你好呀，以后我就用这个声音陪你啦。"
	if cfg.Lover.Enabled {
		text = cfg.Lover.CallMe + "，以后我就用这个声音陪着你，好不好？"
	}
	audio, err := synthesize(ctx, cfg.Voice, text)
	if errors.Is(err, errNoVoice) {
		return "", errors.New("请先填写 API Key 和音色 ID")
	}
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(audio), nil
}

func (s *PetService) Snooze(minutes int) {
	if minutes <= 0 {
		minutes = 10
	}
	s.tracker.Snooze(time.Duration(minutes) * time.Minute)
}

func (s *PetService) WorkMinutes() int {
	return s.tracker.WorkMinutes()
}

func (s *PetService) GetWorkArea() WorkArea {
	if s.pet != nil {
		if scr, err := s.pet.GetScreen(); err == nil && scr != nil {
			wa := scr.WorkArea
			return WorkArea{wa.X, wa.Y, wa.Width, wa.Height}
		}
	}
	if scr := s.app.Screen.GetPrimary(); scr != nil {
		wa := scr.WorkArea
		return WorkArea{wa.X, wa.Y, wa.Width, wa.Height}
	}
	return WorkArea{0, 0, 1920, 1040}
}

func (s *PetService) SavePetPosition() error {
	x, y := s.pet.Position()
	_, err := s.store.Update(func(c *Config) { c.PetX, c.PetY = &x, &y })
	return err
}

func (s *PetService) SetHitRegion(rects []HitRect) {
	hwnd := uintptr(s.pet.NativeWindow())
	application.InvokeAsync(func() { setHitRegion(hwnd, rects) })
}

// Walk moves the pet window horizontally to targetX and returns false if interrupted.
func (s *PetService) Walk(targetX int) bool {
	gen := s.walkGen.Add(1)
	x, y := s.pet.Position()
	dir := 1.0
	if targetX < x {
		dir = -1
	}
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()
	start := time.Now()
	for range ticker.C {
		if s.walkGen.Load() != gen {
			return false
		}
		next := float64(x) + dir*walkSpeed*time.Since(start).Seconds()
		if (dir > 0 && next >= float64(targetX)) || (dir < 0 && next <= float64(targetX)) {
			s.pet.SetPosition(targetX, y)
			return true
		}
		s.pet.SetPosition(int(next), y)
	}
	return false
}

func (s *PetService) StopWalk() {
	s.walkGen.Add(1)
}

func (s *PetService) StartDrag() {
	s.StopWalk()
	hwnd := uintptr(s.pet.NativeWindow())
	application.InvokeAsync(func() { startNativeDrag(hwnd) })
}

func (s *PetService) OpenSettings() {
	s.settings.Show()
	s.settings.Focus()
}

func (s *PetService) HidePet() {
	s.pet.Hide()
}

func (s *PetService) broadcast(cfg Config) {
	s.app.Event.Emit(EventConfigChanged, cfg)
	if s.onChange != nil {
		s.onChange(cfg)
	}
}

func (s *PetService) say(r Remark) {
	cfg := s.store.Get()
	label := categoryLabels[r.Category]
	if label == "" {
		label = categoryLabels["other"]
	}
	who := userTitle(cfg)
	var prompt, fbKind string
	switch r.Kind {
	case "rest":
		prompt = fmt.Sprintf("%s已经连续工作%d分钟了，刚才在%s。提醒%s休息，比如喝水、远眺或伸懒腰。", who, r.Minutes, label, who)
		fbKind = "rest"
	default:
		prompt = fmt.Sprintf("%s正在%s，已经连续用电脑%d分钟。针对正在做的事说一句贴心或有趣的话。", who, label, r.Minutes)
		fbKind = r.Category
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	text, err := askLLM(ctx, cfg, prompt)
	if err != nil {
		text = fallbackFor(cfg, fbKind, r.Minutes)
	}
	s.app.Event.Emit(EventSay, Say{Text: text, Kind: r.Kind})
}
