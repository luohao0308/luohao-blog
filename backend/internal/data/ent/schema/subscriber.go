package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Subscriber holds the schema definition for the Subscriber entity: an email
// registered for future article notifications. Actual sending is not wired
// yet; the table is the registration ledger the future sender reads.
type Subscriber struct {
	ent.Schema
}

// Mixin of the Subscriber.
func (Subscriber) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
		TimeMixin{},
	}
}

// Fields of the Subscriber.
func (Subscriber) Fields() []ent.Field {
	return []ent.Field{
		// email is the subscription key: unique and lowercased by the biz
		// layer before storage. Bounded to fit the unique index comfortably;
		// the biz layer validates the format and a 254-char RFC cap further
		// down to this bound.
		field.String("email").NotEmpty().Unique(),
	}
}
