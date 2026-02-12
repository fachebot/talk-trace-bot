package summarizer

// TopicSubItem 话题下的单条子项（某个发言者的贡献）
type TopicSubItem struct {
	SenderName  string  `json:"sender_name"`
	Description string  `json:"description"`
	MessageIDs  []int64 `json:"message_ids"`
}

// TopicItem 单个话题
type TopicItem struct {
	Title string         `json:"title"`
	Items []TopicSubItem `json:"items"`
}

// SummaryResult 总结结果，按话题分组
type SummaryResult struct {
	Topics []TopicItem `json:"topics"`
}
