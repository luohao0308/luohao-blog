package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

// User holds the schema definition for the User entity. Accounts are created
// by the seed command only; there is no public registration.
type User struct {
	ent.Schema
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
	}
}
