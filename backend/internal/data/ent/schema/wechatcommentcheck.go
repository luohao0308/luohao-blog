package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// No comment FK: a minimal request tombstone must survive physical deletion.
type WechatCommentCheck struct{ ent.Schema }

func (WechatCommentCheck) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}} }
func (WechatCommentCheck) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Collation: "utf8mb4_bin"}}
}
func (WechatCommentCheck) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("comment_id", uuid.Nil).Immutable(), field.UUID("author_id", uuid.Nil).Immutable(), field.String("app_id").MaxLen(32).Immutable(),
		field.String("client_request_id").MaxLen(64).Optional().Nillable().Immutable(), field.String("create_request_hash").MaxLen(64).Default("").Immutable(),
		field.UUID("identity_id", uuid.Nil).Optional().Nillable(), field.String("input_hash").MaxLen(64).Default(""), field.String("state").MaxLen(32), field.Uint64("generation").Default(1),
		field.Time("lease_until").SchemaType(map[string]string{dialect.MySQL: "datetime(6)"}).Optional().Nillable(), field.String("name_trace_id").MaxLen(256).Default(""), field.String("content_trace_id").MaxLen(256).Default(""), field.Time("checked_at").SchemaType(map[string]string{dialect.MySQL: "datetime(6)"}).Optional().Nillable(), field.String("error_kind").MaxLen(64).Default(""),
	}
}
func (WechatCommentCheck) Indexes() []ent.Index {
	return []ent.Index{index.Fields("comment_id", "app_id").Unique().StorageKey("wechat_comment_app"), index.Fields("app_id", "author_id", "client_request_id").Unique().StorageKey("wechat_comment_request"), index.Fields("app_id", "state").StorageKey("wechat_comment_state")}
}

func (WechatCommentCheck) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("author", User.Type).Unique().Required().Immutable().Field("author_id").Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("identity", WechatIdentity.Type).Unique().Field("identity_id").Annotations(entsql.OnDelete(entsql.SetNull)),
	}
}
