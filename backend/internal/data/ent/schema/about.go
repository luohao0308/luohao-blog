package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

type About struct{ ent.Schema }

func (About) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (About) Fields() []ent.Field {
	return []ent.Field{
		field.String("singleton_key").NotEmpty().Unique().Immutable().MaxLen(32),
		field.String("name").Default("").MaxLen(128),
		field.Text("headline").Optional(),
		field.Text("intro").Optional(),
		field.String("profile_image").Default("").MaxLen(255),
		field.Text("current_focus").Optional(),
		field.JSON("skills", []biz.AboutSkillGroup{}),
		field.JSON("timeline", []biz.AboutTimelineEntry{}),
		field.JSON("contacts", []biz.AboutContact{}),
		field.JSON("representative_links", []biz.AboutLink{}),
	}
}
