package ai

// 使用示例：
//
// ===== 1. DeepSeek 流式对话（带上下文记忆 + 多 Key 池）=====
//
//	client := ai.NewDeepSeekClient(
//	    []string{"sk-key1", "sk-key2", "sk-key3"},  // 多 API Key 池
//	    ai.WithSystemPrompt("你是一个有帮助的助手"),
//	    ai.WithMemory(20),          // 保留最近 20 条消息
//	    ai.WithDefaultModel("deepseek-chat"),
//	)
//
//	err := client.ChatStream(ctx, "session-1", "你好", "", func(chunk *ai.ChatStreamChunk) error {
//	    if len(chunk.Choices) > 0 {
//	        fmt.Print(chunk.Choices[0].Delta.Content)
//	    }
//	    return nil
//	})
//
// ===== 2. Claude 非流式对话 =====
//
//	client := ai.NewClaudeClient(
//	    []string{"sk-ant-key1"},
//	    ai.WithDefaultModel("claude-3-5-sonnet-20241022"),
//	)
//	resp, err := client.Chat(ctx, "session-1", "解释帕累托图", "")
//	fmt.Println(resp.Choices[0].Message.Content)
//
// ===== 3. Gemini 流式对话 =====
//
//	client := ai.NewGeminiClient(
//	    []string{"AIza-key1"},
//	    ai.WithDefaultModel("gemini-2.0-flash"),
//	)
//	err := client.ChatStream(ctx, "", "写一首诗", "", func(chunk *ai.ChatStreamChunk) error {
//	    fmt.Print(chunk.Choices[0].Delta.Content)
//	    return nil
//	})
//
// ===== 4. 通义千问（OpenAI 兼容格式）=====
//
//	client := ai.NewQwenClient(
//	    []string{"sk-key1"},
//	    ai.WithDefaultModel("qwen-plus"),
//	)
//
// ===== 5. 自定义 OpenAI 兼容厂商 =====
//
//	provider := ai.NewOpenAIProvider("my-llm", "https://api.my-llm.com/v1")
//	client := ai.NewClient(provider, []string{"my-key"},
//	    ai.WithDefaultModel("my-model"),
//	)
//
// ===== 6. 查看 Key 池状态 =====
//
//	for _, info := range client.KeyStats() {
//	    fmt.Printf("Key: 健康=%v 使用=%d次 失败=%d次\n",
//	        info.Healthy, info.UseCount, info.FailCount)
//	}
