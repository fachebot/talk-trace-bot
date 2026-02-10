package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fachebot/talk-trace-bot/internal/config"
	"github.com/fachebot/talk-trace-bot/internal/logger"
	"github.com/sashabaranov/go-openai"
)

// openAIClientInterface 定义 OpenAI 客户端接口，便于测试
type openAIClientInterface interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

type Client struct {
	config         *config.LLM
	openaiClient   openAIClientInterface
	maxInputTokens int
}

func NewClient(cfg *config.LLM) *Client {
	openaiConfig := openai.DefaultConfig(cfg.APIKey)
	openaiConfig.BaseURL = cfg.BaseURL

	client := &Client{
		config:         cfg,
		openaiClient:   openai.NewClientWithConfig(openaiConfig),
		maxInputTokens: cfg.MaxTokens - 2000, // 预留 2000 tokens 给 system prompt 和输出
	}

	return client
}

// estimateTokens 估算文本的 token 数量
func estimateTokens(text string) int {
	// 简单估算：中文约 1.5 token/字，英文约 1.3 token/词
	// 这里使用字符数 * 1.2 作为近似值
	chineseChars := 0
	englishWords := 0

	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			chineseChars++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			englishWords++
		}
	}

	// 英文词数估算（简单按空格分割）
	words := strings.Fields(text)
	englishWords = len(words)

	// 总 token 估算
	tokens := int(float64(chineseChars)*1.5 + float64(englishWords)*1.3)
	if tokens < len(text)/4 {
		// 如果估算值太小，使用字符数的 1/4 作为下限
		tokens = len(text) / 4
	}

	return tokens
}

// ChatMessage 群聊单条消息
type ChatMessage struct {
	SenderID   int64
	SenderName string
	Text       string
}

// chatSummaryJSON 用于解析 LLM 返回的 JSON
type chatSummaryJSON struct {
	MemberSummaries []struct {
		SenderName string `json:"sender_name"`
		SenderID   int64  `json:"sender_id"`
		Summary    string `json:"summary"`
	} `json:"member_summaries"`
	GroupSummary struct {
		Summary string `json:"summary"`
	} `json:"group_summary"`
}

// messagesToPromptText 将消息数组转为 prompt 文本，格式为每行 "[发送者名|sender_id] 消息内容"
func messagesToPromptText(msgs []ChatMessage) string {
	lines := make([]string, len(msgs))
	for i, m := range msgs {
		lines[i] = fmt.Sprintf("[%s|%d] %s", m.SenderName, m.SenderID, m.Text)
	}
	return strings.Join(lines, "\n")
}

// splitMessagesIntoChunks 将消息数组按 token 估算拆分为多个 chunk
func splitMessagesIntoChunks(msgs []ChatMessage, maxTokensPerChunk int) [][]ChatMessage {
	if len(msgs) == 0 {
		return nil
	}
	chunks := make([][]ChatMessage, 0)
	current := make([]ChatMessage, 0)
	currentTokens := 0

	for _, m := range msgs {
		line := fmt.Sprintf("[%s|%d] %s", m.SenderName, m.SenderID, m.Text)
		tokens := estimateTokens(line)
		if currentTokens+tokens > maxTokensPerChunk && len(current) > 0 {
			chunks = append(chunks, current)
			current = nil
			currentTokens = 0
		}
		current = append(current, m)
		currentTokens += tokens
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

// sendersInChunk 提取 chunk 中出现的 sender_id 集合
func sendersInChunk(msgs []ChatMessage) map[int64]bool {
	seen := make(map[int64]bool)
	for _, m := range msgs {
		seen[m.SenderID] = true
	}
	return seen
}

// SummarizeChat 将群聊消息总结为 JSON，包含 member_summaries 和 group_summary
// 传入结构化的消息数组
// 返回完整的 JSON 字符串
func (c *Client) SummarizeChat(ctx context.Context, messages []ChatMessage) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}
	chatText := messagesToPromptText(messages)
	tokens := estimateTokens(chatText)

	if tokens <= c.maxInputTokens {
		return c.summarizeChatOnce(ctx, chatText, "", "")
	}

	// Token 超限，采用优化版增量拼接
	logger.Infof("[LLM] 群聊消息过长 (%d tokens)，将拆分为多个 chunk 进行总结", tokens)
	chunks := splitMessagesIntoChunks(messages, c.maxInputTokens)

	var accumulated *chatSummaryJSON
	for i, chunkMsgs := range chunks {
		logger.Debugf("[LLM] 处理 chunk %d/%d", i+1, len(chunks))
		chunkText := messagesToPromptText(chunkMsgs)

		var prevPart string
		var prevGroup string
		if accumulated != nil {
			senderSet := sendersInChunk(chunkMsgs)
			var parts []string
			for _, m := range accumulated.MemberSummaries {
				if senderSet[m.SenderID] {
					parts = append(parts, fmt.Sprintf("%s: %s", m.SenderName, m.Summary))
				}
			}
			prevPart = strings.Join(parts, "\n")
			prevGroup = accumulated.GroupSummary.Summary
		}

		raw, err := c.summarizeChatOnce(ctx, chunkText, prevPart, prevGroup)
		if err != nil {
			return "", fmt.Errorf("总结 chunk %d 失败: %w", i+1, err)
		}

		var partial chatSummaryJSON
		if err := json.Unmarshal([]byte(raw), &partial); err != nil {
			return "", fmt.Errorf("解析 chunk %d 的 JSON 失败: %w", i+1, err)
		}

		if accumulated == nil {
			accumulated = &partial
		} else {
			accMap := make(map[int64]struct {
				SenderName string
				Summary    string
			})
			for _, m := range accumulated.MemberSummaries {
				accMap[m.SenderID] = struct {
					SenderName string
					Summary    string
				}{m.SenderName, m.Summary}
			}
			for _, m := range partial.MemberSummaries {
				accMap[m.SenderID] = struct {
					SenderName string
					Summary    string
				}{m.SenderName, m.Summary}
			}
			newMembers := make([]struct {
				SenderName string `json:"sender_name"`
				SenderID   int64  `json:"sender_id"`
				Summary    string `json:"summary"`
			}, 0, len(accMap))
			for id, v := range accMap {
				newMembers = append(newMembers, struct {
					SenderName string `json:"sender_name"`
					SenderID   int64  `json:"sender_id"`
					Summary    string `json:"summary"`
				}{v.SenderName, id, v.Summary})
			}
			accumulated.MemberSummaries = newMembers
			accumulated.GroupSummary.Summary = partial.GroupSummary.Summary
		}
	}

	data, _ := json.Marshal(accumulated)
	return string(data), nil
}

// summarizeChatOnce 执行一次群聊总结请求，返回 JSON 字符串
func (c *Client) summarizeChatOnce(ctx context.Context, chunkContent, prevMembersPart, prevGroupSummary string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	systemPrompt := `你是一个专业的群聊总结助手。根据用户提供的群聊内容，输出严格的 JSON 格式，包含：
1. member_summaries: 数组，每个元素为 {"sender_name": "姓名", "sender_id": 用户ID数字, "summary": "该用户的一句话总结"}
2. group_summary: {"summary": "整个群聊的一句话总结"}

只输出 JSON，不要其他内容。`

	userPrompt := chunkContent
	if prevMembersPart != "" || prevGroupSummary != "" {
		userPrompt = "【上一轮已有总结，请在此基础上合并新内容后输出更新后的完整 JSON】\n\n"
		if prevMembersPart != "" {
			userPrompt += "上一轮这些用户的总结：\n" + prevMembersPart + "\n\n"
		}
		if prevGroupSummary != "" {
			userPrompt += "上一轮群组总结：" + prevGroupSummary + "\n\n"
		}
		userPrompt += "新消息内容：\n" + chunkContent + "\n\n请输出更新后的完整 JSON（包含所有用户和群组总结）。"
	} else {
		userPrompt = "群聊内容：\n" + chunkContent + "\n\n请输出 JSON。"
	}

	req := openai.ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   4000,
	}

	resp, err := c.openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("调用 LLM API 失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM API 返回空结果")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	return content, nil
}
