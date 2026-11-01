package biz

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

type syncTestIndex struct {
	fakeArticleSearchIndex
	snapshot   *ArticleIndexSnapshot
	fail       bool
	inspectErr error
	retries    int
	retryErr   error
}

func (s *syncTestIndex) GetArticleIndexSnapshot(context.Context, string) (*ArticleIndexSnapshot, error) {
	return s.snapshot, s.inspectErr
}
func (s *syncTestIndex) RetryArticleIndex(_ context.Context, a *Article) error {
	s.retries++
	if s.retryErr != nil {
		return s.retryErr
	}
	if s.fail {
		return context.DeadlineExceeded
	}
	s.snapshot = &ArticleIndexSnapshot{Exists: a.Status == ArticleStatusPublished, Article: a}
	return nil
}
func TestSearchSyncStatesAndRetry(t *testing.T) {
	ctx := context.Background()
	repo := newFakeArticleRepo()
	a := &Article{Slug: "article", Title: "title", ContentMD: "body", Status: ArticleStatusPublished}
	repo.articles[a.Slug] = a
	index := &syncTestIndex{snapshot: &ArticleIndexSnapshot{}}
	uc := NewArticleUsecase(repo, index)
	state, err := uc.GetArticleSearchSync(ctx, a.Slug)
	if err != nil || state.State != SearchSyncMissing {
		t.Fatalf("missing: %+v %v", state, err)
	}
	index.fail = true
	state, err = uc.RetryArticleSearchSync(ctx, a.Slug)
	if err != nil || state.State == SearchSyncSynced {
		t.Fatal(state, err)
	}
	index.fail = false
	state, err = uc.RetryArticleSearchSync(ctx, a.Slug)
	if err != nil || state.State != SearchSyncSynced {
		t.Fatal(state, err)
	}
	index.snapshot.SemanticExpected = true
	state, _ = uc.GetArticleSearchSync(ctx, a.Slug)
	if state.State != SearchSyncPartial || state.CanRetry {
		t.Fatal(state)
	}
	index.snapshot.SemanticRepairable = true
	state, _ = uc.GetArticleSearchSync(ctx, a.Slug)
	if state.State != SearchSyncPartial || !state.CanRetry {
		t.Fatal(state)
	}
	index.snapshot = &ArticleIndexSnapshot{Exists: true, Article: &Article{Title: "old"}}
	state, _ = uc.GetArticleSearchSync(ctx, a.Slug)
	if state.State != SearchSyncStale {
		t.Fatal(state)
	}
	a.Status = ArticleStatusDraft
	state, _ = uc.RetryArticleSearchSync(ctx, a.Slug)
	if state.State != SearchSyncNotRequired {
		t.Fatal(state)
	}
	index.inspectErr = context.DeadlineExceeded
	state, _ = uc.GetArticleSearchSync(ctx, a.Slug)
	if state.State != SearchSyncUnavailable {
		t.Fatal("failed read must not report draft cleanup complete", state)
	}
	disabled := NewArticleUsecase(repo, nil)
	state, _ = disabled.GetArticleSearchSync(ctx, a.Slug)
	if state.State != SearchSyncUnavailable || state.CanRetry {
		t.Fatal(state)
	}
}

type concurrentSyncRepo struct {
	ArticleRepo
	mu      sync.Mutex
	article *Article
}

func (r *concurrentSyncRepo) FindBySlug(context.Context, string) (*Article, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.article == nil {
		return nil, ErrArticleNotFound
	}
	a := *r.article
	return &a, nil
}
func (r *concurrentSyncRepo) set(a *Article) { r.mu.Lock(); defer r.mu.Unlock(); r.article = a }

type blockingSyncIndex struct {
	ArticleSearchIndex
	mu               sync.Mutex
	doc              *Article
	entered, release chan struct{}
}

func (i *blockingSyncIndex) GetArticleIndexSnapshot(context.Context, string) (*ArticleIndexSnapshot, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return &ArticleIndexSnapshot{Exists: i.doc != nil, Article: i.doc}, nil
}
func (i *blockingSyncIndex) RetryArticleIndex(ctx context.Context, a *Article) error {
	close(i.entered)
	<-i.release
	return i.IndexArticle(ctx, a)
}
func (i *blockingSyncIndex) IndexArticle(_ context.Context, a *Article) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	copy := *a
	i.doc = &copy
	return nil
}
func (i *blockingSyncIndex) RemoveArticle(context.Context, string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.doc = nil
	return nil
}
func TestSearchSyncRetryCannotOverwriteLaterSaveOrDelete(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(fmt.Sprint(deleted), func(t *testing.T) {
			r := &concurrentSyncRepo{article: &Article{Slug: "article", Title: "old", ContentMD: "body", Status: ArticleStatusPublished}}
			idx := &blockingSyncIndex{entered: make(chan struct{}), release: make(chan struct{})}
			uc := NewArticleUsecase(r, idx)
			retryDone := make(chan struct{})
			go func() { defer close(retryDone); _, _ = uc.RetryArticleSearchSync(context.Background(), "article") }()
			<-idx.entered
			next := &Article{Slug: "article", Title: "new", ContentMD: "new body", Status: ArticleStatusPublished}
			if deleted {
				r.set(nil)
			} else {
				r.set(next)
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				if deleted {
					uc.dropIndex(context.Background(), "article")
				} else {
					uc.syncIndex(context.Background(), next)
				}
			}()
			close(idx.release)
			<-retryDone
			<-done
			snapshot, _ := idx.GetArticleIndexSnapshot(context.Background(), "article")
			if deleted && snapshot.Exists {
				t.Fatal("resurrected deleted article")
			}
			if !deleted && (snapshot.Article == nil || snapshot.Article.Title != "new") {
				t.Fatal("old retry won")
			}
		})
	}
}

func TestSearchSyncMissingIndexRequiresExplicitInitialization(t *testing.T) {
	repo := newFakeArticleRepo()
	repo.articles["article"] = &Article{Slug: "article", Status: ArticleStatusPublished}
	index := &syncTestIndex{inspectErr: fmt.Errorf("wrapped: %w", ErrArticleSearchIndexMissing)}
	uc := NewArticleUsecase(repo, index)
	for _, retry := range []bool{false, true} {
		var status *ArticleSearchSyncStatus
		var err error
		if retry {
			status, err = uc.RetryArticleSearchSync(context.Background(), "article")
		} else {
			status, err = uc.GetArticleSearchSync(context.Background(), "article")
		}
		if err != nil || status.State != SearchSyncUnavailable || status.CanRetry || status.Message != "搜索索引不存在，请管理员初始化或重建后再试。" {
			t.Fatal(status, err)
		}
	}
	if index.retries != 0 {
		t.Fatal("retry attempted to initialize missing index")
	}
}

func TestSearchSyncIndexDisappearsBeforeRetry(t *testing.T) {
	repo := newFakeArticleRepo()
	repo.articles["article"] = &Article{Slug: "article", Status: ArticleStatusPublished}
	index := &syncTestIndex{snapshot: &ArticleIndexSnapshot{}, retryErr: ErrArticleSearchIndexMissing}
	status, err := NewArticleUsecase(repo, index).RetryArticleSearchSync(context.Background(), "article")
	if err != nil || status.State != SearchSyncUnavailable || status.CanRetry || status.Message != "搜索索引不存在，请管理员初始化或重建后再试。" {
		t.Fatal(status, err)
	}
}
