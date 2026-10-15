package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Project struct{ ent.Schema }

func (Project) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").NotEmpty().Unique().MaxLen(64),
		field.String("name").NotEmpty().MaxLen(128),
		field.Text("summary").Optional(),
		field.Text("description").Optional(),
		field.JSON("stack", []string{}),
		field.String("role").Default("").MaxLen(128),
		field.String("repo_url").Default("").MaxLen(512),
		field.String("live_url").Default("").MaxLen(512),
		field.String("emoji").Default("").MaxLen(32),
		field.String("year").Default("").MaxLen(32),
		field.Bool("featured").Default(false),
		field.String("cover_url").Default("").MaxLen(255),
		field.JSON("screenshots", []string{}),
		field.Text("problem").Optional(),
		field.Text("approach").Optional(),
		field.Text("outcome").Optional(),
		field.Bool("published").Default(false),
		field.Int32("sort").Default(0).NonNegative(),
	}
}

func (Project) Indexes() []ent.Index {
	return []ent.Index{index.Fields("published", "sort", "slug"), index.Fields("sort")}
}
