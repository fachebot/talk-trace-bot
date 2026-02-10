package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/fachebot/talk-trace-bot/internal/config"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockOpenAIClient 模拟 OpenAI 客户端
type mockOpenAIClient struct {
	mock.Mock
}

func (m *mockOpenAIClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(openai.ChatCompletionResponse), args.Error(1)
}

// newTestClient 创建用于测试的客户端，注入 mock
func newTestClient(cfg *config.LLM, mockClient openAIClientInterface) *Client {
	return newTestClientWithMaxTokens(cfg, mockClient, 0)
}

// newTestClientWithMaxTokens 可指定 maxInputTokens，0 表示使用 cfg.MaxTokens-2000
func newTestClientWithMaxTokens(cfg *config.LLM, mockClient openAIClientInterface, maxInputTokens int) *Client {
	if maxInputTokens <= 0 {
		maxInputTokens = cfg.MaxTokens - 2000
		if maxInputTokens <= 0 {
			maxInputTokens = 6000
		}
	}
	return &Client{
		config:         cfg,
		openaiClient:   mockClient,
		maxInputTokens: maxInputTokens,
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantMin  int
		wantMax  int
	}{
		{"空文本", "", 0, 0},
		{"纯中文", "这是一段中文测试文本", 8, 50},
		{"纯英文", "This is a test message", 4, 30},
		{"中英混合", "Hello 世界 test 测试", 4, 40},
		{"长文本", "这是一段很长的中文文本。" + "重复" + "重复" + "重复" + "重复" + "重复", 20, 120},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTokens(tt.text)
			assert.GreaterOrEqual(t, got, tt.wantMin)
			assert.LessOrEqual(t, got, tt.wantMax)
		})
	}
}

func TestMessagesToPromptText(t *testing.T) {
	msgs := []ChatMessage{
		{SenderID: 1, SenderName: "张三", Text: "你好"},
		{SenderID: 2, SenderName: "李四", Text: "大家好"},
	}
	got := messagesToPromptText(msgs)
	assert.Contains(t, got, "[张三|1] 你好")
	assert.Contains(t, got, "[李四|2] 大家好")
}

func TestMessagesToPromptText_Empty(t *testing.T) {
	got := messagesToPromptText(nil)
	assert.Empty(t, got)
}

func TestSplitMessagesIntoChunks(t *testing.T) {
	tests := []struct {
		name              string
		msgs              []ChatMessage
		maxTokensPerChunk int
		wantChunks        int
	}{
		{
			name: "短消息不分块",
			msgs: []ChatMessage{
				{SenderID: 1, SenderName: "A", Text: "短消息"},
			},
			maxTokensPerChunk: 1000,
			wantChunks:        1,
		},
		{
			name: "空消息返回nil",
			msgs: nil,
			maxTokensPerChunk: 100,
			wantChunks:        0,
		},
		{
			name: "多消息按 token 分块",
			msgs: func() []ChatMessage {
				var msgs []ChatMessage
				for i := 0; i < 20; i++ {
					msgs = append(msgs, ChatMessage{SenderID: int64(i), SenderName: "User", Text: "这是一条较长的中文测试消息内容"})
				}
				return msgs
			}(),
			maxTokensPerChunk: 50,
			wantChunks:        -1, // -1 表示只校验多块且消息数守恒
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := splitMessagesIntoChunks(tt.msgs, tt.maxTokensPerChunk)
			if tt.wantChunks == 0 {
				assert.Nil(t, chunks)
				return
			}
			if tt.wantChunks > 0 {
				assert.Len(t, chunks, tt.wantChunks)
			} else if tt.wantChunks == -1 {
				assert.GreaterOrEqual(t, len(chunks), 2, "应拆分为多块")
			}
			total := 0
			for _, c := range chunks {
				total += len(c)
			}
			assert.Equal(t, len(tt.msgs), total, "总消息数应守恒")
		})
	}
}

func TestSendersInChunk(t *testing.T) {
	msgs := []ChatMessage{
		{SenderID: 1, SenderName: "A", Text: "x"},
		{SenderID: 2, SenderName: "B", Text: "y"},
		{SenderID: 1, SenderName: "A", Text: "z"},
	}
	got := sendersInChunk(msgs)
	assert.True(t, got[1])
	assert.True(t, got[2])
	assert.False(t, got[3])
}

func TestSummarizeChat_EmptyMessages(t *testing.T) {
	cfg := &config.LLM{Model: "test", MaxTokens: 10000}
	client := newTestClient(cfg, &mockOpenAIClient{})

	result, err := client.SummarizeChat(context.Background(), nil)
	assert.NoError(t, err)
	assert.Empty(t, result)

	result, err = client.SummarizeChat(context.Background(), []ChatMessage{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestSummarizeChat_Success(t *testing.T) {
	jsonResp := `{"member_summaries":[{"sender_name":"张三","sender_id":1,"summary":"分享了方案"},{"sender_name":"李四","sender_id":2,"summary":"汇报进展"}],"group_summary":{"summary":"整体顺利"}}`
	mockAPI := new(mockOpenAIClient)
	mockAPI.On("CreateChatCompletion", mock.Anything, mock.Anything).
		Return(openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{Message: openai.ChatCompletionMessage{Content: jsonResp}},
			},
		}, nil)

	cfg := &config.LLM{Model: "test", MaxTokens: 10000}
	client := newTestClient(cfg, mockAPI)

	msgs := []ChatMessage{
		{SenderID: 1, SenderName: "张三", Text: "分享了技术方案"},
		{SenderID: 2, SenderName: "李四", Text: "汇报了进展"},
	}
	result, err := client.SummarizeChat(context.Background(), msgs)
	assert.NoError(t, err)
	mockAPI.AssertExpectations(t)

	var parsed chatSummaryJSON
	err = json.Unmarshal([]byte(result), &parsed)
	assert.NoError(t, err)
	assert.Len(t, parsed.MemberSummaries, 2)
	assert.Equal(t, "张三", parsed.MemberSummaries[0].SenderName)
	assert.Equal(t, int64(1), parsed.MemberSummaries[0].SenderID)
	assert.Equal(t, "分享了方案", parsed.MemberSummaries[0].Summary)
	assert.Equal(t, "整体顺利", parsed.GroupSummary.Summary)
}

func TestSummarizeChat_APIError(t *testing.T) {
	mockAPI := new(mockOpenAIClient)
	mockAPI.On("CreateChatCompletion", mock.Anything, mock.Anything).
		Return(openai.ChatCompletionResponse{}, errors.New("api error"))

	cfg := &config.LLM{Model: "test", MaxTokens: 10000}
	client := newTestClient(cfg, mockAPI)

	msgs := []ChatMessage{{SenderID: 1, SenderName: "A", Text: "test"}}
	_, err := client.SummarizeChat(context.Background(), msgs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "调用 LLM API 失败")
}

func TestSummarizeChat_EmptyResponse(t *testing.T) {
	mockAPI := new(mockOpenAIClient)
	mockAPI.On("CreateChatCompletion", mock.Anything, mock.Anything).
		Return(openai.ChatCompletionResponse{Choices: nil}, nil)

	cfg := &config.LLM{Model: "test", MaxTokens: 10000}
	client := newTestClient(cfg, mockAPI)

	msgs := []ChatMessage{{SenderID: 1, SenderName: "A", Text: "test"}}
	_, err := client.SummarizeChat(context.Background(), msgs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "返回空结果")
}

func TestSummarizeChat_ReturnsRawContent(t *testing.T) {
	// 单 chunk 时，SummarizeChat 直接返回 API 的原始 content，由调用方负责解析
	mockAPI := new(mockOpenAIClient)
	mockAPI.On("CreateChatCompletion", mock.Anything, mock.Anything).
		Return(openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{Message: openai.ChatCompletionMessage{Content: "not valid json"}},
			},
		}, nil)

	cfg := &config.LLM{Model: "test", MaxTokens: 10000}
	client := newTestClient(cfg, mockAPI)

	msgs := []ChatMessage{{SenderID: 1, SenderName: "A", Text: "test"}}
	result, err := client.SummarizeChat(context.Background(), msgs)
	assert.NoError(t, err)
	assert.Equal(t, "not valid json", result)
}

func TestSummarizeChat_LongMessagesChunked(t *testing.T) {
	// 使用极小的 maxInputTokens 强制触发分块
	chunk1Resp := `{"member_summaries":[{"sender_name":"A","sender_id":1,"summary":"总结1"}],"group_summary":{"summary":"组1"}}`
	chunk2Resp := `{"member_summaries":[{"sender_name":"A","sender_id":1,"summary":"合并总结"},{"sender_name":"B","sender_id":2,"summary":"总结2"}],"group_summary":{"summary":"组2"}}`
	mockAPI := new(mockOpenAIClient)
	mockAPI.On("CreateChatCompletion", mock.Anything, mock.MatchedBy(func(req openai.ChatCompletionRequest) bool {
		// 第一次调用无上一轮总结
		return !strings.Contains(req.Messages[1].Content, "上一轮已有总结")
	})).Return(openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: chunk1Resp}}},
	}, nil).Once()
	mockAPI.On("CreateChatCompletion", mock.Anything, mock.MatchedBy(func(req openai.ChatCompletionRequest) bool {
		return strings.Contains(req.Messages[1].Content, "上一轮已有总结")
	})).Return(openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: chunk2Resp}}},
	}, nil).Once()

	cfg := &config.LLM{Model: "test", MaxTokens: 10000}
	client := newTestClientWithMaxTokens(cfg, mockAPI, 30) // 很小，强制分块

	msgs := []ChatMessage{
		{SenderID: 1, SenderName: "A", Text: "第一条较长的中文消息内容"},
		{SenderID: 2, SenderName: "B", Text: "第二条较长的中文消息内容"},
	}
	result, err := client.SummarizeChat(context.Background(), msgs)
	assert.NoError(t, err)
	mockAPI.AssertExpectations(t)

	var parsed chatSummaryJSON
	err = json.Unmarshal([]byte(result), &parsed)
	assert.NoError(t, err)
	// 合并后应包含 A 和 B
	assert.Len(t, parsed.MemberSummaries, 2)
	assert.Equal(t, "组2", parsed.GroupSummary.Summary)
}

func TestSummarizeChat_TrimsMarkdownCodeBlock(t *testing.T) {
	jsonResp := `{"member_summaries":[{"sender_name":"A","sender_id":1,"summary":"x"}],"group_summary":{"summary":"y"}}`
	wrapped := "```json\n" + jsonResp + "\n```"
	mockAPI := new(mockOpenAIClient)
	mockAPI.On("CreateChatCompletion", mock.Anything, mock.Anything).
		Return(openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{Message: openai.ChatCompletionMessage{Content: wrapped}},
			},
		}, nil)

	cfg := &config.LLM{Model: "test", MaxTokens: 10000}
	client := newTestClient(cfg, mockAPI)

	msgs := []ChatMessage{{SenderID: 1, SenderName: "A", Text: "x"}}
	result, err := client.SummarizeChat(context.Background(), msgs)
	assert.NoError(t, err)
	var parsed chatSummaryJSON
	err = json.Unmarshal([]byte(result), &parsed)
	assert.NoError(t, err)
	assert.Len(t, parsed.MemberSummaries, 1)
}
