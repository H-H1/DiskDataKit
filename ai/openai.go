package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// OpenAIProvider 兼容 OpenAI API 格式的厂商。
// 适用于: OpenAI 官方、DeepSeek、通义千问、Moonshot、零一万物、Groq 等。
type OpenAIProvider struct {
	name   string
	baseURL string
}

// NewOpenAIProvider 创建 OpenAI 兼容厂商实例。
// name 为厂商名称，baseURL 为 API 基础地址（如 "https://api.openai.com/v1"）。
func NewOpenAIProvider(name, baseURL string) *OpenAIProvider {
	return &OpenAIProvider{name: name, baseURL: baseURL}
}

func (p *OpenAIProvider) Name() string { return p.name }

func (p *OpenAIProvider) endpoint() string {
	return p.baseURL + "/chat/completions"
}

func (p *OpenAIProvider) Chat(ctx context.Context, apiKey string, req *ChatRequest) (*ChatResponse, error) {
	req.Stream = false
	body, _ := json.Marshal(req)

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	return doChatRequest(ctx, p.endpoint(), headers, bytes.NewReader(body))
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, apiKey string, req *ChatRequest, onChunk func(chunk *ChatStreamChunk) error) error {
	req.Stream = true
	body, _ := json.Marshal(req)

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	return doStreamRequest(ctx, p.endpoint(), headers, bytes.NewReader(body),
		// OpenAI 格式直接解析
		func(data string) (*ChatStreamChunk, error) {
			var chunk ChatStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return nil, err
			}
			return &chunk, nil
		},
		onChunk,
	)
}

// ---- 预置厂商构造函数 ----

// NewDeepSeekProvider 创建 DeepSeek 厂商。
func NewDeepSeekProvider() *OpenAIProvider {
	return NewOpenAIProvider("deepseek", "https://api.deepseek.com/v1")
}

// NewQwenProvider 创建通义千问厂商。
func NewQwenProvider() *OpenAIProvider {
	return NewOpenAIProvider("qwen", "https://dashscope.aliyuncs.com/compatible-mode/v1")
}

// NewMoonshotProvider 创建 Moonshot (Kimi) 厂商。
func NewMoonshotProvider() *OpenAIProvider {
	return NewOpenAIProvider("moonshot", "https://api.moonshot.cn/v1")
}

// NewGroqProvider 创建 Groq 厂商。
func NewGroqProvider() *OpenAIProvider {
	return NewOpenAIProvider("groq", "https://api.groq.com/openai/v1")
}

// NewZhipuProvider 创建智谱 AI 厂商。
func NewZhipuProvider() *OpenAIProvider {
	return NewOpenAIProvider("zhipu", "https://open.bigmodel.cn/api/paas/v4")
}

// NewOpenAI 官方厂商。
func NewOpenAIOfficial() *OpenAIProvider {
	return NewOpenAIProvider("openai", "https://api.openai.com/v1")
}

// String 返回厂商信息。
func (p *OpenAIProvider) String() string {
	return fmt.Sprintf("%s(%s)", p.name, p.baseURL)
}
