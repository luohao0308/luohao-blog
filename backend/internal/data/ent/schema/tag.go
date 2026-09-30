package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Tag holds the schema definition for the Tag entity. Tags are created on
// first use and referenced by articles through a many-to-many edge.
type Tag struct {
	ent.Schema
}

// Mixin of the Tag.
func (Tag) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
		TimeMixin{},
	}
}

// Fields of the Tag.
func (Tag) Fields() []ent.Field {
	return []ent.Field{
		// name is the tag label as shown on the site, e.g. "go" or "agent".
		field.String("name").NotEmpty().Unique(),
	}
}

// Edges of the Tag. The reciprocal non-unique From edge (Article declares
// `edge.To("tags")`) is what makes this a true M2M with a join table; a
// one-sided declaration would degrade into an O2M foreign key on this table.
func (Tag) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("articles", Article.Type).Ref("tags"),
	}
}
