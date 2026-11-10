package main

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	activityTick  = 5 * time.Second
	pauseAfter    = 3 * time.Minute
	breakResetsAt = 5 * time.Minute
	settleTime    = 2 * time.Minute
	restRepeat    = 15 * time.Minute
	selfExe       = "pcserve.exe"
)

var categoryLabels = map[string]string{
	"code":    "写代码",
	"browse":  "浏览网页",
	"docs":    "写文档",
	"meeting": "开会",
	"video":   "看视频",
	"design":  "做设计",
	"chat":    "聊天",
	"other":   "用电脑",
}

var appCategories = map[string][]string{
	"code": {
		"code.exe", "cursor.exe", "devenv.exe", "idea64.exe", "goland64.exe", "pycharm64.exe",
		"webstorm64.exe", "rider64.exe", "clion64.exe", "studio64.exe", "sublime_text.exe",
		"notepad++.exe", "windowsterminal.exe", "wt.exe", "powershell.exe", "pwsh.exe", "cmd.exe",
		"hbuilderx.exe", "wechatdevtools.exe",
	},
	"browse": {
		"chrome.exe", "msedge.exe", "firefox.exe", "opera.exe", "brave.exe", "360se.exe",
		"360chrome.exe", "qqbrowser.exe", "sogouexplorer.exe", "arc.exe",
	},
	"docs": {
		"winword.exe", "excel.exe", "powerpnt.exe", "wps.exe", "et.exe", "wpp.exe",
		"notion.exe", "obsidian.exe", "typora.exe", "onenote.exe", "xmind.exe",
	},
	"meeting": {"zoom.exe", "teams.exe", "ms-teams.exe", "wemeetapp.exe", "voovmeeting.exe"},
	"video":   {"potplayermini64.exe", "potplayer.exe", "vlc.exe", "mpv.exe", "bilibili.exe"},
	"design":  {"figma.exe", "photoshop.exe", "illustrator.exe", "blender.exe", "sketchup.exe"},
	"chat": {
		"dingtalk.exe", "feishu.exe", "lark.exe", "telegram.exe", "slack.exe", "discord.exe",
	},
}

func categorize(exe string) string {
	for cat, apps := range appCategories {
		if slices.Contains(apps, exe) {
			return cat
		}
	}
	return "other"
}

type Remark struct {
	Kind     string `json:"kind"`
	Category string `json:"category"`
	Minutes  int    `json:"minutes"`
}

type ActivityTracker struct {
	store *ConfigStore
	onSay func(Remark)

	mu          sync.Mutex
	workTime    time.Duration
	category    string
	catSince    time.Time
	lastRemark  time.Time
	lastRest    time.Time
	snoozeUntil time.Time
}

func NewActivityTracker(store *ConfigStore, onSay func(Remark)) *ActivityTracker {
	return &ActivityTracker{store: store, onSay: onSay, lastRemark: time.Now()}
}

func (t *ActivityTracker) Run(ctx context.Context) {
	ticker := time.NewTicker(activityTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			t.tick(now)
		}
	}
}

func (t *ActivityTracker) Snooze(d time.Duration) {
	t.mu.Lock()
	t.snoozeUntil = time.Now().Add(d)
	t.mu.Unlock()
}

func (t *ActivityTracker) WorkMinutes() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return int(t.workTime.Minutes())
}

func (t *ActivityTracker) tick(now time.Time) {
	cfg := t.store.Get().Activity
	idle := idleDuration()
	exe, fullscreen := foregroundApp()

	t.mu.Lock()
	var say *Remark
	defer func() {
		t.mu.Unlock()
		if say != nil {
			t.onSay(*say)
		}
	}()

	if idle >= breakResetsAt {
		t.workTime = 0
		t.category = ""
		return
	}
	if idle >= pauseAfter {
		return
	}
	t.workTime += activityTick

	if exe == "" || exe == selfExe {
		return
	}
	cat := categorize(exe)
	private := slices.ContainsFunc(cfg.ExcludedApps, func(a string) bool {
		return strings.EqualFold(strings.TrimSpace(a), exe)
	})
	if private {
		cat = "private"
	}
	if cat != t.category {
		t.category = cat
		t.catSince = now
	}
	busy := fullscreen || cat == "meeting" || cat == "video"
	minutes := int(t.workTime.Minutes())

	if cfg.RestReminder && t.workTime >= time.Duration(cfg.WorkMinutes)*time.Minute &&
		now.After(t.snoozeUntil) && now.Sub(t.lastRest) >= restRepeat {
		if busy {
			return
		}
		t.lastRest = now
		t.lastRemark = now
		say = &Remark{Kind: "rest", Category: cat, Minutes: minutes}
		return
	}

	if cfg.Remarks && !busy && cat != "private" && cat != "chat" &&
		now.Sub(t.catSince) >= settleTime &&
		now.Sub(t.lastRemark) >= time.Duration(cfg.RemarkMinutes)*time.Minute {
		t.lastRemark = now
		say = &Remark{Kind: "activity", Category: cat, Minutes: minutes}
	}
}
