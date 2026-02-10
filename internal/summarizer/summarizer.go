package summarizer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fachebot/talk-trace-bot/internal/ent"
	"github.com/fachebot/talk-trace-bot/internal/llm"
	"github.com/fachebot/talk-trace-bot/internal/logger"
	"github.com/fachebot/talk-trace-bot/internal/model"
)

// messageProvider 获取时间区间内的消息（便于测试注入 mock）
type messageProvider interface {
	GetByDateRangeAndChat(ctx context.Context, chatID int64, startTime, endTime time.Time) ([]*ent.Message, error)
}

// llmSummarizer 调用 LLM 总结群聊（便于测试注入 mock）
type llmSummarizer interface {
	SummarizeChat(ctx context.Context, messages []llm.ChatMessage) (string, error)
}

// summaryWriter 写入摘要（便于测试注入 mock）
type summaryWriter interface {
	Create(ctx context.Context, data *model.SummaryData) (*ent.Summary, error)
}

type Summarizer struct {
	llmClient    llmSummarizer
	messageModel messageProvider
	summaryModel summaryWriter
}

func NewSummarizer(llmClient *llm.Client, messageModel *model.MessageModel, summaryModel *model.SummaryModel) *Summarizer {
	return &Summarizer{
		llmClient:    llmClient,
		messageModel: messageModel,
		summaryModel: summaryModel,
	}
}

// SummarizeRange 生成指定时间区间的群聊总结
func (s *Summarizer) SummarizeRange(ctx context.Context, chatID int64, startTime, endTime time.Time) (*SummaryResult, error) {
	startStr := startTime.Format("2006-01-02")
	endStr := endTime.Format("2006-01-02")
	logger.Infof("[Summarizer] 开始生成 %s ~ %s 的群聊总结", startStr, endStr)

	messages, err := s.messageModel.GetByDateRangeAndChat(ctx, chatID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("获取消息失败: %w", err)
	}

	if len(messages) == 0 {
		logger.Infof("[Summarizer] 区间内无消息，跳过总结")
		return nil, nil
	}

	logger.Infof("[Summarizer] 找到 %d 条消息", len(messages))

	// 转换为结构化消息数组
	chatMsgs := make([]llm.ChatMessage, len(messages))
	for i, msg := range messages {
		chatMsgs[i] = llm.ChatMessage{
			SenderID:   msg.SenderID,
			SenderName: msg.SenderName,
			Text:       msg.Text,
		}
	}

	// 调用 LLM 总结
	jsonStr, err := s.llmClient.SummarizeChat(ctx, chatMsgs)
	if err != nil {
		return nil, fmt.Errorf("LLM 总结失败: %w", err)
	}

	var result SummaryResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("解析 LLM 返回的 JSON 失败: %w", err)
	}

	// 将 member_summaries 写入 Summary 表
	for _, m := range result.MemberSummaries {
		summaryData := &model.SummaryData{
			ChatID:      chatID,
			SenderID:    m.SenderID,
			SenderName:  m.SenderName,
			SummaryDate: startTime,
			Content:     m.Summary,
		}
		if _, err := s.summaryModel.Create(ctx, summaryData); err != nil {
			logger.Errorf("[Summarizer] 保存摘要失败: %v", err)
		}
	}

	logger.Infof("[Summarizer] 完成总结，共 %d 位成员", len(result.MemberSummaries))
	return &result, nil
}

// FormatSummaryForDisplay 将 SummaryResult 格式化为可读文本
func FormatSummaryForDisplay(result *SummaryResult, dateRange string) string {
	if result == nil || (len(result.MemberSummaries) == 0 && result.GroupSummary.Summary == "") {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 %s 群聊总结\n\n", dateRange))

	if len(result.MemberSummaries) > 0 {
		sb.WriteString("--- 成员总结 ---\n")
		for _, m := range result.MemberSummaries {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", m.SenderName, m.Summary))
		}
		sb.WriteString("\n")
	}

	if result.GroupSummary.Summary != "" {
		sb.WriteString("--- 群组总结 ---\n")
		sb.WriteString(result.GroupSummary.Summary)
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}
