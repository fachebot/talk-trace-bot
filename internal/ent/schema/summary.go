package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

// Summary holds the schema definition for the Summary entity.
type Summary struct {
	ent.Schema
}

func (Summary) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Fields of the Summary.
func (Summary) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("chat_id").Comment("群聊ID"),
		field.Int64("sender_id").Comment("发送者用户ID"),
		field.String("sender_name").Comment("发送者名称"),
		field.String("sender_username").Optional().Comment("发送者用户名"),
		field.String("sender_nickname").Optional().Comment("发送者昵称"),
		field.Time("summary_date").Comment("摘要日期"),
		field.Text("content").Comment("摘要内容"),
	}
}
