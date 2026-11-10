package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
)

const ttsURL = "https://openspeech.bytedance.com/api/v3/tts/unidirectional"

const ttsDone = 20000000

var errNoVoice = errors.New("未配置语音")

// synthesize turns text into mp3 bytes with Doubao speech synthesis.
func synthesize(ctx context.Context, v VoiceConfig, text string) ([]byte, error) {
	if v.APIKey == "" || v.Speaker == "" {
		return nil, errNoVoice
	}
	text = speakable(text)
	if text == "" {
		return nil, nil
	}

	params := map[string]any{
		"text":    text,
		"speaker": v.Speaker,
		"audio_params": map[string]any{
			"format":      "mp3",
			"sample_rate": 24000,
			"speech_rate": v.SpeechRate,
		},
	}
	if v.Style != "" && strings.HasPrefix(v.ResourceID, "seed-tts-2") {
		additions, _ := json.Marshal(map[string]any{"context_texts": []string{v.Style}})
		params["additions"] = string(additions)
	}
	body, _ := json.Marshal(map[string]any{
		"user":       map[string]string{"uid": "pcserve"},
		"req_params": params,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ttsURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Resource-Id", v.ResourceID)
	req.Header.Set("X-Api-Request-Id", requestID())
	if v.AppID != "" {
		req.Header.Set("X-Api-App-Id", v.AppID)
		req.Header.Set("X-Api-Access-Key", v.APIKey)
	} else {
		req.Header.Set("X-Api-Key", v.APIKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("语音接口返回 %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}

	// The body is a stream of JSON objects, each carrying a base64 audio chunk.
	var audio bytes.Buffer
	dec := json.NewDecoder(io.LimitReader(resp.Body, 16<<20))
	for {
		var chunk struct {
			Code    int     `json:"code"`
			Message string  `json:"message"`
			Data    *string `json:"data"`
		}
		if err := dec.Decode(&chunk); err != nil {
			if errors.Is(err, io.EOF) && audio.Len() > 0 {
				return audio.Bytes(), nil
			}
			return nil, fmt.Errorf("语音数据解析失败：%w", err)
		}
		if chunk.Code == ttsDone {
			return audio.Bytes(), nil
		}
		if chunk.Code != 0 {
			return nil, fmt.Errorf("语音合成失败（%d）：%s", chunk.Code, truncate(chunk.Message, 120))
		}
		if chunk.Data != nil && *chunk.Data != "" {
			b, err := base64.StdEncoding.DecodeString(*chunk.Data)
			if err != nil {
				return nil, err
			}
			audio.Write(b)
		}
	}
}

// speakable drops emoji and other symbols the voice would read out awkwardly.
func speakable(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.So, r) || unicode.Is(unicode.Sk, r) || r == '\u200d' || r == '\ufe0f' {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func requestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
