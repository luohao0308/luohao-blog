package schema

import (
	"regexp"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// WechatSyncTask stores an immutable article snapshot and the durable state
// needed to resume WeChat publication without persisting account credentials.
type WechatSyncTask struct {
	ent.Schema
}

func (WechatSyncTask) Mixin() []ent.Mixin {
	return []ent.Mixin{IDMixin{}, TimeMixin{}}
}

func (WechatSyncTask) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").NotEmpty().MaxLen(64),
		field.String("content_hash").NotEmpty().MaxLen(128),
		field.String("mode").NotEmpty().MaxLen(32),
		// Public account identity fingerprint (SHA-256 of AppID). It lets the
		// worker reject a task created for another account without persisting
		// the AppID or any credential.
		field.String("account_fingerprint").Default("").MaxLen(64),
		field.String("state").NotEmpty().MaxLen(32),
		field.Text("article_title"),
		field.Text("article_summary"),
		field.Text("article_content_html"),
		field.Text("article_source_url"),
		field.String("article_author").MaxLen(255),
		field.String("article_thumb_media_id").MaxLen(255),
		field.String("draft_id").Optional().MaxLen(255),
		field.String("remote_id").Optional().MaxLen(255),
		field.Text("article_url").Optional(),
		// last_error is a sanitized provider code only; response bodies and
		// credentials never enter durable task state.
		field.String("last_error").Optional().MaxLen(255).
			Match(regexp.MustCompile(`^[A-Za-z0-9_.:-]*$`)),
		field.Int("attempts").Default(0).NonNegative(),
		field.Time("next_attempt_at").Optional().Nillable(),
	}
}

func (WechatSyncTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("slug", "content_hash", "mode"),
		index.Fields("state", "updated_at"),
	}
}
