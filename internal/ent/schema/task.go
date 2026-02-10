package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

// Task holds the schema definition for the Task entity.
type Task struct {
	ent.Schema
}

func (Task) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Fields of the Task.
func (Task) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("chat_id").Comment("群组ID"),
		field.Time("start_time").Comment("任务日期范围的开始时间"),
		field.Time("end_time").Comment("任务日期范围的结束时间"),
		field.Enum("status").
			Values("pending", "processing", "completed", "failed").
			Default("pending").
			Comment("任务状态：pending=待处理, processing=处理中, completed=已完成, failed=失败"),
		field.Time("completed_at").Optional().Comment("完成时间"),
		field.String("error_message").Optional().Comment("错误信息"),
		field.String("summary_content").Optional().Comment("已生成待发送的摘要内容；非空表示只需重试发送通知"),
	}
}

// Indexes of the Task.
func (Task) Indexes() []ent.Index {
	return []ent.Index{
		// 唯一索引：防止同一日期范围重复创建任务
		index.Fields("chat_id", "start_time", "end_time").Unique(),
		// 索引：用于查询未完成任务
		index.Fields("status"),
	}
}
