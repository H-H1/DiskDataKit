package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// httpDefaultClient 共享 HTTP 客户端
var httpDefaultClient = &http.Client{}

// newHTTPRequest 创建带 context 的 HTTP 请求
func newHTTPRequest(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

// Provider 大模型厂商接口抽象。
// 所有实现统一接收 OpenAI 格式的 ChatRequest，返回 OpenAI 格式的响应或 SSE 流。
type Provider interface {
	// Name 返回厂商名称
	Name() string

	// Chat 非流式对话
	Chat(ctx context.Context, apiKey string, req *ChatRequest) (*ChatResponse, error)

	// ChatStream 流式对话（SSE），通过 onChunk 回调返回 OpenAI 格式的 ChatStreamChunk
	ChatStream(ctx context.Context, apiKey string, req *ChatRequest, onChunk func(chunk *ChatStreamChunk) error) error
}

// ---- SSE 解析工具 ----

// parseSSEStream 读取 HTTP 响应体中的 SSE 流，逐行解析 data: 行。
// onData 回调接收每个 data: 行的内容（不含 "data: " 前缀）。
// 返回 [DONE] 时自动结束。
func parseSSEStream(body io.Reader, onData func(data string) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024) // 支持长行

	for scanner.Scan() {
		line := scanner.Text()

		// SSE 格式: "data: {...}"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return nil
		}

		if err := onData(data); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// doStreamRequest 执行流式 HTTP 请求，解析 SSE 并转换为 OpenAI 格式 chunk。
// transform 回调将厂商原始 JSON 转换为 ChatStreamChunk。
func doStreamRequest(
	ctx context.Context,
	url string,
	headers map[string]string,
	body io.Reader,
	transform func(data string) (*ChatStreamChunk, error),
	onChunk func(chunk *ChatStreamChunk) error,
) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return &ErrorResponse{ErrDetail: APIError{
			Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
			Type:    "http_error",
		}}
	}

	return parseSSEStream(resp.Body, func(data string) error {
		chunk, err := transform(data)
		if err != nil {
			return nil // 跳过无法解析的行
		}
		if chunk == nil {
			return nil
		}
		return onChunk(chunk)
	})
}

// doChatRequest 执行非流式 HTTP 请求，解析响应。
func doChatRequest(
	ctx context.Context,
	url string,
	headers map[string]string,
	body io.Reader,
) (*ChatResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &ErrorResponse{ErrDetail: APIError{
			Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
			Type:    "http_error",
		}}
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
