package data

import (
	"context"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

func TestMiniCommentAutomaticRecoveryApprovesOnlyNewMiniRecords(t *testing.T) {
	repo, _, actor := miniFixture(t)
	ctx := context.Background()
	c := miniCreate(t, repo, actor, "auto-recovery", "ordinary content")
	c, err := repo.CompleteCheck(ctx, c, biz.MiniCommentVerdict{State: biz.MiniCommentUnavailable, ErrorKind: "unavailable"}, false)
	if err != nil {
		t.Fatal(err)
	}
	c, started, err := repo.BeginRecheck(ctx, c.Comment.ID, actor, time.Minute)
	if err != nil || !started || c.Generation != 2 {
		t.Fatal("recheck not started", err)
	}
	c, err = repo.CompleteCheck(ctx, c, biz.MiniCommentVerdict{State: biz.MiniCommentPass, NameTraceID: "name-trace", ContentTraceID: "body-trace", AutoApproveRecovery: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Visible() {
		t.Fatal("automatic recovered pass needs manual approval")
	}
}

func TestMiniCommentAutomaticRecoveryCannotApproveHistoricalWebPending(t *testing.T) {
	repo, db, actor := miniFixture(t)
	ctx := context.Background()
	web, err := NewCommentRepo(&Data{db: db}).Create(ctx, &biz.Comment{ArticleSlug: "post", UserID: &actor.UserID, DisplayName: "old web", Content: "ordinary", Status: biz.CommentStatusPending})
	if err != nil {
		t.Fatal(err)
	}
	c, started, err := repo.BeginRecheck(ctx, web.ID, actor, time.Minute)
	if err != nil || !started {
		t.Fatal(err)
	}
	c, err = repo.CompleteCheck(ctx, c, biz.MiniCommentVerdict{State: biz.MiniCommentPass, NameTraceID: "name-trace", ContentTraceID: "body-trace", AutoApproveRecovery: true}, true)
	if err != nil || c.Visible() || c.Comment.Status != biz.CommentStatusPending {
		t.Fatal("old web pending bypassed original approval", err)
	}
}
