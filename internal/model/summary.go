package model

import (
	"context"
	"time"

	"github.com/fachebot/talk-trace-bot/internal/ent"
	"github.com/fachebot/talk-trace-bot/internal/ent/summary"
)

type SummaryModel struct {
	client *ent.SummaryClient
}

func NewSummaryModel(client *ent.SummaryClient) *SummaryModel {
	return &SummaryModel{client: client}
}

type SummaryData struct {
	ChatID         int64
	SenderID       int64
	SenderName     string
	SenderUsername *string
	SenderNickname *string
	SummaryDate    time.Time
	Content        string
}

// Create 创建摘要
func (m *SummaryModel) Create(ctx context.Context, data *SummaryData) (*ent.Summary, error) {
	create := m.client.Create().
		SetChatID(data.ChatID).
		SetSenderID(data.SenderID).
		SetSenderName(data.SenderName).
		SetSummaryDate(data.SummaryDate).
		SetContent(data.Content)

	if data.SenderUsername != nil {
		create.SetSenderUsername(*data.SenderUsername)
	}
	if data.SenderNickname != nil {
		create.SetSenderNickname(*data.SenderNickname)
	}

	return create.Save(ctx)
}

// GetByDateAndChat 查询指定日期的摘要
func (m *SummaryModel) GetByDateAndChat(ctx context.Context, chatID int64, date time.Time) ([]*ent.Summary, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	return m.client.Query().
		Where(
			summary.ChatIDEQ(chatID),
			summary.SummaryDateGTE(startOfDay),
			summary.SummaryDateLT(endOfDay),
		).
		Order(summary.BySummaryDate()).
		All(ctx)
}
