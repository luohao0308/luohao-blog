package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// WechatIdentity scopes a WeChat identity to its issuing application.
type WechatIdentity struct{ ent.Schema }

func (WechatIdentity) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Collation: "utf8mb4_bin"}}
}

func (WechatIdentity) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}} }
func (WechatIdentity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.Nil).Immutable(),
		field.String("app_id").NotEmpty().MaxLen(32).Immutable(),
		field.String("openid").NotEmpty().MaxLen(64).Immutable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}
func (WechatIdentity) Edges() []ent.Edge {
	return []ent.Edge{edge.From("user", User.Type).Ref("wechat_identities").Unique().Required().Immutable().Field("user_id")}
}
func (WechatIdentity) Indexes() []ent.Index {
	return []ent.Index{index.Fields("app_id", "openid").Unique().StorageKey("wechat_identity_app_openid"), index.Fields("user_id", "app_id").Unique().StorageKey("wechat_identity_user_app")}
}
