package llm

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/fachebot/talk-trace-bot/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// integrationTestConfig 从环境变量构建测试配置，若 LLM_API_KEY 未设置则跳过
func integrationTestConfig(t *testing.T) *config.LLM {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" || apiKey == "your-api-key-here" {
		t.Skip("跳过集成测试：请设置 LLM_API_KEY 环境变量")
	}
	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "gpt-3.5-turbo"
	}
	return &config.LLM{
		APIKey:    apiKey,
		BaseURL:   baseURL,
		Model:     model,
		MaxTokens: 16000,
	}
}

func TestSummarizeChat_Integration(t *testing.T) {
	cfg := integrationTestConfig(t)
	client := NewClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	msgs := []ChatMessage{
		{SenderID: 1, SenderName: "张三", Text: "大家下午好，我们来同步一下本周进度"},
		{SenderID: 2, SenderName: "李四", Text: "好的，我这边前端页面基本完成了，还差几个接口联调"},
		{SenderID: 3, SenderName: "王五", Text: "后端 API 已经开发完了，文档也更新到 swagger 了"},
		{SenderID: 1, SenderName: "张三", Text: "不错，李四你明天跟王五对接一下，把接口串起来"},
		{SenderID: 2, SenderName: "李四", Text: "行，我上午找他"},
		{SenderID: 4, SenderName: "赵六", Text: "测试环境我部署好了，你们联调完告诉我，我安排回归测试"},
		{SenderID: 1, SenderName: "张三", Text: "好，我们争取周四前完成联调，周五留给测试"},
		{SenderID: 3, SenderName: "王五", Text: "有个问题，用户登录那块需要加个验证码，可能要多半天"},
		{SenderID: 1, SenderName: "张三", Text: "可以，你评估一下，如果时间紧就跟我说，咱们看能不能砍掉一些非核心需求"},
		{SenderID: 2, SenderName: "李四", Text: "收到，大家加油"},
	}

	result, err := client.SummarizeChat(ctx, msgs)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	var parsed struct {
		MemberSummaries []struct {
			SenderName string `json:"sender_name"`
			SenderID   int64  `json:"sender_id"`
			Summary    string `json:"summary"`
		} `json:"member_summaries"`
		GroupSummary struct {
			Summary string `json:"summary"`
		} `json:"group_summary"`
	}
	err = json.Unmarshal([]byte(result), &parsed)
	require.NoError(t, err, "返回内容应是合法 JSON: %s", result)

	assert.GreaterOrEqual(t, len(parsed.MemberSummaries), 1, "应有至少一位成员总结")
	assert.NotEmpty(t, parsed.GroupSummary.Summary, "应有群组总结")

	// 输出总结内容
	t.Log("\n--- 成员总结 ---")
	for _, m := range parsed.MemberSummaries {
		t.Logf("- %s: %s", m.SenderName, m.Summary)
	}
	t.Log("\n--- 群组总结 ---")
	t.Log(parsed.GroupSummary.Summary)
}

func TestSummarizeChat_Integration_EmptyMessages(t *testing.T) {
	cfg := integrationTestConfig(t)
	client := NewClient(cfg)
	ctx := context.Background()

	result, err := client.SummarizeChat(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, result)

	result, err = client.SummarizeChat(ctx, []ChatMessage{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestSummarizeChat_Integration_SingleMessage(t *testing.T) {
	cfg := integrationTestConfig(t)
	client := NewClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msgs := []ChatMessage{
		{SenderID: 100, SenderName: "测试用户", Text: "这是一条单条消息的测试"},
	}

	result, err := client.SummarizeChat(ctx, msgs)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	var parsed struct {
		MemberSummaries []struct {
			SenderName string `json:"sender_name"`
			Summary    string `json:"summary"`
		} `json:"member_summaries"`
		GroupSummary struct {
			Summary string `json:"summary"`
		} `json:"group_summary"`
	}
	err = json.Unmarshal([]byte(result), &parsed)
	require.NoError(t, err)
	assert.Len(t, parsed.MemberSummaries, 1)
	assert.Equal(t, "测试用户", parsed.MemberSummaries[0].SenderName)
}
