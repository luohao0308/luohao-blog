package biz

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"sync"
	"testing"
	"time"
)

type syncMemoryRepo struct {
	mu    sync.Mutex
	tasks map[string]*WechatSyncTask
}

func newSyncMemoryRepo() *syncMemoryRepo { return &syncMemoryRepo{tasks: map[string]*WechatSyncTask{}} }
func (r *syncMemoryRepo) Enqueue(_ context.Context, t *WechatSyncTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[t.ID]; !ok {
		copy := *t
		copy.CreatedAt = time.Now()
		copy.UpdatedAt = time.Now()
		r.tasks[t.ID] = &copy
	}
	return nil
}
func (r *syncMemoryRepo) Get(_ context.Context, id string) (*WechatSyncTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, ErrWechatSyncNotFound
	}
	copy := *t
	return &copy, nil
}
func (r *syncMemoryRepo) List(_ context.Context, _, _ int) ([]*WechatSyncTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*WechatSyncTask{}
	for _, t := range r.tasks {
		copy := *t
		out = append(out, &copy)
	}
	return out, nil
}
func (r *syncMemoryRepo) Runnable(ctx context.Context, limit int) ([]*WechatSyncTask, error) {
	all, _ := r.List(ctx, limit, 0)
	out := []*WechatSyncTask{}
	for _, t := range all {
		if (t.State == "queued" || t.State == "ready" || t.State == "polling") && !t.NextAttemptAt.After(time.Now()) {
			out = append(out, t)
		}
	}
	return out, nil
}
func (r *syncMemoryRepo) Claim(_ context.Context, id, from, to string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.tasks[id]
	if t.State != from {
		return false, nil
	}
	t.State = to
	t.Attempts++
	t.UpdatedAt = time.Now()
	return true, nil
}
func (r *syncMemoryRepo) Save(_ context.Context, t *WechatSyncTask, expected string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.tasks[t.ID]
	if old.State != expected {
		return ErrWechatSyncConflict
	}
	copy := *t
	copy.Attempts = old.Attempts
	copy.UpdatedAt = time.Now()
	r.tasks[t.ID] = &copy
	return nil
}
func (r *syncMemoryRepo) Recover(_ context.Context, cutoff time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tasks {
		if (t.State == "drafting" || t.State == "submitting" || t.State == "checking") && t.UpdatedAt.Before(cutoff) {
			t.State = "unknown"
		}
	}
	return nil
}

type syncPublisher struct {
	drafts, submits, queries int
	err                      error
	pending                  bool
	mode, key                string
}

func (p *syncPublisher) CreateDraft(context.Context, WechatArticle) (string, error) {
	p.drafts++
	if p.err != nil {
		return "", p.err
	}
	return "draft-1", nil
}
func (p *syncPublisher) Submit(_ context.Context, _ string, mode, key string) (string, error) {
	p.submits++
	p.mode = mode
	p.key = key
	if p.err != nil {
		return "", p.err
	}
	return "remote-1", nil
}
func (p *syncPublisher) Status(context.Context, string, string) (*WechatPublishResult, error) {
	p.queries++
	return &WechatPublishResult{Pending: p.pending, Succeeded: !p.pending, ArticleURL: "https://mp.weixin.qq.com/s/fixture"}, p.err
}
func syncFixture(mode string) (*WechatSyncUsecase, *syncMemoryRepo, *syncPublisher, *fakeArticleRepo, *Article) {
	repo := newSyncMemoryRepo()
	pub := &syncPublisher{}
	articles := newFakeArticleRepo()
	a := &Article{ID: uuid.Must(uuid.NewV7()), Slug: "wechat-fixture", Title: "测试文章", ContentMD: "hello", ContentHTML: "<p>hello</p>", Status: ArticleStatusPublished}
	articles.articles[a.Slug] = a
	return NewWechatSyncUsecase(repo, articles, pub, WechatSyncSettings{Enabled: true, Configured: true, Mode: mode, SiteURL: "https://blog.example.com", ThumbMediaID: "cover", AccountID: "public-account", AccountFingerprint: "account-a"}), repo, pub, articles, a
}
func tick(t *testing.T, uc *WechatSyncUsecase) {
	t.Helper()
	if err := uc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func TestWechatSyncDeliveryLifecycle(t *testing.T) {
	for _, mode := range []string{"draft", "publish", "mass"} {
		t.Run(mode, func(t *testing.T) {
			uc, repo, pub, _, a := syncFixture(mode)
			task, err := uc.SyncArticle(context.Background(), a.Slug)
			if err != nil {
				t.Fatal(err)
			}
			again, err := uc.SyncArticle(context.Background(), a.Slug)
			if err != nil || again.ID != task.ID {
				t.Fatal("duplicate version")
			}
			tick(t, uc)
			tick(t, uc)
			if mode == "draft" {
				got, _ := repo.Get(context.Background(), task.ID)
				if got.State != "drafted" || pub.submits != 0 {
					t.Fatalf("draft: %+v", got)
				}
				return
			}
			got, _ := repo.Get(context.Background(), task.ID)
			if got.State != "polling" {
				t.Fatalf("must not report submission as success: %+v", got)
			}
			pub.pending = true
			tick(t, uc)
			got, _ = repo.Get(context.Background(), task.ID)
			if got.State != "polling" {
				t.Fatal(got.State)
			}
			pub.pending = false
			repo.clearDue()
			tick(t, uc)
			tick(t, uc)
			got, _ = repo.Get(context.Background(), task.ID)
			if got.State != "succeeded" || pub.drafts != 1 || pub.submits != 1 || len(pub.key) != 32 || pub.mode != mode {
				t.Fatalf("%+v; %+v", got, pub)
			}
		})
	}
}
func TestWechatSyncUnknownWritesCannotRetry(t *testing.T) {
	for _, stage := range []string{"draft", "submit"} {
		t.Run(stage, func(t *testing.T) {
			uc, repo, pub, _, a := syncFixture("mass")
			task, _ := uc.SyncArticle(context.Background(), a.Slug)
			if stage == "submit" {
				tick(t, uc)
			}
			pub.err = &WechatRemoteError{Code: "timeout", Ambiguous: true}
			tick(t, uc)
			tick(t, uc)
			got, _ := repo.Get(context.Background(), task.ID)
			if got.State != "unknown" {
				t.Fatal(got.State)
			}
			if _, err := uc.Retry(context.Background(), task.ID); err != ErrWechatSyncConflict {
				t.Fatal(err)
			}
			if pub.drafts != 1 || pub.submits > 1 {
				t.Fatal("ambiguous write repeated")
			}
		})
	}
}
func TestWechatSyncFailuresCanRetryWithoutNewDraft(t *testing.T) {
	uc, repo, pub, _, a := syncFixture("publish")
	task, _ := uc.SyncArticle(context.Background(), a.Slug)
	tick(t, uc)
	pub.err = &WechatRemoteError{Code: "45009"}
	tick(t, uc)
	got, _ := repo.Get(context.Background(), task.ID)
	if !WechatTaskCanRetry(got) {
		t.Fatal(got.State)
	}
	if _, err := uc.Retry(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	pub.err = nil
	tick(t, uc)
	tick(t, uc)
	if pub.drafts != 1 || pub.submits != 2 {
		t.Fatal("lost draft on retry")
	}
}
func TestWechatSyncStatusErrorsOnlyRepeatQueries(t *testing.T) {
	uc, repo, pub, _, a := syncFixture("publish")
	task, _ := uc.SyncArticle(context.Background(), a.Slug)
	tick(t, uc)
	tick(t, uc)
	pub.err = fmt.Errorf("network")
	tick(t, uc)
	repo.clearDue()
	tick(t, uc)
	pub.err = nil
	repo.clearDue()
	tick(t, uc)
	got, _ := repo.Get(context.Background(), task.ID)
	if got.State != "succeeded" || pub.submits != 1 {
		t.Fatalf("%+v %+v", got, pub)
	}
}
func TestWechatSyncCancelsStaleAndUnpublishedArticles(t *testing.T) {
	for _, change := range []string{"content", "draft", "delete"} {
		t.Run(change, func(t *testing.T) {
			uc, repo, pub, articles, a := syncFixture("publish")
			task, _ := uc.SyncArticle(context.Background(), a.Slug)
			switch change {
			case "content":
				a.ContentHTML = "<p>new</p>"
			case "draft":
				a.Status = ArticleStatusDraft
			case "delete":
				delete(articles.articles, a.Slug)
			}
			tick(t, uc)
			got, _ := repo.Get(context.Background(), task.ID)
			if got.State != "cancelled" || pub.drafts != 0 {
				t.Fatal("stale article delivered")
			}
		})
	}
}
func TestWechatSyncDisabledAndStableIdentity(t *testing.T) {
	uc, _, pub, _, a := syncFixture("mass")
	before := uc.Task(a).ID
	a.ViewCount = 100
	a.UpdatedAt = time.Now()
	if uc.Task(a).ID != before {
		t.Fatal("counters changed identity")
	}
	uc.settings.AccountFingerprint = "account-b"
	if uc.Task(a).ID == before {
		t.Fatal("account switch reused remote task")
	}
	uc.settings.Enabled = false
	if uc.Task(a) != nil {
		t.Fatal("disabled task")
	}
	tick(t, uc)
	if pub.drafts != 0 {
		t.Fatal("disabled client used")
	}
}

type syncAtomicArticleRepo struct {
	*fakeArticleRepo
	queue *syncMemoryRepo
	fail  bool
}

func (r *syncAtomicArticleRepo) UpdateArticleAndEnqueue(ctx context.Context, a *Article, t *WechatSyncTask) (*Article, error) {
	if r.fail {
		return nil, fmt.Errorf("atomic failure")
	}
	if err := r.queue.Enqueue(ctx, t); err != nil {
		return nil, err
	}
	return r.UpdateArticle(ctx, a)
}
func TestArticlePublicationQueuesOnceAndKeepsEditsLocal(t *testing.T) {
	syncUC, queue, _, articles, a := syncFixture("mass")
	a.Status = ArticleStatusDraft
	atomic := &syncAtomicArticleRepo{fakeArticleRepo: articles, queue: queue}
	syncUC.articles = atomic
	uc := NewArticleUsecaseWithWechat(atomic, nil, syncUC)
	published := *a
	published.Status = ArticleStatusPublished
	if _, err := uc.UpdateArticle(context.Background(), &published); err != nil {
		t.Fatal(err)
	}
	edit := published
	edit.Title = "a later edit"
	edit.CoverURL = "/v1/assets/media/0123456789abcdef0123456789abcdef.png"
	edit.Featured = true
	if _, err := uc.UpdateArticle(context.Background(), &edit); err != nil {
		t.Fatal(err)
	}
	if len(queue.tasks) != 1 {
		t.Fatal("edit queued a second notification")
	}
	atomic.fail = true
	edit.Status = ArticleStatusDraft
	articles.articles[a.Slug] = &edit
	republish := edit
	republish.Status = ArticleStatusPublished
	if _, err := uc.UpdateArticle(context.Background(), &republish); err == nil {
		t.Fatal("atomic failure hidden")
	}
	if articles.articles[a.Slug].Status != ArticleStatusDraft {
		t.Fatal("failed transaction published")
	}
}

func TestWechatSyncAccountSwitchNeverUsesNewAccountForOldTasks(t *testing.T) {
	for _, stage := range []string{"queued", "ready", "polling"} {
		t.Run(stage, func(t *testing.T) {
			uc, repo, pub, _, a := syncFixture("publish")
			task, _ := uc.SyncArticle(context.Background(), a.Slug)
			if stage != "queued" {
				tick(t, uc)
			}
			if stage == "polling" {
				tick(t, uc)
			}
			drafts, submits, queries := pub.drafts, pub.submits, pub.queries
			uc.settings.AccountFingerprint = "new-account"
			tick(t, uc)
			got, _ := repo.Get(context.Background(), task.ID)
			expected := "unknown"
			if stage == "queued" {
				expected = "cancelled"
			}
			if got.State != expected || pub.drafts != drafts || pub.submits != submits || pub.queries != queries {
				t.Fatalf("account switch: %+v %+v", got, pub)
			}
		})
	}
}

func TestWechatSyncSiteURLSwitchNeverRepublishesOldTasks(t *testing.T) {
	const (
		oldSiteURL = "http://193.112.128.245"
		newSiteURL = "https://makerhao.cn"
	)

	t.Run("changes task identity and source URL", func(t *testing.T) {
		uc, _, _, _, a := syncFixture("publish")
		uc.settings.SiteURL = oldSiteURL
		oldTask := uc.Task(a)
		uc.settings.SiteURL = newSiteURL
		newTask := uc.Task(a)
		if oldTask.ID == newTask.ID {
			t.Fatal("site URL switch reused task identity")
		}
		if oldTask.Article.SourceURL != oldSiteURL+"/posts/"+a.Slug {
			t.Fatalf("old source URL: %q", oldTask.Article.SourceURL)
		}
		if newTask.Article.SourceURL != newSiteURL+"/posts/"+a.Slug {
			t.Fatalf("new source URL: %q", newTask.Article.SourceURL)
		}
	})

	for _, stage := range []string{"queued", "ready"} {
		t.Run(stage+" is cancelled before remote write", func(t *testing.T) {
			uc, repo, pub, _, a := syncFixture("publish")
			uc.settings.SiteURL = oldSiteURL
			task, err := uc.SyncArticle(context.Background(), a.Slug)
			if err != nil {
				t.Fatal(err)
			}
			if stage == "ready" {
				tick(t, uc)
			}
			before, err := repo.Get(context.Background(), task.ID)
			if err != nil {
				t.Fatal(err)
			}
			drafts, submits := pub.drafts, pub.submits
			uc.settings.SiteURL = newSiteURL
			tick(t, uc)
			got, err := repo.Get(context.Background(), task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != "cancelled" {
				t.Fatalf("old %s task state: %s", stage, got.State)
			}
			if pub.drafts != drafts || pub.submits != submits {
				t.Fatalf("old %s task performed remote write: drafts=%d submits=%d", stage, pub.drafts, pub.submits)
			}
			if got.DraftID != before.DraftID {
				t.Fatalf("old %s task lost draft ID: before=%q after=%q", stage, before.DraftID, got.DraftID)
			}
			if stage == "ready" && got.DraftID == "" {
				t.Fatal("ready task fixture did not retain its existing draft ID")
			}
		})
	}

	t.Run("failed retry is rejected without mutation", func(t *testing.T) {
		uc, repo, pub, _, a := syncFixture("publish")
		uc.settings.SiteURL = oldSiteURL
		task, err := uc.SyncArticle(context.Background(), a.Slug)
		if err != nil {
			t.Fatal(err)
		}
		pub.err = &WechatRemoteError{Code: "45009"}
		tick(t, uc)
		before, err := repo.Get(context.Background(), task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if before.State != "failed" {
			t.Fatalf("failed fixture state: %s", before.State)
		}
		drafts, submits, queries := pub.drafts, pub.submits, pub.queries
		uc.settings.SiteURL = newSiteURL
		if _, err := uc.Retry(context.Background(), task.ID); err != ErrWechatSyncConflict {
			t.Fatalf("retry old failed task: %v", err)
		}
		after, err := repo.Get(context.Background(), task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if after.State != before.State || after.Attempts != before.Attempts || after.LastError != before.LastError {
			t.Fatalf("rejected retry mutated task: before=%+v after=%+v", before, after)
		}
		if pub.drafts != drafts || pub.submits != submits || pub.queries != queries {
			t.Fatalf("rejected retry called publisher: drafts=%d submits=%d queries=%d", pub.drafts, pub.submits, pub.queries)
		}
	})
}

func (r *syncMemoryRepo) clearDue() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, task := range r.tasks {
		task.NextAttemptAt = time.Time{}
	}
}
func TestWechatSyncQueryBackoffAndPermanentErrors(t *testing.T) {
	for _, code := range []string{"40014", "45009", "48001"} {
		t.Run(code, func(t *testing.T) {
			uc, repo, pub, _, a := syncFixture("publish")
			task, _ := uc.SyncArticle(context.Background(), a.Slug)
			tick(t, uc)
			tick(t, uc)
			pub.err = &WechatRemoteError{Code: code}
			tick(t, uc)
			got, _ := repo.Get(context.Background(), task.ID)
			if got.LastError != code {
				t.Fatalf("lost error: %+v", got)
			}
			if code == "48001" {
				if got.State != "unknown" {
					t.Fatal(got.State)
				}
			} else {
				if got.State != "polling" || !got.NextAttemptAt.After(time.Now()) {
					t.Fatalf("no persistent backoff: %+v", got)
				}
			}
			count := pub.queries
			tick(t, uc)
			if pub.queries != count {
				t.Fatal("poll repeated before due")
			}
		})
	}
}
