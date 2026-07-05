package ai

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Client 统一大模型客户端。
// 整合 Provider 抽象、Key 池、对话记忆，对外提供简洁的调用接口。
type Client struct {
	provider     Provider
	keyPool      *KeyPool
	memory       *Memory
	systemPrompt string
	defaultModel string
}

// Option 客户端配置选项
type Option func(*Client)

// WithSystemPrompt 设置系统提示词
func WithSystemPrompt(prompt string) Option {
	return func(c *Client) {
		c.systemPrompt = prompt
	}
}

// WithMemory 设置对话记忆（不设置则无记忆）
func WithMemory(maxHistory int) Option {
	return func(c *Client) {
		c.memory = NewMemory(maxHistory)
	}
}

// WithDefaultModel 设置默认模型
func WithDefaultModel(model string) Option {
	return func(c *Client) {
		c.defaultModel = model
	}
}

// NewClient 创建统一客户端。
// provider 为厂商实例，keys 为一个或多个 API Key。
func NewClient(provider Provider, keys []string, opts ...Option) *Client {
	c := &Client{
		provider: provider,
		keyPool:  NewKeyPool(keys...),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Name 返回厂商名称
func (c *Client) Name() string { return c.provider.Name() }

// SetSystemPrompt 设置/修改系统提示词
func (c *Client) SetSystemPrompt(prompt string) {
	c.systemPrompt = prompt
}

// ClearMemory 清除指定会话的记忆
func (c *Client) ClearMemory(sessionID string) {
	if c.memory != nil {
		c.memory.Clear(sessionID)
	}
}

// KeyStats 返回 Key 池状态
func (c *Client) KeyStats() []*KeyInfo {
	return c.keyPool.Stats()
}

// Chat 非流式对话（带记忆）。
// sessionID 用于上下文记忆，传空字符串则不使用记忆。
func (c *Client) Chat(ctx context.Context, sessionID, userMessage, model string) (*ChatResponse, error) {
	if model == "" {
		model = c.defaultModel
	}

	// 构建消息列表
	var messages []Message
	if c.memory != nil && sessionID != "" {
		messages = c.memory.GetWithSystem(sessionID, c.systemPrompt)
	} else if c.systemPrompt != "" {
		messages = append(messages, Message{Role: RoleSystem, Content: c.systemPrompt})
	}
	messages = append(messages, Message{Role: RoleUser, Content: userMessage})

	req := &ChatRequest{
		Model:    model,
		Messages: messages,
	}

	// 获取 API Key
	apiKey, _ := c.keyPool.Get()
	if apiKey == "" {
		return nil, fmt.Errorf("没有可用的 API Key（全部冷却中或未配置）")
	}

	// 调用
	resp, err := c.provider.Chat(ctx, apiKey, req)
	if err != nil {
		c.keyPool.MarkFailed(apiKey)
		// 尝试切换到下一个 Key 重试一次
		apiKey2, _ := c.keyPool.Get()
		if apiKey2 != "" && apiKey2 != apiKey {
			resp, err = c.provider.Chat(ctx, apiKey2, req)
			if err != nil {
				c.keyPool.MarkFailed(apiKey2)
				return nil, err
			}
			c.keyPool.MarkSuccess(apiKey2)
		} else {
			return nil, err
		}
	} else {
		c.keyPool.MarkSuccess(apiKey)
	}

	// 保存到记忆
	if c.memory != nil && sessionID != "" && len(resp.Choices) > 0 {
		c.memory.Add(sessionID, Message{Role: RoleUser, Content: userMessage})
		c.memory.Add(sessionID, resp.Choices[0].Message)
	}

	return resp, nil
}

// ChatStream 流式对话（SSE，带记忆）。
// onChunk 回调接收 OpenAI 格式的 ChatStreamChunk。
// sessionID 用于上下文记忆，传空字符串则不使用记忆。
func (c *Client) ChatStream(ctx context.Context, sessionID, userMessage, model string, onChunk func(chunk *ChatStreamChunk) error) error {
	if model == "" {
		model = c.defaultModel
	}

	// 构建消息列表
	var messages []Message
	if c.memory != nil && sessionID != "" {
		messages = c.memory.GetWithSystem(sessionID, c.systemPrompt)
	} else if c.systemPrompt != "" {
		messages = append(messages, Message{Role: RoleSystem, Content: c.systemPrompt})
	}
	messages = append(messages, Message{Role: RoleUser, Content: userMessage})

	req := &ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	// 获取 API Key
	apiKey, _ := c.keyPool.Get()
	if apiKey == "" {
		return fmt.Errorf("没有可用的 API Key（全部冷却中或未配置）")
	}

	// 收集流式响应内容（用于记忆）
	var fullContent strings.Builder

	// 调用流式接口
	err := c.provider.ChatStream(ctx, apiKey, req, func(chunk *ChatStreamChunk) error {
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			fullContent.WriteString(chunk.Choices[0].Delta.Content)
		}
		return onChunk(chunk)
	})

	if err != nil {
		c.keyPool.MarkFailed(apiKey)
		return err
	}

	c.keyPool.MarkSuccess(apiKey)

	// 保存到记忆
	if c.memory != nil && sessionID != "" && fullContent.Len() > 0 {
		c.memory.Add(sessionID, Message{Role: RoleUser, Content: userMessage})
		c.memory.Add(sessionID, Message{Role: RoleAssistant, Content: fullContent.String()})
	}

	return nil
}

// ---- 预置客户端构造函数 ----

// NewDeepSeekClient 创建 DeepSeek 客户端。
func NewDeepSeekClient(keys []string, opts ...Option) *Client {
	return NewClient(NewDeepSeekProvider(), keys, opts...)
}

// NewQwenClient 创建通义千问客户端。
func NewQwenClient(keys []string, opts ...Option) *Client {
	return NewClient(NewQwenProvider(), keys, opts...)
}

// NewClaudeClient 创建 Claude 客户端。
func NewClaudeClient(keys []string, opts ...Option) *Client {
	return NewClient(NewClaudeProvider(), keys, opts...)
}

// NewGeminiClient 创建 Gemini 客户端。
func NewGeminiClient(keys []string, opts ...Option) *Client {
	return NewClient(NewGeminiProvider(), keys, opts...)
}

// NewOpenAIClient 创建 OpenAI 官方客户端。
func NewOpenAIClient(keys []string, opts ...Option) *Client {
	return NewClient(NewOpenAIOfficial(), keys, opts...)
}

// NewMoonshotClient 创建 Moonshot (Kimi) 客户端。
func NewMoonshotClient(keys []string, opts ...Option) *Client {
	return NewClient(NewMoonshotProvider(), keys, opts...)
}

// NewGroqClient 创建 Groq 客户端。
func NewGroqClient(keys []string, opts ...Option) *Client {
	return NewClient(NewGroqProvider(), keys, opts...)
}

// NewZhipuClient 创建智谱 AI 客户端。
func NewZhipuClient(keys []string, opts ...Option) *Client {
	return NewClient(NewZhipuProvider(), keys, opts...)
}

// Timeout 包装 context 超时
func Timeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
