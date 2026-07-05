package main

import (
	"DiskDataKit/ai"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// initAI 从环境变量或配置文件读取 API Key，初始化 DeepSeek 客户端。
func initAI() {
	// 从环境变量读取，支持多个 key（逗号分隔）
	keys := []string{}
	if envKeys := os.Getenv("DEEPSEEK_API_KEY"); envKeys != "" {
		for _, k := range strings.Split(envKeys, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				keys = append(keys, k)
			}
		}
	}

	// 从配置文件读取
	if len(keys) == 0 {
		cfgPath := configFilePath()
		if data, err := os.ReadFile(cfgPath); err == nil {
			var cfg struct {
				DeepSeekKeys []string `json:"deepseek_keys"`
				Model        string   `json:"model"`
				SystemPrompt string   `json:"system_prompt"`
			}
			if json.Unmarshal(data, &cfg) == nil {
				keys = cfg.DeepSeekKeys
				aiClient = ai.NewDeepSeekClient(keys,
					ai.WithSystemPrompt(cfg.SystemPrompt),
					ai.WithDefaultModel(cfg.Model),
					ai.WithMemory(20),
				)
			}
		}
	}

	if len(keys) == 0 {
		// 未配置 Key，客户端为 nil，前端显示未配置状态
		return
	}

	if aiClient == nil {
		aiClient = ai.NewDeepSeekClient(keys,
			ai.WithSystemPrompt("你是 DiskDataKit 的 AI 助手，可以帮用户分析磁盘文件分布、解答问题。请简洁明了地回答。"),
			ai.WithDefaultModel("deepseek-chat"),
			ai.WithMemory(20),
		)
	}

	fmt.Printf("DeepSeek AI 已初始化，Key 数量: %d\n", len(keys))
}

// configFilePath 返回 AI 配置文件路径。
func configFilePath() string {
	var dir string
	switch {
	case isWindows:
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "DiskDataKit")
	case isDarwin:
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "DiskDataKit")
	default:
		dir = filepath.Join(os.Getenv("HOME"), ".config", "DiskDataKit")
	}
	return filepath.Join(dir, "ai_config.json")
}

// handleChat 处理聊天请求（SSE 流式响应）。
// POST /api/chat
// Body: {"message": "你好", "sessionID": "session-1", "model": "deepseek-chat"}
func handleChat(w http.ResponseWriter, r *http.Request) {
	if aiClient == nil {
		json.NewEncoder(w).Encode(map[string]any{
			"error": "AI 未配置，请设置 DEEPSEEK_API_KEY 环境变量或创建配置文件",
		})
		return
	}

	var req struct {
		Message   string `json:"message"`
		SessionID string `json:"sessionID"`
		Model     string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "请求格式错误"})
		return
	}

	if req.Message == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "消息不能为空"})
		return
	}

	if req.SessionID == "" {
		req.SessionID = "default"
	}

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		json.NewEncoder(w).Encode(map[string]any{"error": "不支持流式响应"})
		return
	}

	// 超时控制
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	// 调用 DeepSeek 流式接口
	err := aiClient.ChatStream(ctx, req.SessionID, req.Message, req.Model, func(chunk *ai.ChatStreamChunk) error {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return nil
	})

	if err != nil {
		// 发送错误事件
		errData, _ := json.Marshal(map[string]any{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", errData)
		flusher.Flush()
		return
	}

	// 发送结束标记
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// handleChatClear 清除会话记忆。
// POST /api/chat/clear?sessionID=xxx
func handleChatClear(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		sessionID = "default"
	}

	if aiClient != nil {
		aiClient.ClearMemory(sessionID)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handleChatConfig 返回 AI 配置状态。
// GET /api/chat/config
func handleChatConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if aiClient == nil {
		json.NewEncoder(w).Encode(map[string]any{
			"configured": false,
			"message":    "未配置 DEEPSEEK_API_KEY",
		})
		return
	}

	stats := aiClient.KeyStats()
	json.NewEncoder(w).Encode(map[string]any{
		"configured": true,
		"provider":   "deepseek",
		"keyCount":   len(stats),
		"available":  aiClient.KeyStats(),
	})
}
