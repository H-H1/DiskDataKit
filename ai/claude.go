package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"
)

// ClaudeProvider Anthropic Claude 厂商。
// Claude API 格式与 OpenAI 不同，这里做转换，对外统一返回 OpenAI 格式。
type ClaudeProvider struct {
	baseURL string
}

// NewClaudeProvider 创建 Claude 厂商实例。
func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{baseURL: "https://api.anthropic.com/v1"}
}

func (p *ClaudeProvider) Name() string { return "claude" }

// ---- Claude 原始类型 ----

type claudeRequest struct {
	Model     string          `json:"model"`
	Messages  []claudeMessage `json:"messages"`
	System    string          `json:"system,omitempty"`
	MaxTokens int             `json:"max_tokens"`
	Stream    bool            `json:"stream,omitempty"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	ID      string `json:"id"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type claudeStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Message claudeResponse `json:"message"`
}

// toClaudeRequest 将 OpenAI 格式请求转为 Claude 格式。
func toClaudeRequest(req *ChatRequest) *claudeRequest {
	cr := &claudeRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}
	if cr.MaxTokens == 0 {
		cr.MaxTokens = 4096
	}

	for _, msg := range req.Messages {
		if msg.Role == RoleSystem {
			cr.System = msg.Content
			continue
		}
		cr.Messages = append(cr.Messages, claudeMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	return cr
}

func (p *ClaudeProvider) headers(apiKey string) map[string]string {
	return map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
	}
}

func (p *ClaudeProvider) Chat(ctx context.Context, apiKey string, req *ChatRequest) (*ChatResponse, error) {
	cr := toClaudeRequest(req)
	cr.Stream = false
	body, _ := json.Marshal(cr)

	httpReq, err := newHTTPRequest(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body), p.headers(apiKey))
	if err != nil {
		return nil, err
	}

	resp, err := httpDefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &ErrorResponse{ErrDetail: APIError{
			Message: "HTTP " + resp.Status + ": " + string(respBody),
			Type:    "http_error",
		}}
	}

	var cr2 claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr2); err != nil {
		return nil, err
	}

	// 提取文本
	text := ""
	for _, c := range cr2.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	return &ChatResponse{
		ID:      cr2.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   cr2.Model,
		Choices: []Choice{{
			Index:        0,
			Message:      Message{Role: RoleAssistant, Content: text},
			FinishReason: cr2.StopReason,
		}},
		Usage: Usage{
			PromptTokens:     cr2.Usage.InputTokens,
			CompletionTokens: cr2.Usage.OutputTokens,
			TotalTokens:      cr2.Usage.InputTokens + cr2.Usage.OutputTokens,
		},
	}, nil
}

func (p *ClaudeProvider) ChatStream(ctx context.Context, apiKey string, req *ChatRequest, onChunk func(chunk *ChatStreamChunk) error) error {
	cr := toClaudeRequest(req)
	cr.Stream = true
	body, _ := json.Marshal(cr)

	return doStreamRequest(ctx, p.baseURL+"/messages", p.headers(apiKey), bytes.NewReader(body),
		// 将 Claude SSE 事件转换为 OpenAI 格式 chunk
		func(data string) (*ChatStreamChunk, error) {
			var event claudeStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return nil, err
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta.Type == "text_delta" {
					return &ChatStreamChunk{
						ID:      event.Message.ID,
						Object:  "chat.completion.chunk",
						Created: time.Now().Unix(),
						Model:   req.Model,
						Choices: []StreamChoice{{
							Index: 0,
							Delta: Delta{Content: event.Delta.Text},
						}},
					}, nil
				}
			case "message_stop":
				return &ChatStreamChunk{
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   req.Model,
					Choices: []StreamChoice{{
						Index:        0,
						FinishReason: "stop",
					}},
				}, nil
			}

			return nil, nil
		},
		onChunk,
	)
}
