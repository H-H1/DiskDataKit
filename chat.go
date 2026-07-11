package main

import (
	"DiskDataKit/ai"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// AIConfig AI 配置文件结构，按模型组织。
// key 为模型名（也作为切换标识），value 含 keys 和 base_url。
//
// 配置文件路径:
//
//	Windows: %LOCALAPPDATA%\DiskDataKit\ai_config.json
//	macOS:   ~/Library/Application Support/DiskDataKit/ai_config.json
//	Linux:   ~/.config/DiskDataKit/ai_config.json
//
// 示例:
//
//	{
//	  "default": "deepseek-v4-pro",
//	  "system_prompt": "你是 AI 助手",
//	  "providers_model": {
//	    "deepseek-v4-pro": {
//	      "keys": ["sk-key1", "sk-key2"],
//	      "base_url": "https://api.deepseek.com"
//	    },
//	    "deepseek-v4-flash": {
//	      "keys": ["sk-key1"],
//	      "base_url": "https://api.deepseek.com"
//	    }
//	  }
//	}
type AIConfig struct {
	Default        string                   `json:"default"`
	SystemPrompt   string                   `json:"system_prompt"`
	ProvidersModel map[string]AIProviderCfg `json:"providers_model"`
}

// AIProviderCfg 单个模型的配置
type AIProviderCfg struct {
	Keys    []string `json:"keys"`
	BaseURL string   `json:"base_url"`
}

// initAI 从配置文件读取多模型 API Key，初始化客户端池。
func initAI() {
	aiClients = make(map[string]*ai.Client)
	exePath, _ := os.Executable()
	// 优先读取 ai_config.json，不存在则回退到项目根目录的 ai_config.example.json
	cfgPath := configFilePath(exePath)

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		// 回退到项目根目录的 example 文件
		examplePath := filepath.Join(filepath.Dir(exePath), "ai_config.example.json")
		data, err = os.ReadFile(examplePath)
		if err != nil {
			fmt.Println("AI 配置文件未找到:", cfgPath)
			fmt.Println("也不存在示例文件:", examplePath)
			return
		}
		fmt.Println("使用示例配置:", examplePath)
	}

	var cfg AIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Println("AI 配置文件解析失败:", err)
		return
	}

	aiConfig = &cfg
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "你是 DiskDataKit 的 AI 助手，可以帮用户分析磁盘文件分布、解答问题。请简洁明了地回答。"
	}

	// 按模型初始化客户端，模型名即 key
	for modelName, pcfg := range cfg.ProvidersModel {
		if len(pcfg.Keys) == 0 {
			continue
		}
		baseURL := pcfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.deepseek.com"
		}
		client := createAIClient(modelName, baseURL, pcfg.Keys, systemPrompt)
		if client != nil {
			aiClients[modelName] = client
		}
	}

	// 设置默认模型
	if cfg.Default != "" {
		aiCurrentProvider = cfg.Default
	}
	if aiCurrentProvider == "" || aiClients[aiCurrentProvider] == nil {
		for name := range aiClients {
			aiCurrentProvider = name
			break
		}
	}

	if aiCurrentProvider != "" {
		fmt.Printf("AI 已初始化，可用模型: %d，当前: %s\n", len(aiClients), aiCurrentProvider)
	}
}

// createAIClient 根据 base_url 和模型名创建 OpenAI 兼容客户端。
// 所有模型统一走 OpenAI 兼容格式，用 base_url 区分厂商。
func createAIClient(modelName, baseURL string, keys []string, systemPrompt string) *ai.Client {
	provider := ai.NewOpenAIProvider(modelName, baseURL)
	return ai.NewClient(provider, keys,
		ai.WithSystemPrompt(systemPrompt),
		ai.WithDefaultModel(modelName),
		ai.WithMemory(20),
	)
}

// getCurrentAIClient 返回当前激活的 AI 客户端。
func getCurrentAIClient() *ai.Client {
	if aiClients == nil {
		return nil
	}
	return aiClients[aiCurrentProvider]
}

// configFilePath 返回 AI 配置文件路径。
func configFilePath(dir string) string {

	switch {
	case isWindows:
		dir = filepath.Join(dir, "DiskDataKit")
	case isDarwin:
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "DiskDataKit")
	default:
		dir = filepath.Join(os.Getenv("HOME"), ".config", "DiskDataKit")
	}
	return filepath.Join(dir, "ai_config.json")
}

// handleChat 处理聊天请求（SSE 流式响应）。
// POST /api/chat
// Body: {"message": "你好", "sessionID": "session-1", "model": "deepseek-v4-pro"}
func handleChat(w http.ResponseWriter, r *http.Request) {
	if len(aiClients) == 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]any{
			"error": "AI 未配置，请创建配置文件: ",
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

	// model 字段选择客户端（model 名即配置中的 key）
	model := req.Model
	if model == "" {
		model = aiCurrentProvider
	}
	client := aiClients[model]
	if client == nil {
		// 回退到当前默认
		client = aiClients[aiCurrentProvider]
		model = aiCurrentProvider
	}
	if client == nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "无可用模型"})
		return
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

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Minute)
	defer cancel()

	err := client.ChatStream(ctx, req.SessionID, req.Message, model, func(chunk *ai.ChatStreamChunk) error {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return nil
	})

	if err != nil {
		errData, _ := json.Marshal(map[string]any{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", errData)
		flusher.Flush()
		return
	}

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

	if client := getCurrentAIClient(); client != nil {
		client.ClearMemory(sessionID)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handleChatConfig 返回 AI 配置状态和可用模型列表。
// GET /api/chat/config
func handleChatConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if len(aiClients) == 0 {
		json.NewEncoder(w).Encode(map[string]any{
			"configured": false,
			"message":    "未配置任何模型，请创建 ",
		})
		return
	}

	// 返回可用模型列表（从配置文件读取）
	providers := []map[string]any{}
	for name, c := range aiClients {
		stats := c.KeyStats()
		baseURL := ""
		if aiConfig != nil {
			if pcfg, ok := aiConfig.ProvidersModel[name]; ok {
				baseURL = pcfg.BaseURL
			}
		}
		providers = append(providers, map[string]any{
			"name":     name,
			"baseURL":  baseURL,
			"keyCount": len(stats),
		})
	}

	json.NewEncoder(w).Encode(map[string]any{
		"configured":      true,
		"currentProvider": aiCurrentProvider,
		"providers":       providers,
		// "configPath":      configFilePath(),
	})
}
