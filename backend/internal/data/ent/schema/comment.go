package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

// Comment holds the schema definition for a visitor comment. Comments link to
// articles by slug (articles soft-delete, so no FK edge is warranted) and are
// pre-moderated: they start pending and only approved rows are public. New
// comments carry their author account; legacy rows from the anonymous era
// have a NULL user_id and keep their stored display name.
type Comment struct {
	ent.Schema
}

// Mixin of the Comment.
func (Comment) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
		TimeMixin{},
	}
}

// Fields of the Comment.
func (Comment) Fields() []ent.Field {
	return []ent.Field{
		// article_slug is the public identifier of the commented article.
		// Field limits (name ≤ 32 runes, content ≤ 1000 runes) are enforced
		// at the biz layer; the schema only guarantees presence.
		field.String("article_slug").NotEmpty(),
		field.String("display_name").NotEmpty(),
		// content has no fixed upper bound in storage; the 1000-rune limit
		// is a biz-layer validation rule.
		field.Text("content").Optional(),
		// status marks the moderation state: pending rows are invisible to
		// public reads until an admin approves them.
		field.Int32("status").
			GoType(biz.CommentStatus(0)).
			Default(int32(biz.CommentStatusPending)),
		// Authoring account (mirror column lives in migration 000009; the
		// two sides must change together). NULL for legacy anonymous rows.
		field.UUID("user_id", uuid.UUID{}).
			Optional().
			Nillable(),
	}
}

// Indexes of the Comment.
func (Comment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("article_slug"),
		index.Fields("status"),
	}
}
