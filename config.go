package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type LLMConfig struct {
	BaseURL string `json:"baseUrl"`
	APIKey  string `json:"apiKey"`
	Model   string `json:"model"`
}

type ActivityConfig struct {
	Remarks       bool     `json:"remarks"`
	RestReminder  bool     `json:"restReminder"`
	WorkMinutes   int      `json:"workMinutes"`
	RemarkMinutes int      `json:"remarkMinutes"`
	ExcludedApps  []string `json:"excludedApps"`
}

// VoiceConfig is for Doubao speech synthesis (Volcengine). New-console keys only need APIKey;
// old-console apps use AppID + APIKey (the access token).
type VoiceConfig struct {
	Enabled    bool   `json:"enabled"`
	APIKey     string `json:"apiKey"`
	AppID      string `json:"appId"`
	ResourceID string `json:"resourceId"`
	Speaker    string `json:"speaker"`
	SpeechRate int    `json:"speechRate"`
	Style      string `json:"style"`
}

type LoverConfig struct {
	Enabled bool   `json:"enabled"`
	CallMe  string `json:"callMe"`
	Persona string `json:"persona"`
}

type Config struct {
	Pet       string         `json:"pet"`
	Name      string         `json:"name"`
	Muted     bool           `json:"muted"`
	AutoStart bool           `json:"autoStart"`
	PetX      *int           `json:"petX,omitempty"`
	PetY      *int           `json:"petY,omitempty"`
	LLM       LLMConfig      `json:"llm"`
	Voice     VoiceConfig    `json:"voice"`
	Lover     LoverConfig    `json:"lover"`
	Activity  ActivityConfig `json:"activity"`
}

const defaultLoverPersona = "你是我的恋人，温柔体贴，偶尔撒娇，会关心我的身体和心情。说话甜但不油腻，不说露骨或越界的话。"

var petNames = map[string]string{
	"cat":    "猫",
	"dog":    "狗",
	"parrot": "鹦鹉",
}

func defaultConfig() Config {
	return Config{
		Pet:  "cat",
		Name: "团子",
		Voice: VoiceConfig{
			ResourceID: "seed-tts-2.0",
			Style:      "用温柔亲昵的语气说",
		},
		Lover: LoverConfig{
			CallMe:  "宝贝",
			Persona: defaultLoverPersona,
		},
		Activity: ActivityConfig{
			Remarks:       true,
			RestReminder:  true,
			WorkMinutes:   50,
			RemarkMinutes: 20,
			ExcludedApps: []string{
				"wechat.exe", "weixin.exe", "qq.exe",
				"keepass.exe", "keepassxc.exe", "1password.exe", "bitwarden.exe",
			},
		},
	}
}

type ConfigStore struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

func NewConfigStore() *ConfigStore {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	s := &ConfigStore{
		path: filepath.Join(dir, "pcserve", "config.json"),
		cfg:  defaultConfig(),
	}
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, &s.cfg)
	}
	s.cfg = normalize(s.cfg)
	return s
}

func normalize(c Config) Config {
	d := defaultConfig()
	if _, ok := petNames[c.Pet]; !ok {
		c.Pet = d.Pet
	}
	if c.Name == "" {
		c.Name = d.Name
	}
	if c.Activity.WorkMinutes < 10 {
		c.Activity.WorkMinutes = d.Activity.WorkMinutes
	}
	if c.Activity.RemarkMinutes < 5 {
		c.Activity.RemarkMinutes = d.Activity.RemarkMinutes
	}
	if c.Voice.ResourceID == "" {
		c.Voice.ResourceID = d.Voice.ResourceID
	}
	c.Voice.SpeechRate = max(-50, min(100, c.Voice.SpeechRate))
	if c.Lover.CallMe == "" {
		c.Lover.CallMe = d.Lover.CallMe
	}
	if c.Lover.Persona == "" {
		c.Lover.Persona = d.Lover.Persona
	}
	if c.Activity.ExcludedApps == nil {
		c.Activity.ExcludedApps = d.Activity.ExcludedApps
	}
	return c
}

func (s *ConfigStore) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *ConfigStore) Update(fn func(c *Config)) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	fn(&next)
	next = normalize(next)
	if err := s.write(next); err != nil {
		return s.cfg, err
	}
	s.cfg = next
	return next, nil
}

func (s *ConfigStore) write(c Config) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	return nil
}
