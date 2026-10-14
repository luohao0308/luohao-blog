package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type AccountEmailToken struct{ ent.Schema }

func (AccountEmailToken) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (AccountEmailToken) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.Nil),
		field.String("purpose").NotEmpty().MaxLen(16),
		field.String("token_hash").NotEmpty().MaxLen(64).Unique(),
		field.Uint64("credential_version").Default(0),
		field.Time("expires_at").Default(time.Now),
	}
}

func (AccountEmailToken) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id", "purpose").Unique(), index.Fields("expires_at")}
}
