package data

import (
	"context"
	stdsql "database/sql"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func newTestWechatSyncRepo(t *testing.T) (biz.WechatSyncRepo, *Data) {
	t.Helper()
	db, err := stdsql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	d := &Data{db: client}
	return NewWechatSyncRepo(d), d
}

func testWechatTask(slug, hash, mode, state string) *biz.WechatSyncTask {
	return &biz.WechatSyncTask{
		ID:                 uuid.NewString(),
		Slug:               slug,
		ContentHash:        hash,
		Mode:               mode,
		AccountFingerprint: "fingerprint-1",
		State:              state,
		Article: biz.WechatArticle{
			Title:        "Title " + slug,
			Summary:      "Summary " + slug,
			ContentHTML:  "<p>" + slug + "</p>",
			SourceURL:    "https://makerhao.cn/articles/" + slug,
			Author:       "makerhao",
			ThumbMediaID: "thumb-1",
		},
	}
}

func TestWechatSyncRepoEnqueueDeduplicatesSnapshot(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestWechatSyncRepo(t)
	task := testWechatTask("post", "hash-1", "draft", "queued")
	if err := repo.Enqueue(ctx, task); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	duplicate := testWechatTask(task.Slug, task.ContentHash, task.Mode, "queued")
	duplicate.ID = task.ID
	duplicate.Article.Title = "must not replace the first snapshot"
	if err := repo.Enqueue(ctx, duplicate); err != nil {
		t.Fatalf("Enqueue(duplicate) error = %v", err)
	}
	got, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List() len = %d, want 1", len(got))
	}
	if got[0].ID != task.ID || got[0].AccountFingerprint != task.AccountFingerprint || got[0].Article.Title != task.Article.Title || got[0].Article.ContentHTML != task.Article.ContentHTML {
		t.Fatalf("persisted task = %+v, want original snapshot %+v", got[0], task)
	}
	// The deterministic id includes account/config inputs owned by biz. Two
	// distinct ids must coexist even when article/hash/mode are equal.
	anotherAccount := testWechatTask(task.Slug, task.ContentHash, task.Mode, "queued")
	if err := repo.Enqueue(ctx, anotherAccount); err != nil {
		t.Fatalf("Enqueue(other account/config) error = %v", err)
	}
	got, err = repo.List(ctx, 10, 0)
	if err != nil || len(got) != 2 {
		t.Fatalf("List() after distinct id = %d, %v, want 2", len(got), err)
	}
	if byID, err := repo.Get(ctx, task.ID); err != nil || byID.Article.SourceURL != task.Article.SourceURL {
		t.Fatalf("Get() = %+v, %v", byID, err)
	}
}

func TestWechatSyncRepoRunnableClaimAndSaveCAS(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestWechatSyncRepo(t)
	for _, state := range []string{"queued", "ready", "polling", "succeeded"} {
		if err := repo.Enqueue(ctx, testWechatTask(state, "hash-"+state, "publish", state)); err != nil {
			t.Fatalf("Enqueue(%s) error = %v", state, err)
		}
	}
	runnable, err := repo.Runnable(ctx, 10)
	if err != nil {
		t.Fatalf("Runnable() error = %v", err)
	}
	if len(runnable) != 3 {
		t.Fatalf("Runnable() len = %d, want 3", len(runnable))
	}

	task := runnable[0]
	claimed, err := repo.Claim(ctx, task.ID, task.State, "drafting")
	if err != nil || !claimed {
		t.Fatalf("Claim() = %v, %v, want true", claimed, err)
	}
	claimed, err = repo.Claim(ctx, task.ID, task.State, "drafting")
	if err != nil || claimed {
		t.Fatalf("Claim(stale) = %v, %v, want false", claimed, err)
	}
	got, err := repo.Get(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "drafting" || got.Attempts != 1 {
		t.Fatalf("claimed task = %+v, want drafting attempts=1", got)
	}

	// Save receives the worker's pre-claim snapshot (Attempts=0); it must not
	// overwrite the increment performed atomically by Claim.
	task.State = "ready"
	task.DraftID = "draft-123"
	task.LastError = ""
	if err := repo.Save(ctx, task, "drafting"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	task.State = "failed"
	task.LastError = "safe-code"
	if err := repo.Save(ctx, task, "drafting"); err == nil {
		t.Fatal("Save(stale expected state) error = nil, want CAS conflict")
	} else if !biz.ErrWechatSyncConflict.Is(err) {
		t.Fatalf("Save(stale expected state) error = %v, want sync conflict", err)
	}
	persisted, err := repo.Get(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.State != "ready" || persisted.DraftID != "draft-123" || persisted.Attempts != 1 {
		t.Fatalf("saved task = %+v", persisted)
	}
	claimed, err = repo.Claim(ctx, task.ID, "ready", "checking")
	if err != nil || !claimed {
		t.Fatalf("Claim(ready) = %v, %v", claimed, err)
	}
	task.State = "polling"
	task.LastError = "response body with spaces"
	if err := repo.Save(ctx, task, "checking"); err == nil {
		t.Fatal("Save(unsafe error text) error = nil")
	}
}

func TestWechatSyncRepoRunnableAndClaimHonorNextAttemptAt(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestWechatSyncRepo(t)
	future := testWechatTask("future", "hash-future", "publish", "polling")
	future.NextAttemptAt = time.Now().Add(time.Hour)
	past := testWechatTask("past", "hash-past", "publish", "polling")
	past.NextAttemptAt = time.Now().Add(-time.Minute)
	for _, task := range []*biz.WechatSyncTask{future, past} {
		if err := repo.Enqueue(ctx, task); err != nil {
			t.Fatalf("Enqueue(%s) error = %v", task.Slug, err)
		}
	}
	runnable, err := repo.Runnable(ctx, 10)
	if err != nil {
		t.Fatalf("Runnable() error = %v", err)
	}
	if len(runnable) != 1 || runnable[0].ID != past.ID || !runnable[0].NextAttemptAt.Equal(past.NextAttemptAt) {
		t.Fatalf("Runnable() = %+v, want only due task", runnable)
	}
	if claimed, err := repo.Claim(ctx, future.ID, "polling", "checking"); err != nil || claimed {
		t.Fatalf("Claim(future) = %v, %v, want false", claimed, err)
	}
	if claimed, err := repo.Claim(ctx, past.ID, "polling", "checking"); err != nil || !claimed {
		t.Fatalf("Claim(past) = %v, %v, want true", claimed, err)
	}
	past.State = "polling"
	past.NextAttemptAt = time.Now().Add(time.Hour)
	if err := repo.Save(ctx, past, "checking"); err != nil {
		t.Fatalf("Save(future retry) error = %v", err)
	}
	if claimed, err := repo.Claim(ctx, past.ID, "polling", "checking"); err != nil || claimed {
		t.Fatalf("Claim(saved future) = %v, %v, want false", claimed, err)
	}
	past.NextAttemptAt = time.Time{}
	if err := repo.Save(ctx, past, "polling"); err != nil {
		t.Fatalf("Save(clear retry) error = %v", err)
	}
	cleared, err := repo.Get(ctx, past.ID)
	if err != nil || !cleared.NextAttemptAt.IsZero() {
		t.Fatalf("Get(cleared retry) = %+v, %v", cleared, err)
	}
	if claimed, err := repo.Claim(ctx, past.ID, "polling", "checking"); err != nil || !claimed {
		t.Fatalf("Claim(cleared retry) = %v, %v, want true", claimed, err)
	}
}

func TestWechatSyncRepoRecoverSeparatesWritesFromStatusQueries(t *testing.T) {
	ctx := context.Background()
	repo, d := newTestWechatSyncRepo(t)
	stale := time.Now().Add(-2 * time.Hour)
	for _, state := range []string{"drafting", "submitting", "checking", "queued"} {
		task := testWechatTask(state, "hash-"+state, "publish", state)
		if err := repo.Enqueue(ctx, task); err != nil {
			t.Fatal(err)
		}
		id, err := uuid.Parse(task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.db.WechatSyncTask.UpdateOneID(id).SetUpdatedAt(stale).Save(ctx); err != nil {
			t.Fatalf("age task: %v", err)
		}
	}
	if err := repo.Recover(ctx, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	all, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	errorsBySlug := map[string]string{}
	for _, task := range all {
		states[task.Slug] = task.State
		errorsBySlug[task.Slug] = task.LastError
	}
	for _, slug := range []string{"drafting", "submitting"} {
		if states[slug] != "unknown" {
			t.Fatalf("state[%s] = %q, want unknown", slug, states[slug])
		}
		if errorsBySlug[slug] != "recovery_ambiguous" {
			t.Fatalf("last_error[%s] = %q", slug, errorsBySlug[slug])
		}
	}
	if states["checking"] != "polling" || errorsBySlug["checking"] != "recovery_query_retry" {
		t.Fatalf("checking recovered as state=%q error=%q", states["checking"], errorsBySlug["checking"])
	}
	if states["queued"] != "queued" {
		t.Fatalf("queued recovered as %q", states["queued"])
	}
}

func TestArticleUpdateAndEnqueueIsAtomic(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestArticleRepo(t)
	concrete := repo.(*articleRepo)
	created, err := repo.CreateArticle(ctx, &biz.Article{Slug: "atomic", Title: "before", ContentMD: "before"})
	if err != nil {
		t.Fatal(err)
	}
	created.Title = "after"
	created.ContentMD = "after"
	task := testWechatTask(created.Slug, "hash-atomic", "publish", "queued")
	if _, err := concrete.UpdateArticleAndEnqueue(ctx, created, task); err != nil {
		t.Fatalf("UpdateArticleAndEnqueue() error = %v", err)
	}
	if _, err := NewWechatSyncRepo(concrete.data).Get(ctx, task.ID); err != nil {
		t.Fatalf("queued task missing: %v", err)
	}

	created.Title = "must roll back"
	bad := testWechatTask(created.Slug, "hash-invalid", "publish", "queued")
	bad.ID = "not-a-uuid"
	if _, err := concrete.UpdateArticleAndEnqueue(ctx, created, bad); err == nil {
		t.Fatal("UpdateArticleAndEnqueue(invalid task) error = nil")
	}
	persisted, err := repo.FindBySlug(ctx, created.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Title != "after" {
		t.Fatalf("article title = %q, want transaction rollback to after", persisted.Title)
	}
}
