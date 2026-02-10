package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

// DailyRun holds the schema definition for the DailyRun entity.
type DailyRun struct {
	ent.Schema
}

func (DailyRun) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Fields of the DailyRun.
func (DailyRun) Fields() []ent.Field {
	return []ent.Field{
		field.Time("start_time").Comment("日期范围开始时间"),
		field.Time("end_time").Comment("日期范围结束时间"),
		field.Enum("status").
			Values("pending", "in_progress", "completed", "failed").
			Default("in_progress").
			Comment("运行状态：pending=待执行, in_progress=执行中, completed=已完成, failed=失败"),
		field.String("error_message").Optional().Comment("错误信息"),
	}
}

// Indexes of the DailyRun.
func (DailyRun) Indexes() []ent.Index {
	return []ent.Index{
		// 唯一索引：防止同一日期范围重复创建
		index.Fields("start_time", "end_time").Unique(),
		// 索引：用于查询未完成运行
		index.Fields("status"),
	}
}
