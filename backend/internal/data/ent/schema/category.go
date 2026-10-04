package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Category holds the schema definition for the Category entity. Categories
// are an admin-curated single-level taxonomy: unlike tags they are not
// created on first use, and an article belongs to at most one of them
// (articles.category_id, nullable = uncategorized).
type Category struct {
	ent.Schema
}

// Mixin of the Category.
func (Category) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
		TimeMixin{},
	}
}

// Fields of the Category.
func (Category) Fields() []ent.Field {
	return []ent.Field{
		// slug follows the article slug rules (biz.ValidSlug: URL-safe,
		// 1-64 bytes) and is the public identifier, unique and immutable.
		field.String("slug").NotEmpty().Unique(),
		// name is the display label, e.g. "工程实践"; the biz layer bounds
		// it to 64 runes.
		field.String("name").NotEmpty(),
		// sort orders categories in navigation; lower comes first.
		field.Int32("sort").Default(0),
	}
}

// Edges of the Category. The non-unique To edge is the inverse of the
// Article's unique optional "category" From edge: one category has many
// articles, and the foreign key lives on the articles table. StorageKey pins
// that FK column to `category_id` (the default would be `category_articles`).
func (Category) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("articles", Article.Type).StorageKey(edge.Column("category_id")),
	}
}
