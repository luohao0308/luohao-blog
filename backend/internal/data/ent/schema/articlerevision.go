package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"time"
)

type ArticleRevision struct{ ent.Schema }

func (ArticleRevision) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}} }
func (ArticleRevision) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("article_id", uuid.Nil).Immutable(), field.Uint64("version").Immutable(), field.String("action").MaxLen(32).Immutable(), field.UUID("actor_id", uuid.Nil).Optional().Nillable().Immutable(), field.Time("created_at").Default(time.Now).Immutable(), field.JSON("snapshot", &biz.ArticleRevisionSnapshot{}).Immutable()}
}
func (ArticleRevision) Indexes() []ent.Index {
	return []ent.Index{index.Fields("article_id", "version").Unique()}
}
