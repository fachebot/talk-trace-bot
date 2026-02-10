# LLM 客户端测试说明

## 测试文件

- `client_test.go` - 单元测试（使用 mock，不依赖外部服务）
- `client_integration_test.go` - 集成测试（需要真实 API key，未设置时自动跳过）

## 运行测试

### 运行所有测试（集成测试在无 API key 时跳过）

```bash
go test ./internal/llm -v
```

### 运行特定测试

```bash
# 仅单元测试
go test ./internal/llm -v -run "TestEstimate|TestMessages|TestSplit|TestSenders|TestSummarizeChat_(Empty|Success|API|EmptyResponse|Returns|Long|Trims)"

# Token 估算
go test ./internal/llm -v -run TestEstimateTokens

# 消息分块
go test ./internal/llm -v -run TestSplitMessagesIntoChunks

# SummarizeChat
go test ./internal/llm -v -run TestSummarizeChat
```

### 运行集成测试

集成测试在 `LLM_API_KEY` 未设置时自动跳过。

```bash
# 设置环境变量后运行
set LLM_API_KEY=your-api-key
set LLM_BASE_URL=https://api.openai.com/v1   # 可选，默认 OpenAI
set LLM_MODEL=gpt-3.5-turbo                  # 可选

go test ./internal/llm -v -run Integration
```

## 测试覆盖

### 单元测试

1. **estimateTokens** - 空文本、纯中文、纯英文、中英混合、长文本
2. **messagesToPromptText** - 正常消息、空数组
3. **splitMessagesIntoChunks** - 短消息不分块、空消息、多消息分块
4. **sendersInChunk** - 发送者去重
5. **SummarizeChat** - 空消息、成功、API 错误、空响应、原始内容透传、Markdown 代码块去除、长消息分块合并

### 集成测试

1. **SummarizeChat** - 真实 API 调用，多消息总结
2. **EmptyMessages** - 空消息
3. **SingleMessage** - 单条消息

## Mock 使用

使用 `mockOpenAIClient` 模拟 OpenAI API：

```go
mockAPI := new(mockOpenAIClient)
mockAPI.On("CreateChatCompletion", mock.Anything, mock.Anything).
    Return(openai.ChatCompletionResponse{
        Choices: []openai.ChatCompletionChoice{
            {Message: openai.ChatCompletionMessage{Content: jsonResp}},
        },
    }, nil)

client := newTestClient(cfg, mockAPI)
result, err := client.SummarizeChat(ctx, messages)
```

## 注意事项

1. 集成测试会消耗 API 额度，仅在需要时设置 `LLM_API_KEY` 运行
2. 集成测试需要网络连接，有 30–60 秒超时
3. 单元测试不依赖外部服务，可频繁运行

## 覆盖率

```bash
go test ./internal/llm -cover
go test ./internal/llm -coverprofile=coverage.out
go tool cover -html=coverage.out
```
