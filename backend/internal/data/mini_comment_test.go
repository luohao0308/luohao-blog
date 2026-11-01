package data

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	entgo "entgo.io/ent"
	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatcommentcheck"
	"sync/atomic"
)

func miniFixture(t *testing.T) (biz.MiniCommentRepo, *ent.Client, biz.MiniCommentActor) {
	t.Helper()
	_, db := newTestCommentRepo(t)
	ctx := context.Background()
	author, e := db.User.Create().SetEmail(t.Name() + "@example.invalid").SetPasswordHash("synthetic-only").SetDisplayName("原始昵称").SetRole(biz.UserRoleReader).Save(ctx)
	if e != nil {
		t.Fatal(e)
	}
	identity, e := db.WechatIdentity.Create().SetUserID(author.ID).SetAppID("wx-mini-fixture").SetOpenid("openid-fixture").Save(ctx)
	if e != nil {
		t.Fatal(e)
	}
	return NewMiniCommentRepo(&Data{db: db}), db, biz.MiniCommentActor{UserID: author.ID, AppID: identity.AppID, OpenID: identity.Openid}
}
func miniCreate(t *testing.T, r biz.MiniCommentRepo, a biz.MiniCommentActor, key, content string) *biz.MiniComment {
	t.Helper()
	input := &biz.Comment{ArticleSlug: "post", Content: content}
	out, created, e := r.CreatePending(context.Background(), input, a, key, biz.MiniCommentRequestHash(input.ArticleSlug, input.Content), time.Minute)
	if e != nil || !created {
		t.Fatalf("create: %v %v", created, e)
	}
	return out
}
func miniPass(t *testing.T, r biz.MiniCommentRepo, c *biz.MiniComment, approve bool) *biz.MiniComment {
	t.Helper()
	v, e := r.CompleteCheck(context.Background(), c, biz.MiniCommentVerdict{State: biz.MiniCommentPass, NameTraceID: "name-trace", ContentTraceID: "body-trace"}, approve)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestMiniCommentPendingIdempotencyAndSnapshot(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	c := miniCreate(t, r, a, "intent-1", "hello")
	if c.Comment.Status != biz.CommentStatusPending || c.State != biz.MiniCommentChecking || !c.FingerprintValid || c.Comment.DisplayName != "原始昵称" {
		t.Fatal("pending snapshot invalid")
	}
	if _, e := db.User.UpdateOneID(a.UserID).SetDisplayName("后改昵称").Save(ctx); e != nil {
		t.Fatal(e)
	}
	old, created, e := r.CreatePending(ctx, &biz.Comment{ArticleSlug: "post", Content: "hello"}, a, "intent-1", biz.MiniCommentRequestHash("post", "hello"), time.Minute)
	if e != nil || created || old.Comment.ID != c.Comment.ID || old.Comment.DisplayName != "原始昵称" {
		t.Fatal("replay reread mutable nickname", e)
	}
	if _, e = r.FindRequest(ctx, a.UserID, a.AppID, "intent-1", biz.MiniCommentRequestHash("post", "different")); !errors.Is(e, biz.ErrMiniCommentConflict) {
		t.Fatal("different payload not409", e)
	}
	if n, _ := db.Comment.Query().Count(ctx); n != 1 {
		t.Fatal("replay duplicate comment")
	}
	other := a
	other.OpenID = "wrong"
	if _, _, e = r.CreatePending(ctx, &biz.Comment{ArticleSlug: "post", Content: "x"}, other, "other", biz.MiniCommentRequestHash("post", "x"), time.Minute); e == nil {
		t.Fatal("wrong identity accepted")
	}
}
func TestMiniCommentApprovalAndOwnerProjection(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	c := miniCreate(t, r, a, "one", "hello")
	c = miniPass(t, r, c, false)
	if c.Visible() {
		t.Fatal("pass bypassed global approval")
	}
	if _, e := NewCommentRepo(&Data{db: db}).Approve(ctx, c.Comment.ID); e != nil {
		t.Fatal(e)
	}
	list, next, e := r.ListPublic(ctx, "post", a.AppID, 20, 0)
	if e != nil || len(list) != 1 || !list[0].Visible() || next != -1 || list[0].Comment.AvatarURL != "" {
		t.Fatal("public projection", e)
	}
	if _, e = r.FindOwned(ctx, c.Comment.ID, uuid.New(), a.AppID); !errors.Is(e, biz.ErrCommentNotFound) {
		t.Fatal("owner disclosure", e)
	}
	list, _, e = r.ListPublic(ctx, "post", "other-app", 20, 0)
	if e != nil || len(list) != 0 {
		t.Fatal("cross app visible")
	}
	web, e := NewCommentRepo(&Data{db: db}).Create(ctx, &biz.Comment{ArticleSlug: "post", Content: "web", DisplayName: "Web", UserID: &a.UserID, Status: biz.CommentStatusApproved})
	if e != nil {
		t.Fatal(e)
	}
	mine, _, e := r.ListMine(ctx, "post", a.UserID, a.AppID, 20, 0)
	if e != nil || len(mine) != 2 {
		t.Fatal("mine missing web", e)
	}
	owned, e := r.FindOwned(ctx, web.ID, a.UserID, a.AppID)
	if e != nil || owned.State != biz.MiniCommentUnknown || owned.Visible() {
		t.Fatal("legacy wrongly certified", e)
	}
}
func TestMiniCommentGenerationLeaseAndVerdicts(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	old := miniCreate(t, r, a, "one", "hello")
	if _, _, e := r.BeginRecheck(ctx, old.Comment.ID, a, time.Minute); !errors.Is(e, biz.ErrMiniCommentCheckBusy) {
		t.Fatal("live lease not busy", e)
	}
	if _, e := db.WechatCommentCheck.Update().Where(wechatcommentcheck.CommentIDEQ(old.Comment.ID)).SetLeaseUntil(time.Now().Add(-time.Second)).Save(ctx); e != nil {
		t.Fatal(e)
	}
	current, started, e := r.BeginRecheck(ctx, old.Comment.ID, a, time.Minute)
	if e != nil || !started || current.Generation != 2 {
		t.Fatal("lease recovery", e)
	}
	if _, e = r.CompleteCheck(ctx, old, biz.MiniCommentVerdict{State: biz.MiniCommentPass, NameTraceID: "name-trace", ContentTraceID: "body-trace"}, true); !errors.Is(e, biz.ErrMiniCommentConflict) {
		t.Fatal("late old generation not rejected by generation/record guard", e)
	}
	latest, e := r.FindOwned(ctx, old.Comment.ID, a.UserID, a.AppID)
	if e != nil || latest.Generation != 2 || latest.State != biz.MiniCommentChecking || latest.Comment.Status != biz.CommentStatusPending {
		t.Fatal("late result changed the active generation", e)
	}
	current = miniPass(t, r, current, true)
	if current.Comment.Status != biz.CommentStatusPending {
		t.Fatal("recheck incorrectly bypassed approval")
	}
	if v, started, e := r.BeginRecheck(ctx, current.Comment.ID, a, time.Minute); e != nil || started || v.Generation != 2 {
		t.Fatal("pass rechecked", e)
	}
	risky := miniCreate(t, r, a, "two", "risky")
	risky, e = r.CompleteCheck(ctx, risky, biz.MiniCommentVerdict{State: biz.MiniCommentRisky}, true)
	if e != nil || risky.Comment.Status != biz.CommentStatusPending {
		t.Fatal("risky approved", e)
	}
	if _, started, e = r.BeginRecheck(ctx, risky.Comment.ID, a, time.Minute); e != nil || started {
		t.Fatal("unchanged risky rechecked", e)
	}
}
func TestMiniCommentDeleteTombstoneAndLateCompletion(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	c := miniCreate(t, r, a, "intent", "hello")
	if e := NewCommentRepo(&Data{db: db}).Delete(ctx, c.Comment.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := r.CompleteCheck(ctx, c, biz.MiniCommentVerdict{State: biz.MiniCommentPass, NameTraceID: "name-trace", ContentTraceID: "body-trace"}, true); !errors.Is(e, biz.ErrCommentNotFound) {
		t.Fatal("completion did not reach the deleted-row guard", e)
	}
	if _, e := r.FindRequest(ctx, a.UserID, a.AppID, "intent", biz.MiniCommentRequestHash("post", "hello")); !errors.Is(e, biz.ErrMiniCommentDeleted) {
		t.Fatal("tombstone not410", e)
	}
	if _, _, e := r.CreatePending(ctx, &biz.Comment{ArticleSlug: "post", Content: "hello"}, a, "intent", biz.MiniCommentRequestHash("post", "hello"), time.Minute); !errors.Is(e, biz.ErrMiniCommentDeleted) {
		t.Fatal("deleted replay recreated", e)
	}
	check, e := db.WechatCommentCheck.Query().Only(ctx)
	if e != nil || check.State != "deleted" || check.IdentityID != nil || check.LeaseUntil != nil || check.InputHash != "" || check.NameTraceID != "" || check.ContentTraceID != "" || check.ErrorKind != "" {
		t.Fatal("deleted evidence retained", e)
	}
	if n, _ := db.Comment.Query().Count(ctx); n != 0 {
		t.Fatal("orphan comment")
	}
}

func TestMiniCommentOldPassCannotOverwriteNewRisky(t *testing.T) {
	repo, db, actor := miniFixture(t)
	ctx := context.Background()
	old := miniCreate(t, repo, actor, "late-pass", "body")
	if _, err := db.WechatCommentCheck.Update().Where(wechatcommentcheck.CommentIDEQ(old.Comment.ID)).SetLeaseUntil(time.Now().Add(-time.Second)).Save(ctx); err != nil {
		t.Fatal(err)
	}
	current, started, err := repo.BeginRecheck(ctx, old.Comment.ID, actor, time.Minute)
	if err != nil || !started {
		t.Fatal("new check did not start", err)
	}
	if _, err = repo.CompleteCheck(ctx, current, biz.MiniCommentVerdict{State: biz.MiniCommentRisky, NameTraceID: "new-risk-trace"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CompleteCheck(ctx, old, biz.MiniCommentVerdict{State: biz.MiniCommentPass, NameTraceID: "old-name-pass", ContentTraceID: "old-body-pass"}, true); !errors.Is(err, biz.ErrMiniCommentConflict) {
		t.Fatal("old pass overwrote new verdict", err)
	}
	latest, err := repo.FindOwned(ctx, old.Comment.ID, actor.UserID, actor.AppID)
	if err != nil || latest.State != biz.MiniCommentRisky || latest.Generation != 2 || latest.Visible() || latest.Comment.Status != biz.CommentStatusPending {
		t.Fatal("new risk verdict changed", err)
	}
}
func TestMiniCommentInvalidFingerprintPagination(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	var invalid *biz.MiniComment
	for i := 0; i < 3; i++ {
		invalid = miniPass(t, r, miniCreate(t, r, a, fmt.Sprint(i), fmt.Sprint(i)), true)
	}
	if _, e := db.Comment.UpdateOneID(invalid.Comment.ID).SetContent("changed outside checked snapshot").Save(ctx); e != nil {
		t.Fatal(e)
	}
	first, next, e := r.ListPublic(ctx, "post", a.AppID, 1, 0)
	if e != nil || len(first) != 0 || next != 1 {
		t.Fatal("stale row truncated cursor", next, e)
	}
	second, next, e := r.ListPublic(ctx, "post", a.AppID, 1, next)
	if e != nil || len(second) != 1 || next != 2 {
		t.Fatal("later valid row skipped", next, e)
	}
	last, next, e := r.ListPublic(ctx, "post", a.AppID, 1, next)
	if e != nil || len(last) != 1 || next != -1 {
		t.Fatal("end cursor", next, e)
	}
}
func TestMiniCommentConcurrentReplay(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	ids := make(chan uuid.UUID, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, _, e := r.CreatePending(ctx, &biz.Comment{ArticleSlug: "post", Content: "same"}, a, "intent", biz.MiniCommentRequestHash("post", "same"), time.Minute)
			if e != nil {
				errs <- e
				return
			}
			ids <- v.Comment.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	var last uuid.UUID
	for id := range ids {
		if last != uuid.Nil && last != id {
			t.Error("duplicate winners")
		}
		last = id
	}
	if n, _ := db.Comment.Query().Count(ctx); n != 1 {
		t.Errorf("comments=%d", n)
	}
}

func TestMiniCommentAtomicCreateRollback(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	db.WechatCommentCheck.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			if m.Op().Is(ent.OpCreate) {
				return nil, errors.New("synthetic-private-input")
			}
			return next.Mutate(ctx, m)
		})
	})
	_, _, err := r.CreatePending(ctx, &biz.Comment{ArticleSlug: "post", Content: "hello"}, a, "one", biz.MiniCommentRequestHash("post", "hello"), time.Minute)
	if !errors.Is(err, biz.ErrContentSafetyUnavailable) {
		t.Fatal("raw create failure escaped", err)
	}
	if n, _ := db.Comment.Query().Count(ctx); n != 0 {
		t.Fatal("loser comment was not rolled back")
	}
}
func TestMiniCommentIdentityForeignKeysAndLegacyDeletion(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	c := miniPass(t, r, miniCreate(t, r, a, "one", "hello"), true)
	if err := db.WechatIdentity.DeleteOneID(c.IdentityID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	check, err := db.WechatCommentCheck.Query().Only(ctx)
	if err != nil || check.IdentityID != nil {
		t.Fatal("identity FK must SET NULL", err)
	}
	list, _, err := r.ListPublic(ctx, "post", a.AppID, 20, 0)
	if err != nil || len(list) != 0 {
		t.Fatal("deleted identity remained public", err)
	}
	if err := db.User.DeleteOneID(a.UserID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if n, _ := db.WechatCommentCheck.Query().Count(ctx); n != 0 {
		t.Fatal("author FK must cascade")
	}
}
func TestMiniCommentWebRecheckAndUnavailableRecovery(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	web, e := NewCommentRepo(&Data{db: db}).Create(ctx, &biz.Comment{ArticleSlug: "post", UserID: &a.UserID, DisplayName: "stored web name", Content: "web", Status: biz.CommentStatusPending})
	if e != nil {
		t.Fatal(e)
	}
	c, started, e := r.BeginRecheck(ctx, web.ID, a, time.Minute)
	if e != nil || !started || c.Comment.DisplayName != "stored web name" {
		t.Fatal("web snapshot", e)
	}
	c, e = r.CompleteCheck(ctx, c, biz.MiniCommentVerdict{State: biz.MiniCommentUnavailable, ErrorKind: "unavailable"}, true)
	if e != nil || c.Visible() {
		t.Fatal("unavailable visible", e)
	}
	c, started, e = r.BeginRecheck(ctx, web.ID, a, time.Minute)
	if e != nil || !started || c.Generation != 2 {
		t.Fatal("unavailable not recoverable", e)
	}
	c = miniPass(t, r, c, true)
	if c.Comment.Status != biz.CommentStatusPending {
		t.Fatal("legacy recheck bypassed approval")
	}
	if err := NewCommentRepo(&Data{db: db}).Delete(ctx, web.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := db.WechatCommentCheck.Query().Count(ctx); n != 0 {
		t.Fatal("legacy check should not leave unnecessary tombstone")
	}
}
func TestMiniCommentRequiresCompletePassEvidence(t *testing.T) {
	r, _, a := miniFixture(t)
	ctx := context.Background()
	c := miniCreate(t, r, a, "one", "hello")
	if _, err := r.CompleteCheck(ctx, c, biz.MiniCommentVerdict{State: biz.MiniCommentPass}, true); !errors.Is(err, biz.ErrMiniCommentConflict) {
		t.Fatal("pass without both traces accepted", err)
	}
}

func TestMiniCommentCompletedSnapshotMustMatchItsHash(t *testing.T) {
	r, _, a := miniFixture(t)
	c := miniCreate(t, r, a, "one", "stored body")
	copied := *c
	body := *c.Comment
	copied.Comment = &body
	copied.Comment.Content = "different text checked upstream"
	_, err := r.CompleteCheck(context.Background(), &copied, biz.MiniCommentVerdict{State: biz.MiniCommentPass, NameTraceID: "name", ContentTraceID: "body"}, true)
	if !errors.Is(err, biz.ErrMiniCommentConflict) {
		t.Fatal("upstream result for a different snapshot certified stored text", err)
	}
}

func TestMiniCommentDeletePreservesOtherEvidence(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	remove := miniCreate(t, r, a, "remove", "one")
	keep := miniPass(t, r, miniCreate(t, r, a, "keep", "two"), true)
	if err := NewCommentRepo(&Data{db: db}).Delete(ctx, remove.Comment.ID); err != nil {
		t.Fatal(err)
	}
	current, err := r.FindOwned(ctx, keep.Comment.ID, a.UserID, a.AppID)
	if err != nil || !current.Visible() || current.NameTraceID != keep.NameTraceID || current.InputHash != keep.InputHash {
		t.Fatal("deletion erased another comment's evidence", err)
	}
}

func TestMiniCommentListUsesBoundedBatchQueries(t *testing.T) {
	r, db, a := miniFixture(t)
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		miniPass(t, r, miniCreate(t, r, a, fmt.Sprint(i), fmt.Sprint(i)), true)
	}
	var count atomic.Int32
	db.Intercept(entgo.InterceptFunc(func(next entgo.Querier) entgo.Querier {
		return entgo.QuerierFunc(func(ctx context.Context, q entgo.Query) (entgo.Value, error) { count.Add(1); return next.Query(ctx, q) })
	}))
	public, _, err := r.ListPublic(ctx, "post", a.AppID, 100, 0)
	if err != nil || len(public) != 12 {
		t.Fatal("public batch", err)
	}
	if n := count.Swap(0); n > 3 {
		t.Fatalf("public list used %d queries for 12 rows", n)
	}
	mine, _, err := r.ListMine(ctx, "post", a.UserID, a.AppID, 100, 0)
	if err != nil || len(mine) != 12 {
		t.Fatal("mine batch", err)
	}
	if n := count.Load(); n > 3 {
		t.Fatalf("private list used %d queries for 12 rows", n)
	}
}

func TestMiniCommentMissingTraceCannotRemainPublic(t *testing.T) {
	r, db, actor := miniFixture(t)
	ctx := context.Background()
	record := miniPass(t, r, miniCreate(t, r, actor, "trace-record", "body"), true)
	if _, err := db.WechatCommentCheck.Update().Where(wechatcommentcheck.CommentIDEQ(record.Comment.ID)).SetContentTraceID("").Save(ctx); err != nil {
		t.Fatal(err)
	}
	public, _, err := r.ListPublic(ctx, "post", actor.AppID, 20, 0)
	if err != nil || len(public) != 0 {
		t.Fatal("incomplete evidence remained publicly visible", err)
	}
	owned, err := r.FindOwned(ctx, record.Comment.ID, actor.UserID, actor.AppID)
	if err != nil || !owned.CanRecheck(time.Now()) {
		t.Fatal("incomplete evidence cannot be repaired", err)
	}
}
