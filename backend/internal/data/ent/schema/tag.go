package schema

import (
	"entgo.io/ent"
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
