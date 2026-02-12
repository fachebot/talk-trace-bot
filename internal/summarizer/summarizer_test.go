package summarizer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fachebot/talk-trace-bot/internal/ent"
	"github.com/fachebot/talk-trace-bot/internal/llm"
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

func mustEntMessage(messageID int64, senderID int64, senderName, text string, sentAt time.Time) *ent.Message {
	return &ent.Message{
		MessageID:  messageID,
		SenderID:   senderID,
		SenderName: senderName,
		Text:       text,
		SentAt:     sentAt,
	}
}

func TestFormatSummaryForDisplay(t *testing.T) {
	// 使用典型超级群组 chatID: -1001427755127
	chatID := int64(-1001427755127)

	tests := []struct {
		name      string
		result    *SummaryResult
		chatID    int64
		startDate string
		endDate   string
		want      string
	}{
		{
			name:      "nil result 返回空字符串",
			result:    nil,
			chatID:    chatID,
			startDate: "2026-02-11",
			endDate:   "2026-02-11",
			want:      "",
		},
		{
			name:      "空结果返回空字符串",
			result:    &SummaryResult{},
			chatID:    chatID,
			startDate: "2026-02-11",
			endDate:   "2026-02-11",
			want:      "",
		},
		{
			name: "单个话题格式正确",
			result: &SummaryResult{
				Topics: []TopicItem{
					{
						Title: "技术方案讨论",
						Items: []TopicSubItem{
							{
								SenderName:  "张三",
								Description: "分享了技术方案",
								MessageIDs:  []int64{100, 101},
							},
							{
								SenderName:  "李四",
								Description: "提出了优化建议",
								MessageIDs:  []int64{102},
							},
						},
					},
				},
			},
			chatID:    chatID,
			startDate: "2026-02-11",
			endDate:   "2026-02-11",
			want: "📊 <b>群组总结</b>\n📅 2026-02-11 至 2026-02-11 (UTC)\n\n" +
				"1. 技术方案讨论\n" +
				"- <b>张三</b> 分享了技术方案 [<a href=\"https://t.me/c/1427755127/100\">link</a>] [<a href=\"https://t.me/c/1427755127/101\">link</a>]\n" +
				"- <b>李四</b> 提出了优化建议 [<a href=\"https://t.me/c/1427755127/102\">link</a>]\n",
		},
		{
			name: "多个话题格式正确",
			result: &SummaryResult{
				Topics: []TopicItem{
					{
						Title: "话题一",
						Items: []TopicSubItem{
							{SenderName: "A", Description: "说了什么", MessageIDs: []int64{1}},
						},
					},
					{
						Title: "话题二",
						Items: []TopicSubItem{
							{SenderName: "B", Description: "做了什么", MessageIDs: []int64{2}},
						},
					},
				},
			},
			chatID:    chatID,
			startDate: "2026-02-10",
			endDate:   "2026-02-11",
			want: "📊 <b>群组总结</b>\n📅 2026-02-10 至 2026-02-11 (UTC)\n\n" +
				"1. 话题一\n" +
				"- <b>A</b> 说了什么 [<a href=\"https://t.me/c/1427755127/1\">link</a>]\n\n" +
				"2. 话题二\n" +
				"- <b>B</b> 做了什么 [<a href=\"https://t.me/c/1427755127/2\">link</a>]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSummaryForDisplay(tt.result, tt.chatID, tt.startDate, tt.endDate)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToLinkMessageID(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want int64
	}{
		{"TDLib 大 ID 右移 20 位", 28132245504, 26829},
		{"已是短 ID 不变", 26829, 26829},
		{"小 ID 不变", 100, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toLinkMessageID(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildMessageLink(t *testing.T) {
	tests := []struct {
		name      string
		chatID    int64
		messageID int64
		want      string
	}{
		{
			name:      "超级群组链接",
			chatID:    -1001427755127,
			messageID: 2868456,
			want:      "https://t.me/c/1427755127/2868456",
		},
		{
			name:      "超级群组使用链接用短 message_id",
			chatID:    -1003634348229,
			messageID: 26829,
			want:      "https://t.me/c/3634348229/26829",
		},
		{
			name:      "非超级群组返回空",
			chatID:    -123456,
			messageID: 100,
			want:      "",
		},
		{
			name:      "正数 chatID 返回空",
			chatID:    12345,
			messageID: 100,
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMessageLink(tt.chatID, tt.messageID)
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
				mustEntMessage(100, 1, "张三", "你好", now),
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
				mustEntMessage(100, 1, "张三", "你好", now),
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
			mustEntMessage(100, 1, "张三", "分享了技术方案", now),
			mustEntMessage(101, 2, "李四", "汇报了进展", now),
		},
	}
	llmResp := `{"topics":[{"title":"技术讨论","items":[{"sender_name":"张三","description":"分享了技术方案","message_ids":[100]},{"sender_name":"李四","description":"汇报了进展","message_ids":[101]}]}]}`
	s := &Summarizer{
		messageModel: msgProvider,
		llmClient:    &mockLLMSummarizer{jsonResp: llmResp},
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
	assert.Len(t, result.Topics, 1)
	assert.Equal(t, "技术讨论", result.Topics[0].Title)
	assert.Len(t, result.Topics[0].Items, 2)
	assert.Equal(t, "张三", result.Topics[0].Items[0].SenderName)
	assert.Equal(t, "分享了技术方案", result.Topics[0].Items[0].Description)
	assert.Equal(t, []int64{100}, result.Topics[0].Items[0].MessageIDs)
}

func TestSummarizeRange_PassesStructuredMessages(t *testing.T) {
	now := time.Now()
	msgProvider := &mockMessageProvider{
		messages: []*ent.Message{
			mustEntMessage(500, 100, "Alice", "Hello world", now),
			mustEntMessage(501, 200, "Bob", "Hi there", now),
		},
	}
	var capturedMsgs []llm.ChatMessage
	llmMock := &mockLLMSummarizer{
		jsonResp: `{"topics":[{"title":"Greetings","items":[{"sender_name":"Alice","description":"said hello","message_ids":[500]},{"sender_name":"Bob","description":"said hi","message_ids":[501]}]}]}`,
	}
	llmWrapper := &capturingLLM{
		inner:   llmMock,
		capture: func(msgs []llm.ChatMessage) { capturedMsgs = msgs },
	}
	s := &Summarizer{
		messageModel: msgProvider,
		llmClient:    llmWrapper,
	}
	ctx := context.Background()
	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 2, 8, 0, 0, 0, 0, time.UTC)

	_, err := s.SummarizeRange(ctx, 123, start, end)
	assert.NoError(t, err)
	assert.Len(t, capturedMsgs, 2)
	assert.Equal(t, int64(500), capturedMsgs[0].MessageID)
	assert.Equal(t, int64(100), capturedMsgs[0].SenderID)
	assert.Equal(t, "Alice", capturedMsgs[0].SenderName)
	assert.Equal(t, "Hello world", capturedMsgs[0].Text)
	assert.Equal(t, int64(501), capturedMsgs[1].MessageID)
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
