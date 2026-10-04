package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

// fakeArticleRepo records calls and keeps articles in memory, so usecase tests
// exercise validation and status transitions without a storage driver.
type fakeArticleRepo struct {
	articles map[string]*Article
	public   bool
	views    map[string]uint64
	likes    map[string]uint64
}

type fakeArticleSearchIndex struct {
	indexed []*Article
	removed []string
	results []string
	err     error
}

func (f *fakeArticleSearchIndex) IndexArticle(_ context.Context, a *Article) error {
	f.indexed = append(f.indexed, a)
	return f.err
}
func (f *fakeArticleSearchIndex) RemoveArticle(_ context.Context, slug string) error {
	f.removed = append(f.removed, slug)
	return f.err
}
func (f *fakeArticleSearchIndex) Search(_ context.Context, _ string, _, _ int) ([]string, error) {
	return f.results, f.err
}
func (f *fakeArticleSearchIndex) RecreateIndex(_ context.Context) error {
	return f.err
}

func newFakeArticleRepo() *fakeArticleRepo {
	return &fakeArticleRepo{articles: map[string]*Article{}, views: map[string]uint64{}, likes: map[string]uint64{}}
}

func (f *fakeArticleRepo) FindBySlug(_ context.Context, slug string) (*Article, error) {
	a, ok := f.articles[slug]
	if !ok {
		return nil, ErrArticleNotFound
	}
	return a, nil
}

func (f *fakeArticleRepo) ListArticles(_ context.Context, opts ...ListOption) ([]*Article, error) {
	var options ListOptions
	for _, opt := range opts {
		opt(&options)
	}
	f.public = options.Public
	articles := make([]*Article, 0, len(f.articles))
	for _, article := range f.articles {
		if options.Public && article.Status != ArticleStatusPublished {
			continue
		}
		articles = append(articles, article)
	}
	return articles, nil
}

func (f *fakeArticleRepo) CreateArticle(_ context.Context, a *Article) (*Article, error) {
	if _, ok := f.articles[a.Slug]; ok {
		return nil, ErrArticleSlugConflict
	}
	a.ID = uuid.Must(uuid.NewV7())
	f.articles[a.Slug] = a
	return a, nil
}

func (f *fakeArticleRepo) UpdateArticle(_ context.Context, a *Article) (*Article, error) {
	if _, ok := f.articles[a.Slug]; !ok {
		return nil, ErrArticleNotFound
	}
	f.articles[a.Slug] = a
	return a, nil
}

// IncrementView counts every call as a fresh view: the dedup window is a
// data-layer concern the fake does not model.
func (f *fakeArticleRepo) IncrementView(_ context.Context, slug, _ string) (uint64, bool, error) {
	a, ok := f.articles[slug]
	if !ok {
		return 0, false, ErrArticleNotFound
	}
	f.views[slug]++
	a.ViewCount = f.views[slug]
	return a.ViewCount, true, nil
}

// IncrementLike mirrors IncrementView for the like counter.
func (f *fakeArticleRepo) IncrementLike(_ context.Context, slug, _ string) (uint64, bool, error) {
	a, ok := f.articles[slug]
	if !ok {
		return 0, false, ErrArticleNotFound
	}
	f.likes[slug]++
	a.LikeCount = f.likes[slug]
	return a.LikeCount, true, nil
}

func (f *fakeArticleRepo) DeleteArticle(_ context.Context, slug string) error {
	if _, ok := f.articles[slug]; !ok {
		return ErrArticleNotFound
	}
	delete(f.articles, slug)
	return nil
}

func TestArticleUsecaseCreateForcesDraft(t *testing.T) {
	uc := NewArticleUsecase(newFakeArticleRepo(), nil)

	created, err := uc.CreateArticle(context.Background(), &Article{
		Slug:      "create-force-draft",
		Title:     "t",
		ContentMD: "# 渲染测试",
		Status:    ArticleStatusPublished, // must be ignored
	})
	if err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}
	if created.Status != ArticleStatusDraft {
		t.Fatalf("status = %d, want draft", created.Status)
	}
	if created.PublishedAt != nil {
		t.Fatal("published_at set on create, want nil")
	}
	if !strings.Contains(created.ContentHTML, "<h1") || !strings.Contains(created.ContentHTML, "渲染测试") {
		t.Fatalf("content_html = %q, want rendered heading", created.ContentHTML)
	}
}

func TestArticleUsecaseSlugValidation(t *testing.T) {
	uc := NewArticleUsecase(newFakeArticleRepo(), nil)

	for _, slug := range []string{"", "Bad Slug", "UPPER", "-lead", "trail-", "dou--ble", string(make([]byte, 65))} {
		if _, err := uc.CreateArticle(context.Background(), &Article{Slug: slug, Title: "t", ContentMD: "c"}); !kratoserrors.IsBadRequest(err) {
			t.Fatalf("CreateArticle(slug=%q) error = %v, want bad request", slug, err)
		}
	}
	if _, err := uc.CreateArticle(context.Background(), &Article{Slug: "ok-slug-1", Title: "t", ContentMD: "c"}); err != nil {
		t.Fatalf("CreateArticle(ok) error = %v", err)
	}
}

func TestArticleUsecasePublicReadsHideDrafts(t *testing.T) {
	repo := newFakeArticleRepo()
	uc := NewArticleUsecase(repo, nil)
	repo.articles["draft"] = &Article{Slug: "draft", Status: ArticleStatusDraft}
	repo.articles["published"] = &Article{Slug: "published", Status: ArticleStatusPublished}

	if _, err := uc.GetPublicArticle(context.Background(), "draft"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("GetPublicArticle(draft) error = %v, want not found", err)
	}
	if got, err := uc.GetPublicArticle(context.Background(), "published"); err != nil || got.Slug != "published" {
		t.Fatalf("GetPublicArticle(published) = %+v, %v", got, err)
	}
	articles, err := uc.ListArticles(context.Background(), ListPublic())
	if err != nil {
		t.Fatalf("ListArticles(public) error = %v", err)
	}
	if !repo.public {
		t.Fatal("ListArticles(public) did not pass the public query boundary")
	}
	if len(articles) != 1 || articles[0].Slug != "published" {
		t.Fatalf("ListArticles(public) = %+v, want only published article", articles)
	}
}

func TestArticleUsecasePublishStampsPublishedAt(t *testing.T) {
	ctx := context.Background()
	repo := newFakeArticleRepo()
	uc := NewArticleUsecase(repo, nil)

	if _, err := uc.CreateArticle(ctx, &Article{Slug: "publish-me", Title: "t", ContentMD: "c"}); err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}

	published, err := uc.UpdateArticle(ctx, &Article{
		Slug:      "publish-me",
		Title:     "t",
		ContentMD: "c",
		Status:    ArticleStatusPublished,
		Tags:      []string{"go"},
	})
	if err != nil {
		t.Fatalf("UpdateArticle(to published) error = %v", err)
	}
	if published.PublishedAt == nil {
		t.Fatal("published_at not stamped on first publish")
	}

	// A later update keeps the original publication timestamp.
	time.Sleep(2 * time.Millisecond)
	again, err := uc.UpdateArticle(ctx, &Article{
		Slug:      "publish-me",
		Title:     "t2",
		ContentMD: "c2",
		Status:    ArticleStatusPublished,
	})
	if err != nil {
		t.Fatalf("UpdateArticle(still published) error = %v", err)
	}
	if !again.PublishedAt.Equal(*published.PublishedAt) {
		t.Fatalf("published_at changed on second update: %v -> %v", *published.PublishedAt, *again.PublishedAt)
	}
}

func TestArticleUsecaseSearchIndexIsBestEffort(t *testing.T) {
	ctx := context.Background()
	repo := newFakeArticleRepo()
	index := &fakeArticleSearchIndex{err: context.DeadlineExceeded}
	uc := NewArticleUsecase(repo, index)
	repo.articles["indexed"] = &Article{Slug: "indexed", Title: "old", ContentMD: "old", Status: ArticleStatusDraft}
	updated, err := uc.UpdateArticle(ctx, &Article{Slug: "indexed", Title: "new", ContentMD: "body", Status: ArticleStatusPublished})
	if err != nil || updated.Status != ArticleStatusPublished {
		t.Fatalf("UpdateArticle() = %+v, %v; index failure must not block write", updated, err)
	}
	if len(index.indexed) != 1 {
		t.Fatalf("index calls = %d, want 1", len(index.indexed))
	}
	if err := uc.DeleteArticle(ctx, "indexed"); err != nil {
		t.Fatalf("DeleteArticle() error = %v", err)
	}
	if len(index.removed) != 1 || index.removed[0] != "indexed" {
		t.Fatalf("remove calls = %v, want indexed", index.removed)
	}
}

func TestArticleUsecaseSearchHydratesPublishedOnly(t *testing.T) {
	repo := newFakeArticleRepo()
	repo.articles["published"] = &Article{Slug: "published", Status: ArticleStatusPublished}
	repo.articles["draft"] = &Article{Slug: "draft", Status: ArticleStatusDraft}
	index := &fakeArticleSearchIndex{results: []string{"draft", "published"}}
	articles, err := NewArticleUsecase(repo, index).SearchArticles(context.Background(), "query", 10, 0)
	if err != nil || len(articles) != 1 || articles[0].Slug != "published" {
		t.Fatalf("SearchArticles() = %+v, %v; want published hit only", articles, err)
	}
}

func TestArticleUsecaseUpdateDeletedRejected(t *testing.T) {
	ctx := context.Background()
	repo := newFakeArticleRepo()
	uc := NewArticleUsecase(repo, nil)

	if _, err := uc.CreateArticle(ctx, &Article{Slug: "delete-via-update", Title: "t", ContentMD: "c"}); err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}

	// DELETED is not a reachable status through UpdateArticle.
	if _, err := uc.UpdateArticle(ctx, &Article{Slug: "delete-via-update", Title: "t", ContentMD: "c", Status: ArticleStatusDeleted}); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("UpdateArticle(to deleted) error = %v, want bad request", err)
	}

	// Soft delete goes through DeleteArticle.
	if err := uc.DeleteArticle(ctx, "delete-via-update"); err != nil {
		t.Fatalf("DeleteArticle() error = %v", err)
	}
	if _, err := uc.GetArticle(ctx, "delete-via-update"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("GetArticle() after delete error = %v, want not found", err)
	}
}

func TestArticleUsecaseMarkViewed(t *testing.T) {
	ctx := context.Background()
	repo := newFakeArticleRepo()
	uc := NewArticleUsecase(repo, nil)
	if _, err := uc.CreateArticle(ctx, &Article{Slug: "viewed-article", Title: "t", ContentMD: "c"}); err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}

	// Drafts are not publicly readable, so views must not count either.
	if _, counted, err := uc.MarkViewed(ctx, "viewed-article", "client-a"); !kratoserrors.IsNotFound(err) || counted {
		t.Fatalf("MarkViewed(draft) = (%v, %v), want not found and not counted", err, counted)
	}

	repo.articles["viewed-article"].Status = ArticleStatusPublished
	count, counted, err := uc.MarkViewed(ctx, "viewed-article", "client-a")
	if err != nil || !counted || count != 1 {
		t.Fatalf("MarkViewed(first) = (%d, %v, %v), want (1, true, nil)", count, counted, err)
	}

	// Malformed slugs are rejected before touching the repo.
	if _, _, err := uc.MarkViewed(ctx, "INVALID SLUG", "client-a"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("MarkViewed(bad slug) error = %v, want bad request", err)
	}
}
