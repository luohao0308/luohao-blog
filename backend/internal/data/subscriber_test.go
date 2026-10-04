package data

import (
	"context"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

func TestSubscriberCRUD(t *testing.T) {
	ctx := context.Background()
	_, _, data := newTestRepo(t)
	repo := NewSubscriberRepo(data)

	if err := repo.DeleteSubscriber(ctx, "missing@example.com"); err != biz.ErrSubscriberNotFound {
		t.Fatalf("delete missing = %v, want not found", err)
	}
	if _, err := repo.CreateSubscriber(ctx, &biz.Subscriber{Email: "reader@example.com"}); err != nil {
		t.Fatal(err)
	}
	// The unique index backs the idempotent subscribe: creation of a duplicate
	// fails at the driver level and the usecase checks existence beforehand.
	if _, err := repo.CreateSubscriber(ctx, &biz.Subscriber{Email: "reader@example.com"}); err == nil {
		t.Fatal("duplicate subscriber accepted, want constraint error")
	}
	got, err := repo.FindByEmail(ctx, "reader@example.com")
	if err != nil || got.Email != "reader@example.com" {
		t.Fatalf("FindByEmail = %+v, %v", got, err)
	}
	if _, err := repo.CreateSubscriber(ctx, &biz.Subscriber{Email: "other@example.com"}); err != nil {
		t.Fatal(err)
	}
	list, err := repo.ListSubscribers(ctx, 0, 10)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListSubscribers = %d entries, %v; want 2", len(list), err)
	}
	if err := repo.DeleteSubscriber(ctx, "reader@example.com"); err != nil {
		t.Fatalf("delete subscriber: %v", err)
	}
	if _, err := repo.FindByEmail(ctx, "reader@example.com"); err != biz.ErrSubscriberNotFound {
		t.Fatalf("after delete = %v, want not found", err)
	}
}
