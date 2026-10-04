package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

// Article holds the schema definition for the Article entity.
type Article struct {
	ent.Schema
}

// Mixin of the Article.
func (Article) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
		TimeMixin{},
	}
}

// Fields of the Article.
func (Article) Fields() []ent.Field {
	return []ent.Field{
		// slug is the public identifier: unique, URL-safe ([a-z0-9-]),
		// and immutable after creation. Validation of the format happens at
		// the biz layer; the schema only guarantees non-empty uniqueness.
		// It stays a bounded String: max 64 bytes, well within the default
		// VARCHAR(191) that unique indexes require on MySQL utf8mb4.
		field.String("slug").NotEmpty().Unique(),
		// Long-form fields are Text (nullable, zero-value "" on read): the
		// rendered HTML in particular has no meaningful upper bound, and
		// MySQL TEXT-typed columns cannot carry defaults.
		field.Text("title").Optional(),
		field.Text("summary").Optional(),
		// content_md is the Markdown source; content_html is the rendered
		// cache produced on write. Readers must never parse content_md.
		field.Text("content_md").Optional(),
		field.Text("content_html").Optional(),
		// status marks the row lifecycle: deletes flip it to deleted instead
		// of removing the row, so every read filters on non-deleted. Stored
		// as an integer bound to the domain type, whose values match the api
		// enum.
		field.Int32("status").
			GoType(biz.ArticleStatus(0)).
			Default(int32(biz.ArticleStatusDraft)),
		// published_at is set the first time an article moves to published
		// and never cleared afterwards.
		field.Time("published_at").Optional().Nillable(),
		// view_count is a denormalized public read counter. Only the
		// view-report endpoint increments it (after the Redis per-client
		// dedup); the article write path leaves it untouched.
		field.Uint64("view_count").Default(0),
	}
}

// Indexes of the Article.
func (Article) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("published_at"),
	}
}

// Edges of the Article. A non-unique To edge is a many-to-many: articles
// reference tag rows through the generated join table. The unique optional
// category From edge is the one-to-many foreign key: an article belongs to
// at most one category, and NULL means uncategorized.
func (Article) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tags", Tag.Type),
		edge.From("category", Category.Type).Ref("articles").Unique(),
	}
}
