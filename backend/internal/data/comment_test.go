package data

import (
	"context"
	stdsql "database/sql"
	"testing"

	entgo "entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
)

// newTestCommentRepo builds an in-memory SQLite-backed comment repo.
func newTestCommentRepo(t *testing.T) (biz.CommentRepo, *ent.Client) {
	t.Helper()
	db, err := stdsql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(entgo.SQLite, db)))
	t.Cleanup(func() {
		_ = client.Close()
	})
	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return NewCommentRepo(&Data{db: client}), client
}

func TestCommentRepoModerationLifecycle(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestCommentRepo(t)

	created, err := repo.Create(ctx, &biz.Comment{ArticleSlug: "post", DisplayName: "访客", Content: "留言", Status: biz.CommentStatusPending})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Pending comments are invisible to the public listing.
	visible, err := repo.ListApprovedBySlug(ctx, "post", 20, 0)
	if err != nil || len(visible) != 0 {
		t.Fatalf("ListApprovedBySlug(pending) = (%d, %v), want 0", len(visible), err)
	}

	approved, err := repo.Approve(ctx, created.ID)
	if err != nil || approved.Status != biz.CommentStatusApproved {
		t.Fatalf("Approve() = (%v, %v), want approved", approved, err)
	}
	// Idempotent second approval.
	if _, err := repo.Approve(ctx, created.ID); err != nil {
		t.Fatalf("Approve(again) error = %v", err)
	}
	visible, err = repo.ListApprovedBySlug(ctx, "post", 20, 0)
	if err != nil || len(visible) != 1 {
		t.Fatalf("ListApprovedBySlug(approved) = (%d, %v), want 1", len(visible), err)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := repo.Delete(ctx, created.ID); !kratoserrors.IsNotFound(err) {
		t.Fatalf("Delete(again) error = %v, want not found", err)
	}
}

func TestCommentRepoApproveMissing(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestCommentRepo(t)
	if _, err := repo.Approve(ctx, uuid.MustParse("00000000-0000-7000-8000-000000000001")); !kratoserrors.IsNotFound(err) {
		t.Fatalf("Approve(missing) error = %v, want not found", err)
	}
}
