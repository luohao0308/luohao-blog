package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"time"
)

type Collection struct{ ent.Schema }

func (Collection) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}} }
func (Collection) Fields() []ent.Field {
	return []ent.Field{field.Time("saved_at").Default(time.Now).Immutable(), field.UUID("user_id", uuid.Nil), field.UUID("article_id", uuid.Nil), field.String("article_slug").NotEmpty().MaxLen(64).Immutable()}
}
func (Collection) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("collections").Unique().Required().Field("user_id"),
		edge.From("article", Article.Type).Ref("collections").Unique().Required().Field("article_id"),
	}
}
func (Collection) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id", "article_id").Unique(), index.Fields("user_id", "saved_at", "article_slug", "id")}
}
