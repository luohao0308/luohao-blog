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
}

func newFakeArticleRepo() *fakeArticleRepo {
	return &fakeArticleRepo{articles: map[string]*Article{}}
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

func (f *fakeArticleRepo) DeleteArticle(_ context.Context, slug string) error {
	if _, ok := f.articles[slug]; !ok {
		return ErrArticleNotFound
	}
	delete(f.articles, slug)
	return nil
}

func TestArticleUsecaseCreateForcesDraft(t *testing.T) {
	uc := NewArticleUsecase(newFakeArticleRepo())

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
	uc := NewArticleUsecase(newFakeArticleRepo())

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
	uc := NewArticleUsecase(repo)
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
	uc := NewArticleUsecase(repo)

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

func TestArticleUsecaseUpdateDeletedRejected(t *testing.T) {
	ctx := context.Background()
	repo := newFakeArticleRepo()
	uc := NewArticleUsecase(repo)

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
