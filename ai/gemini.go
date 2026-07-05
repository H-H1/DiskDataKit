package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// GeminiProvider Google Gemini 厂商。
// Gemini API 格式与 OpenAI 不同，这里做转换，对外统一返回 OpenAI 格式。
type GeminiProvider struct {
	baseURL string
}

// NewGeminiProvider 创建 Gemini 厂商实例。
func NewGeminiProvider() *GeminiProvider {
	return &GeminiProvider{baseURL: "https://generativelanguage.googleapis.com/v1beta"}
}

func (p *GeminiProvider) Name() string { return "gemini" }

// ---- Gemini 原始类型 ----

type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiGenConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// toGeminiRequest 将 OpenAI 格式请求转为 Gemini 格式。
func toGeminiRequest(req *ChatRequest) *geminiRequest {
	gr := &geminiRequest{
		GenerationConfig: &geminiGenConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
			TopP:            req.TopP,
		},
	}

	for _, msg := range req.Messages {
		if msg.Role == RoleSystem {
			gr.SystemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: msg.Content}},
			}
			continue
		}
		role := "user"
		if msg.Role == RoleAssistant {
			role = "model"
		}
		gr.Contents = append(gr.Contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: msg.Content}},
		})
	}

	return gr
}

func (p *GeminiProvider) Chat(ctx context.Context, apiKey string, req *ChatRequest) (*ChatResponse, error) {
	gr := toGeminiRequest(req)
	body, _ := json.Marshal(gr)

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, req.Model, apiKey)

	httpReq, err := newHTTPRequest(ctx, "POST", url, bytes.NewReader(body), nil)
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

	var gr2 geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr2); err != nil {
		return nil, err
	}

	text := ""
	finishReason := ""
	if len(gr2.Candidates) > 0 {
		for _, part := range gr2.Candidates[0].Content.Parts {
			text += part.Text
		}
		finishReason = gr2.Candidates[0].FinishReason
	}

	return &ChatResponse{
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{{
			Index:        0,
			Message:      Message{Role: RoleAssistant, Content: text},
			FinishReason: finishReason,
		}},
		Usage: Usage{
			PromptTokens:     gr2.UsageMetadata.PromptTokenCount,
			CompletionTokens: gr2.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gr2.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

func (p *GeminiProvider) ChatStream(ctx context.Context, apiKey string, req *ChatRequest, onChunk func(chunk *ChatStreamChunk) error) error {
	gr := toGeminiRequest(req)
	body, _ := json.Marshal(gr)

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", p.baseURL, req.Model, apiKey)

	headers := map[string]string{
		"Accept": "text/event-stream",
	}

	return doStreamRequest(ctx, url, headers, bytes.NewReader(body),
		// 将 Gemini SSE 事件转换为 OpenAI 格式 chunk
		func(data string) (*ChatStreamChunk, error) {
			var gr2 geminiResponse
			if err := json.Unmarshal([]byte(data), &gr2); err != nil {
				return nil, err
			}

			if len(gr2.Candidates) == 0 {
				return nil, nil
			}

			cand := gr2.Candidates[0]
			text := ""
			for _, part := range cand.Content.Parts {
				text += part.Text
			}

			finishReason := ""
			if cand.FinishReason != "" && cand.FinishReason != "FINISH_REASON_UNSPECIFIED" {
				finishReason = "stop"
			}

			return &ChatStreamChunk{
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []StreamChoice{{
					Index:        0,
					Delta:        Delta{Content: text},
					FinishReason: finishReason,
				}},
			}, nil
		},
		onChunk,
	)
}
