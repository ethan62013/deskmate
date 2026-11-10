package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

var personas = map[string]string{
	"cat":    "你是一只有点傲娇但很黏人的猫",
	"dog":    "你是一只热情、忠诚、总是很开心的小狗",
	"parrot": "你是一只聪明又话多的鹦鹉，偶尔会重复主人的话",
}

var httpClient = &http.Client{Timeout: 20 * time.Second}

func systemPrompt(c Config) string {
	const format = "用中文口语回复，不超过40个字，不要用Markdown、引号或表情符号以外的格式。"
	if c.Lover.Enabled {
		return fmt.Sprintf(
			"%s\n你以一只%s的样子住在我的电脑桌面上，名字叫%s，称呼我为“%s”。现在是%s。%s",
			c.Lover.Persona, petNames[c.Pet], c.Name, c.Lover.CallMe, timeOfDay(time.Now()), format,
		)
	}
	return fmt.Sprintf("%s，名字叫%s，住在主人的电脑桌面上。%s", personas[c.Pet], c.Name, format)
}

// userTitle is how prompts refer to the user.
func userTitle(c Config) string {
	if c.Lover.Enabled {
		return "我"
	}
	return "主人"
}

func timeOfDay(t time.Time) string {
	var part string
	switch h := t.Hour(); {
	case h < 5:
		part = "深夜"
	case h < 9:
		part = "早上"
	case h < 12:
		part = "上午"
	case h < 14:
		part = "中午"
	case h < 18:
		part = "下午"
	case h < 23:
		part = "晚上"
	default:
		part = "深夜"
	}
	return fmt.Sprintf("%s %s", part, t.Format("15:04"))
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func askLLM(ctx context.Context, c Config, userPrompt string) (string, error) {
	if c.LLM.BaseURL == "" || c.LLM.Model == "" {
		return "", errNoLLM
	}
	body, _ := json.Marshal(map[string]any{
		"model": c.LLM.Model,
		"messages": []chatMessage{
			{Role: "system", Content: systemPrompt(c)},
			{Role: "user", Content: userPrompt},
		},
		"max_tokens":  120,
		"temperature": 0.9,
	})
	url := strings.TrimRight(c.LLM.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	if c.LLM.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.LLM.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("模型接口返回 %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	var out struct {
		Choices []struct {
			Message chatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("模型没有返回内容")
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	text = strings.Trim(text, "\"“”")
	return truncate(text, 80), nil
}

var errNoLLM = fmt.Errorf("未配置模型接口")

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var fallbackLines = map[string][]string{
	"pet":     {"嘿嘿，再摸摸～", "好舒服呀！", "你今天也辛苦啦", "陪你一起加油！"},
	"chat":    {"我还没学会说话呢，去设置里给我接个模型吧～", "嗯嗯，我在听！"},
	"rest":    {"已经连续忙了 %d 分钟啦，起来喝口水、看看远处吧！", "坐了 %d 分钟了，伸个懒腰再继续～"},
	"code":    {"代码写得怎么样啦？记得多存档～", "又在敲代码呀，bug 都退散！"},
	"browse":  {"在找什么好东西呀？", "网上冲浪也别太久哦～"},
	"docs":    {"写文档辛苦啦，要不要歇一下？", "字越写越多了呢！"},
	"meeting": {"开完会记得活动一下～"},
	"video":   {"这个好看吗？我也想看！"},
	"design":  {"做设计好厉害，配色真好看～"},
	"other":   {"我在这儿陪着你哦～"},
}

var loverFallbackLines = map[string][]string{
	"pet":   {"嘿嘿，被你摸摸好开心～", "想你了，就一下下", "今天也最喜欢你啦"},
	"chat":  {"我还不会好好说话，去设置里给我接个模型吧～", "嗯嗯，我一直在听你说"},
	"rest":  {"你都忙了 %d 分钟啦，起来喝口水好不好？我心疼", "坐了 %d 分钟了，抱一下再继续嘛～"},
	"other": {"我一直在这儿陪着你哦"},
}

func fallbackFor(c Config, kind string, minutes int) string {
	if c.Lover.Enabled {
		if _, ok := loverFallbackLines[kind]; !ok {
			kind = "other"
		}
		return pickLine(loverFallbackLines[kind], minutes)
	}
	return fallback(kind, minutes)
}

func pickLine(lines []string, minutes int) string {
	line := lines[rand.IntN(len(lines))]
	if strings.Contains(line, "%d") {
		return fmt.Sprintf(line, minutes)
	}
	return line
}

func fallback(kind string, minutes int) string {
	lines, ok := fallbackLines[kind]
	if !ok {
		lines = fallbackLines["other"]
	}
	return pickLine(lines, minutes)
}
