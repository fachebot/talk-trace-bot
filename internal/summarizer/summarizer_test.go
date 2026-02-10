package summarizer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fachebot/talk-trace-bot/internal/ent"
	"github.com/fachebot/talk-trace-bot/internal/llm"
	"github.com/fachebot/talk-trace-bot/internal/model"
	"github.com/stretchr/testify/assert"
)

// mockMessageProvider 用于测试的 messageProvider mock
type mockMessageProvider struct {
	messages []*ent.Message
	err      error
}

func (m *mockMessageProvider) GetByDateRangeAndChat(ctx context.Context, chatID int64, startTime, endTime time.Time) ([]*ent.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.messages, nil
}

// mockLLMSummarizer 用于测试的 llmSummarizer mock
type mockLLMSummarizer struct {
	jsonResp string
	err      error
}

func (m *mockLLMSummarizer) SummarizeChat(ctx context.Context, messages []llm.ChatMessage) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.jsonResp, nil
}

// mockSummaryWriter 用于测试的 summaryWriter mock
type mockSummaryWriter struct {
	err error
}

func (m *mockSummaryWriter) Create(ctx context.Context, data *model.SummaryData) (*ent.Summary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &ent.Summary{}, nil
}

func mustEntMessage(senderID int64, senderName, text string, sentAt time.Time) *ent.Message {
	return &ent.Message{
		SenderID:   senderID,
		SenderName: senderName,
		Text:       text,
		SentAt:     sentAt,
	}
}

func TestFormatSummaryForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		result   *SummaryResult
		dateRange string
		want     string
	}{
		{
			name:     "nil result 返回空字符串",
			result:   nil,
			dateRange: "2025-02-01 ~ 2025-02-07",
			want:     "",
		},
		{
			name:     "空结果返回空字符串",
			result:   &SummaryResult{},
			dateRange: "2025-02-01 ~ 2025-02-07",
			want:     "",
		},
		{
			name: "仅有群组总结",
			result: &SummaryResult{
				GroupSummary: GroupSummaryItem{Summary: "本周讨论了项目进度"},
			},
			dateRange: "2025-02-01 ~ 2025-02-07",
			want:     "📊 2025-02-01 ~ 2025-02-07 群聊总结\n\n--- 群组总结 ---\n本周讨论了项目进度",
		},
		{
			name: "仅有成员总结",
			result: &SummaryResult{
				MemberSummaries: []MemberSummaryItem{
					{SenderName: "张三", SenderID: 1, Summary: "分享了技术方案"},
					{SenderName: "李四", SenderID: 2, Summary: "汇报了进展"},
				},
			},
			dateRange: "2025-02-01 ~ 2025-02-07",
			want:     "📊 2025-02-01 ~ 2025-02-07 群聊总结\n\n--- 成员总结 ---\n- 张三: 分享了技术方案\n- 李四: 汇报了进展",
		},
		{
			name: "成员总结和群组总结都有",
			result: &SummaryResult{
				MemberSummaries: []MemberSummaryItem{
					{SenderName: "张三", SenderID: 1, Summary: "分享了技术方案"},
				},
				GroupSummary: GroupSummaryItem{Summary: "整体进展顺利"},
			},
			dateRange: "2025-02-01 ~ 2025-02-07",
			want:     "📊 2025-02-01 ~ 2025-02-07 群聊总结\n\n--- 成员总结 ---\n- 张三: 分享了技术方案\n\n--- 群组总结 ---\n整体进展顺利",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSummaryForDisplay(tt.result, tt.dateRange)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSummarizeRange_EmptyMessages(t *testing.T) {
	s := &Summarizer{
		messageModel: &mockMessageProvider{messages: nil},
	}
	ctx := context.Background()
	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 2, 8, 0, 0, 0, 0, time.UTC)

	result, err := s.SummarizeRange(ctx, 123, start, end)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestSummarizeRange_MessageFetchError(t *testing.T) {
	s := &Summarizer{
		messageModel: &mockMessageProvider{err: errors.New("db error")},
	}
	ctx := context.Background()
	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 2, 8, 0, 0, 0, 0, time.UTC)

	result, err := s.SummarizeRange(ctx, 123, start, end)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "获取消息失败")
}

func TestSummarizeRange_LLMError(t *testing.T) {
	now := time.Now()
	s := &Summarizer{
		messageModel: &mockMessageProvider{
			messages: []*ent.Message{
				mustEntMessage(1, "张三", "你好", now),
			},
		},
		llmClient: &mockLLMSummarizer{err: errors.New("api error")},
	}
	ctx := context.Background()
	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 2, 8, 0, 0, 0, 0, time.UTC)

	result, err := s.SummarizeRange(ctx, 123, start, end)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "LLM 总结失败")
}

func TestSummarizeRange_InvalidJSON(t *testing.T) {
	now := time.Now()
	s := &Summarizer{
		messageModel: &mockMessageProvider{
			messages: []*ent.Message{
				mustEntMessage(1, "张三", "你好", now),
			},
		},
		llmClient: &mockLLMSummarizer{jsonResp: "not valid json"},
	}
	ctx := context.Background()
	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 2, 8, 0, 0, 0, 0, time.UTC)

	result, err := s.SummarizeRange(ctx, 123, start, end)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "解析")
}

func TestSummarizeRange_Success(t *testing.T) {
	now := time.Now()
	msgProvider := &mockMessageProvider{
		messages: []*ent.Message{
			mustEntMessage(1, "张三", "分享了技术方案", now),
			mustEntMessage(2, "李四", "汇报了进展", now),
		},
	}
	llmResp := `{"member_summaries":[{"sender_name":"张三","sender_id":1,"summary":"张三分享了技术方案"},{"sender_name":"李四","sender_id":2,"summary":"李四汇报了进展"}],"group_summary":{"summary":"整体进展顺利"}}`
	s := &Summarizer{
		messageModel: msgProvider,
		llmClient:    &mockLLMSummarizer{jsonResp: llmResp},
		summaryModel: &mockSummaryWriter{},
	}
	ctx := context.Background()
	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 2, 8, 0, 0, 0, 0, time.UTC)

	result, err := s.SummarizeRange(ctx, 123, start, end)
	assert.NoError(t, err)
	requireNotNil := assert.NotNil(t, result)
	if !requireNotNil {
		return
	}
	assert.Len(t, result.MemberSummaries, 2)
	assert.Equal(t, "张三", result.MemberSummaries[0].SenderName)
	assert.Equal(t, int64(1), result.MemberSummaries[0].SenderID)
	assert.Equal(t, "张三分享了技术方案", result.MemberSummaries[0].Summary)
	assert.Equal(t, "李四", result.MemberSummaries[1].SenderName)
	assert.Equal(t, "整体进展顺利", result.GroupSummary.Summary)
}

func TestSummarizeRange_PassesStructuredMessages(t *testing.T) {
	now := time.Now()
	msgProvider := &mockMessageProvider{
		messages: []*ent.Message{
			mustEntMessage(100, "Alice", "Hello world", now),
			mustEntMessage(200, "Bob", "Hi there", now),
		},
	}
	var capturedMsgs []llm.ChatMessage
	llmMock := &mockLLMSummarizer{
		jsonResp: `{"member_summaries":[{"sender_name":"Alice","sender_id":100,"summary":"said hello"},{"sender_name":"Bob","sender_id":200,"summary":"said hi"}],"group_summary":{"summary":"greetings"}}`,
	}
	llmWrapper := &capturingLLM{
		inner:   llmMock,
		capture: func(msgs []llm.ChatMessage) { capturedMsgs = msgs },
	}
	s := &Summarizer{
		messageModel: msgProvider,
		llmClient:    llmWrapper,
		summaryModel: &mockSummaryWriter{},
	}
	ctx := context.Background()
	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 2, 8, 0, 0, 0, 0, time.UTC)

	_, err := s.SummarizeRange(ctx, 123, start, end)
	assert.NoError(t, err)
	assert.Len(t, capturedMsgs, 2)
	assert.Equal(t, int64(100), capturedMsgs[0].SenderID)
	assert.Equal(t, "Alice", capturedMsgs[0].SenderName)
	assert.Equal(t, "Hello world", capturedMsgs[0].Text)
	assert.Equal(t, int64(200), capturedMsgs[1].SenderID)
	assert.Equal(t, "Bob", capturedMsgs[1].SenderName)
	assert.Equal(t, "Hi there", capturedMsgs[1].Text)
}

// capturingLLM 用于在测试中捕获传给 SummarizeChat 的消息数组
type capturingLLM struct {
	inner   llmSummarizer
	capture func([]llm.ChatMessage)
}

func (c *capturingLLM) SummarizeChat(ctx context.Context, messages []llm.ChatMessage) (string, error) {
	c.capture(messages)
	return c.inner.SummarizeChat(ctx, messages)
}
