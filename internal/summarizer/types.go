package summarizer

// MemberSummaryItem 单个成员的总结
type MemberSummaryItem struct {
	SenderName string `json:"sender_name"`
	SenderID   int64  `json:"sender_id"`
	Summary    string `json:"summary"`
}

// GroupSummaryItem 群组总结
type GroupSummaryItem struct {
	Summary string `json:"summary"`
}

// SummaryResult 总结结果，包含成员总结和群组总结
type SummaryResult struct {
	MemberSummaries []MemberSummaryItem `json:"member_summaries"`
	GroupSummary    GroupSummaryItem    `json:"group_summary"`
}
