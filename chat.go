package main

import (
	"DiskDataKit/ai"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// aiMu 保护 aiClients / aiConfig / aiCurrentProvider 的并发访问。
var aiMu sync.RWMutex

// baseModelNames 记录来自可执行文件旁配置的模型名（限时模型）。
var baseModelNames map[string]bool

// AIProviderInfo 厂商元信息（前端自定义模型弹窗使用）。
type AIProviderInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Region  string `json:"region"`
	Desc    string `json:"desc"`
	Website string `json:"website"`
	BaseURL string `json:"base_url"`
}

// aiProviderInfos 预置厂商列表。
var aiProviderInfos = []AIProviderInfo{
	{ID: "deepseek", Name: "深度求索 (DeepSeek)", Region: "国内", Desc: "性价比极高，推理能力强，支持超长上下文（1M），V3/R1 系列热门", Website: "deepseek.com", BaseURL: "https://api.deepseek.com"},
	{ID: "zhipu", Name: "智谱AI (Zhipu AI)", Region: "国内", Desc: "清华系大模型，GLM 系列，企业级应用广泛，支持工具调用", Website: "bigmodel.cn", BaseURL: "https://open.bigmodel.cn/api/paas/v4"},
	{ID: "minimax", Name: "MiniMax", Region: "国内", Desc: "稀宇科技，旗下有 GLM 对标产品，主打 M3 系列，音频/多模态能力强", Website: "minimaxi.com", BaseURL: "https://api.minimaxi.com/v1"},
	{ID: "xiaomi", Name: "小米 MIMO", Region: "国内", Desc: "小米自研大模型，MIMO 系列，深度集成小米生态", Website: "xiaomimimo.com", BaseURL: "https://api.xiaomimimo.com/v1"},
	{ID: "alibaba", Name: "阿里云 (DashScope)", Region: "国内", Desc: "通义千问系列，国内市场份额大，Plus/Turbo/Max 多档位", Website: "dashscope.aliyun.com", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
	{ID: "bytedance", Name: "字节跳动 (豆包/Doubao)", Region: "国内", Desc: "字节跳动旗下大模型，豆包系列，通过火山引擎方舟平台调用，中文理解能力强，性价比高", Website: "volcengine.com", BaseURL: "https://ark.cn-beijing.volces.com/api/v3"},
}

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

// initAI 初始化 AI 客户端池。
// 基础配置已内置到代码中，用户配置（%LOCALAPPDATA%\DiskDataKit\ai_config.json）可覆盖同名模型。
func initAI() {
	aiMu.Lock()
	defer aiMu.Unlock()
	aiClients = make(map[string]*ai.Client)
	baseModelNames = make(map[string]bool)

	defaultPrompt := "你是 DiskDataKit 的 AI 助手，可以帮用户分析磁盘文件分布、解答问题。请简洁明了地回答。"

	// 合并后的配置
	var merged AIConfig
	merged.ProvidersModel = make(map[string]AIProviderCfg)
	merged.SystemPrompt = defaultPrompt

	// 1. 内置基础配置（原 ai_config.json 的值，直接写入代码）
	baseCfg := AIConfig{
		Default:      "deepseek-v4-pro",
		SystemPrompt: defaultPrompt,
		ProvidersModel: map[string]AIProviderCfg{
			"deepseek-v4-pro": {
				Keys:    []string{"sk-903626bbe0df4f34b631d313dab375a0"},
				BaseURL: "https://api.deepseek.com",
			},
			"deepseek-v4-flash": {
				Keys:    []string{"sk-903626bbe0df4f34b631d313dab375a0"},
				BaseURL: "https://api.deepseek.com",
			},
		},
	}
	if baseCfg.SystemPrompt != "" {
		merged.SystemPrompt = baseCfg.SystemPrompt
	}
	for name, pcfg := range baseCfg.ProvidersModel {
		merged.ProvidersModel[name] = pcfg
		baseModelNames[name] = true
	}
	if baseCfg.Default != "" {
		merged.Default = baseCfg.Default
	}

	// 2. 加载用户配置（%LOCALAPPDATA%），覆盖同名基础模型
	userPath := configFilePath()
	if userData, err := os.ReadFile(userPath); err == nil {
		var userCfg AIConfig
		if json.Unmarshal(userData, &userCfg) == nil {
			if userCfg.SystemPrompt != "" {
				merged.SystemPrompt = userCfg.SystemPrompt
			}
			for name, pcfg := range userCfg.ProvidersModel {
				merged.ProvidersModel[name] = pcfg
				delete(baseModelNames, name) // 用户配置覆盖，不再标记为限时
			}
			if userCfg.Default != "" {
				merged.Default = userCfg.Default
			}
			fmt.Println("已加载用户配置:", userPath)
		}
	}

	aiConfig = &merged
	systemPrompt := merged.SystemPrompt

	// 按模型初始化客户端，模型名即 key
	for modelName, pcfg := range merged.ProvidersModel {
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
	if merged.Default != "" {
		aiCurrentProvider = merged.Default
	}
	if aiCurrentProvider == "" || aiClients[aiCurrentProvider] == nil {
		for name := range aiClients {
			aiCurrentProvider = name
			break
		}
	}

	if aiCurrentProvider != "" {
		fmt.Printf("AI 已初始化，可用模型: %d（限时 %d），当前: %s\n",
			len(aiClients), len(baseModelNames), aiCurrentProvider)
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
	aiMu.RLock()
	defer aiMu.RUnlock()
	if aiClients == nil {
		return nil
	}
	return aiClients[aiCurrentProvider]
}

// configFilePath 返回 AI 配置文件路径。
// Windows: %LOCALAPPDATA%\DiskDataKit\ai_config.json
// macOS:   ~/Library/Application Support/DiskDataKit/ai_config.json
// Linux:   ~/.config/DiskDataKit/ai_config.json
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
// Body: {"message": "你好", "sessionID": "session-1", "model": "deepseek-v4-pro"}
func handleChat(w http.ResponseWriter, r *http.Request) {
	aiMu.RLock()
	if len(aiClients) == 0 {
		aiMu.RUnlock()
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
		aiMu.RUnlock()
		json.NewEncoder(w).Encode(map[string]any{"error": "请求格式错误"})
		return
	}

	if req.Message == "" {
		aiMu.RUnlock()
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
	aiMu.RUnlock()

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

// handleChatSwitch 切换当前默认模型，同步影响流氓软件检测、启动项分析、缓存分析等功能。
// POST /api/chat/switch
// Body: {"model": "deepseek-v4-pro"}
func handleChatSwitch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "请求格式错误"})
		return
	}

	aiMu.Lock()
	defer aiMu.Unlock()
	if aiClients == nil || aiClients[req.Model] == nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "模型不存在: " + req.Model})
		return
	}
	aiCurrentProvider = req.Model
	fmt.Printf("已切换默认模型: %s（流氓软件检测/启动项/缓存分析将使用此模型）\n", aiCurrentProvider)
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "current": aiCurrentProvider})
}

// handleChatConfig 返回 AI 配置状态和可用模型列表。
// GET /api/chat/config
func handleChatConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	aiMu.RLock()
	defer aiMu.RUnlock()

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
			"limited":  baseModelNames[name],
		})
	}

	json.NewEncoder(w).Encode(map[string]any{
		"configured":      true,
		"currentProvider": aiCurrentProvider,
		"providers":       providers,
	})
}

// handleChatProviders 返回预置厂商列表。
// GET /api/chat/providers
func handleChatProviders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{
		"providers": aiProviderInfos,
	})
}

// handleChatModels 使用 API Key 请求厂商的模型列表。
// POST /api/chat/models
// Body: {"provider": "deepseek", "api_key": "sk-xxx"}
func handleChatModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "请求格式错误"})
		return
	}

	if req.APIKey == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "请输入 API Key"})
		return
	}

	// 查找厂商 base_url
	var baseURL string
	for _, p := range aiProviderInfos {
		if p.ID == req.Provider {
			baseURL = p.BaseURL
			break
		}
	}
	if baseURL == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "未知厂商: " + req.Provider})
		return
	}

	// 请求 GET {base_url}/models
	modelsURL := baseURL + "/models"
	httpReq, err := http.NewRequestWithContext(r.Context(), "GET", modelsURL, nil)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "创建请求失败: " + err.Error()})
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "请求模型列表失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "读取响应失败: " + err.Error()})
		return
	}

	if resp.StatusCode != 200 {
		json.NewEncoder(w).Encode(map[string]any{
			"error": fmt.Sprintf("厂商返回错误 (HTTP %d): %s", resp.StatusCode, string(body)),
		})
		return
	}

	// 解析 OpenAI 兼容的 /models 响应: {"data": [{"id": "...", ...}, ...]}
	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "解析模型列表失败: " + err.Error()})
		return
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"models": models,
	})
}

// handleChatConfigSave 保存自定义模型配置到本地文件，并热重载 AI 客户端。
// POST /api/chat/config/save
// Body: {"provider": "deepseek", "api_key": "sk-xxx", "base_url": "https://...", "models": ["model1", "model2"]}
func handleChatConfigSave(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req struct {
		Provider string   `json:"provider"`
		APIKey   string   `json:"api_key"`
		BaseURL  string   `json:"base_url"`
		Models   []string `json:"models"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "请求格式错误"})
		return
	}

	if req.APIKey == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "请输入 API Key"})
		return
	}
	if len(req.Models) == 0 {
		json.NewEncoder(w).Encode(map[string]any{"error": "请至少选择一个模型"})
		return
	}

	// 查找厂商信息（如果 base_url 为空则用预置值）
	if req.BaseURL == "" {
		for _, p := range aiProviderInfos {
			if p.ID == req.Provider {
				req.BaseURL = p.BaseURL
				break
			}
		}
	}
	if req.BaseURL == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "未知厂商且未提供 base_url"})
		return
	}

	cfgPath := configFilePath()

	// 读取现有配置（合并模式）
	var cfg AIConfig
	if data, err := os.ReadFile(cfgPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg.ProvidersModel == nil {
		cfg.ProvidersModel = make(map[string]AIProviderCfg)
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = "你是 DiskDataKit 的 AI 助手，可以帮用户分析磁盘文件分布、解答问题。请简洁明了地回答。"
	}

	// 合并新模型到配置
	for _, model := range req.Models {
		cfg.ProvidersModel[model] = AIProviderCfg{
			Keys:    []string{req.APIKey},
			BaseURL: req.BaseURL,
		}
	}

	// 如果没有默认模型或默认模型不存在，设为第一个
	if _, ok := cfg.ProvidersModel[cfg.Default]; !ok {
		if len(req.Models) > 0 {
			cfg.Default = req.Models[0]
		}
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "创建配置目录失败: " + err.Error()})
		return
	}

	// 写入配置文件
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "序列化配置失败: " + err.Error()})
		return
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "写入配置文件失败: " + err.Error()})
		return
	}

	// 热重载
	reloadAI()

	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "配置已保存并重新加载",
		"models":  req.Models,
	})
}

// reloadAI 重新读取配置文件并重建 AI 客户端池（基础配置 + 用户配置）。

// handleChatConfigDelete 删除指定模型配置。
// POST /api/chat/config/delete
// Body: {"model": "model-name"}
func handleChatConfigDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "请求格式错误"})
		return
	}
	if req.Model == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "缺少 model 参数"})
		return
	}

	cfgPath := configFilePath()

	// 读取用户配置
	var cfg AIConfig
	if data, err := os.ReadFile(cfgPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg.ProvidersModel == nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "配置为空"})
		return
	}

	// 检查是否存在于用户配置（非基础配置）
	if _, ok := cfg.ProvidersModel[req.Model]; !ok {
		json.NewEncoder(w).Encode(map[string]any{"error": "该模型不在用户配置中，无法删除"})
		return
	}

	// 删除模型
	delete(cfg.ProvidersModel, req.Model)

	// 如果删除的是默认模型，重新选择一个
	if cfg.Default == req.Model {
		cfg.Default = ""
		for name := range cfg.ProvidersModel {
			cfg.Default = name
			break
		}
	}

	// 写入配置文件
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "序列化配置失败"})
		return
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "写入配置文件失败"})
		return
	}

	// 热重载
	reloadAI()

	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "已删除模型: " + req.Model,
	})
}
func reloadAI() {
	initAI()
}
