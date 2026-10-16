package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

// User holds the schema definition for the User entity. READER accounts are
// created through the open registration endpoint; ADMIN accounts are created
// by the seed command only.
type User struct {
	ent.Schema
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{edge.To("collections", Collection.Type), edge.To("wechat_identities", WechatIdentity.Type).Annotations(entsql.OnDelete(entsql.Cascade))}
}

// Mixin of the User.
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
		TimeMixin{},
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("email").NotEmpty().Unique(),
		// argon2id PHC-encoded string (~100 bytes for standard parameters).
		field.String("password_hash").NotEmpty(),
		field.String("display_name").Default(""),
		field.Int32("role").
			GoType(biz.UserRole(0)).
			Default(int32(biz.UserRoleAdmin)),
		// Site-relative avatar path served by GetAvatar, e.g.
		// /v1/assets/avatars/<32-hex>.<ext>. Empty until the first upload.
		field.String("avatar_url").Default(""),
		field.Bool("email_verified").Default(false),
		field.Uint64("credential_version").Default(0),
		// Legacy identity retained unchanged for explicit, verified-AppID backfill.
		// Runtime authentication uses WechatIdentity and never falls back here.
		field.String("wechat_openid").Optional().Unique(),
	}
}
