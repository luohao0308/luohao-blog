package data

import (
	"context"
	stdsql "database/sql"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"go.einride.tech/aip/ordering"
	_ "modernc.org/sqlite"
)

// newTestArticleRepo opens an in-memory SQLite database with the schema
// applied, so repo tests exercise real ent queries at the storage boundary.
// The modernc driver registers itself as "sqlite", so the ent dialect is set
// explicitly.
func newTestArticleRepo(t *testing.T) (biz.ArticleRepo, *ent.Client) {
	t.Helper()
	db, err := stdsql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() {
		_ = client.Close()
	})
	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return NewArticleRepo(&Data{db: client}), client
}

// orderByCreatedAt keeps list assertions deterministic.
func orderByCreatedAt() biz.ListOption {
	return biz.ListOrderBy(ordering.OrderBy{
		Fields: []ordering.Field{{Path: "created_at"}},
	})
}

func TestArticleRepoCRUD(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestArticleRepo(t)

	created, err := repo.CreateArticle(ctx, &biz.Article{
		Slug:      "hello-kratos",
		Title:     "hello kratos",
		ContentMD: "# hi",
		Tags:      []string{"go", "kratos"},
	})
	if err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}
	if created.ID.String() == "" {
		t.Fatal("CreateArticle() did not assign an id")
	}
	if created.Status != biz.ArticleStatusDraft {
		t.Fatalf("status = %d, want draft", created.Status)
	}
	if len(created.Tags) != 2 {
		t.Fatalf("tags = %v, want [go kratos]", created.Tags)
	}

	got, err := repo.FindBySlug(ctx, "hello-kratos")
	if err != nil {
		t.Fatalf("FindBySlug() error = %v", err)
	}
	if got.Title != "hello kratos" || got.ContentMD != "# hi" {
		t.Fatalf("FindBySlug() = %+v, want created article", got)
	}

	updated, err := repo.UpdateArticle(ctx, &biz.Article{
		Slug:      "hello-kratos",
		Title:     "hello again",
		ContentMD: "# hi again",
		Status:    biz.ArticleStatusPublished,
		Tags:      []string{"kratos", "blog"},
	})
	if err != nil {
		t.Fatalf("UpdateArticle() error = %v", err)
	}
	if updated.Title != "hello again" || updated.Status != biz.ArticleStatusPublished {
		t.Fatalf("UpdateArticle() = %+v, want updated title and published", updated)
	}
	if len(updated.Tags) != 2 || updated.Tags[0] != "kratos" {
		t.Fatalf("tags = %v, want [kratos blog]", updated.Tags)
	}

	if _, err := repo.FindBySlug(ctx, "missing-post"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("FindBySlug(missing) error = %v, want not found", err)
	}
}

func TestArticleRepoSlugConflict(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestArticleRepo(t)

	if _, err := repo.CreateArticle(ctx, &biz.Article{Slug: "dup", Title: "a", ContentMD: "a"}); err != nil {
		t.Fatalf("CreateArticle(first) error = %v", err)
	}
	if _, err := repo.CreateArticle(ctx, &biz.Article{Slug: "dup", Title: "b", ContentMD: "b"}); !kratoserrors.IsConflict(err) {
		t.Fatalf("CreateArticle(duplicate) error = %v, want slug conflict", err)
	}
}

func TestArticleRepoDeleteIsSoft(t *testing.T) {
	ctx := context.Background()
	repo, client := newTestArticleRepo(t)

	created, err := repo.CreateArticle(ctx, &biz.Article{Slug: "soft-delete-me", Title: "t", ContentMD: "c"})
	if err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}

	if err := repo.DeleteArticle(ctx, created.Slug); err != nil {
		t.Fatalf("DeleteArticle() error = %v", err)
	}

	// The row survives with status flipped to deleted.
	po, err := client.Article.Query().Where(article.SlugEQ(created.Slug)).Only(ctx)
	if err != nil {
		t.Fatalf("ent query after delete error = %v", err)
	}
	if po.Status != biz.ArticleStatusDeleted {
		t.Fatalf("status after delete = %d, want %d", po.Status, biz.ArticleStatusDeleted)
	}

	// Every read and write path hides it.
	if _, err := repo.FindBySlug(ctx, created.Slug); !kratoserrors.IsNotFound(err) {
		t.Fatalf("FindBySlug() after delete error = %v, want not found", err)
	}
	articles, err := repo.ListArticles(ctx, biz.ListLimit(10))
	if err != nil {
		t.Fatalf("ListArticles() error = %v", err)
	}
	if len(articles) != 0 {
		t.Fatalf("ListArticles() len = %d, want 0", len(articles))
	}
	if _, err := repo.UpdateArticle(ctx, &biz.Article{Slug: created.Slug, Title: "revive", ContentMD: "c", Status: biz.ArticleStatusDraft}); !kratoserrors.IsNotFound(err) {
		t.Fatalf("UpdateArticle() after delete error = %v, want not found", err)
	}
	if err := repo.DeleteArticle(ctx, created.Slug); !kratoserrors.IsNotFound(err) {
		t.Fatalf("DeleteArticle() twice error = %v, want not found", err)
	}
}

func TestArticleRepoListPagination(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestArticleRepo(t)

	for _, slug := range []string{"a-first", "b-second", "c-third"} {
		if _, err := repo.CreateArticle(ctx, &biz.Article{Slug: slug, Title: slug, ContentMD: "c"}); err != nil {
			t.Fatalf("CreateArticle(%q) error = %v", slug, err)
		}
	}

	firstPage, err := repo.ListArticles(ctx, orderByCreatedAt(), biz.ListLimit(2))
	if err != nil {
		t.Fatalf("ListArticles(first page) error = %v", err)
	}
	if len(firstPage) != 2 {
		t.Fatalf("ListArticles(first page) len = %d, want 2", len(firstPage))
	}
	if firstPage[0].Slug != "a-first" {
		t.Fatalf("ListArticles(first page) slug = %q, want a-first", firstPage[0].Slug)
	}

	secondPage, err := repo.ListArticles(ctx, orderByCreatedAt(), biz.ListLimit(2), biz.ListOffset(2))
	if err != nil {
		t.Fatalf("ListArticles(second page) error = %v", err)
	}
	if len(secondPage) != 1 {
		t.Fatalf("ListArticles(second page) len = %d, want 1", len(secondPage))
	}
	if secondPage[0].Slug != "c-third" {
		t.Fatalf("ListArticles(second page) slug = %q, want c-third", secondPage[0].Slug)
	}

	if _, err := repo.ListArticles(ctx, biz.ListLimit(0)); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("ListArticles(zero limit) error = %v, want bad request", err)
	}
}

// Offset pagination is only correct under a total order. With no order_by the
// repo falls back to id, which is UUIDv7 and therefore time-ordered, so paging
// covers every row exactly once.
func TestArticleRepoListDefaultOrderIsStable(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestArticleRepo(t)

	const total = 6
	slugs := make([]string, 0, total)
	for i := range total {
		slug := "post-" + string(rune('a'+i))
		if _, err := repo.CreateArticle(ctx, &biz.Article{Slug: slug, Title: slug, ContentMD: "c"}); err != nil {
			t.Fatalf("CreateArticle(%q) error = %v", slug, err)
		}
		slugs = append(slugs, slug)
	}

	var paged []string
	for offset := 0; offset < total; offset += 2 {
		page, err := repo.ListArticles(ctx, biz.ListLimit(2), biz.ListOffset(offset))
		if err != nil {
			t.Fatalf("ListArticles(offset=%d) error = %v", offset, err)
		}
		for _, a := range page {
			paged = append(paged, a.Slug)
		}
	}

	if len(paged) != total {
		t.Fatalf("paged through %d rows, want %d (duplicated or skipped)", len(paged), total)
	}
	for i, want := range slugs {
		if paged[i] != want {
			t.Fatalf("paged = %v, want creation order %v", paged, slugs)
		}
	}
}

func TestArticleRepoListOrderByDesc(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestArticleRepo(t)

	for _, slug := range []string{"alpha", "beta", "gamma"} {
		if _, err := repo.CreateArticle(ctx, &biz.Article{Slug: slug, Title: slug, ContentMD: "c"}); err != nil {
			t.Fatalf("CreateArticle(%q) error = %v", slug, err)
		}
	}

	articles, err := repo.ListArticles(ctx,
		biz.ListOrderBy(ordering.OrderBy{
			Fields: []ordering.Field{{Path: "slug", Desc: true}},
		}),
		biz.ListLimit(10),
	)
	if err != nil {
		t.Fatalf("ListArticles(order desc) error = %v", err)
	}
	if len(articles) != 3 {
		t.Fatalf("ListArticles(order desc) len = %d, want 3", len(articles))
	}
	if articles[0].Slug != "gamma" || articles[2].Slug != "alpha" {
		t.Fatalf("ListArticles(order desc) = %q..%q, want gamma..alpha", articles[0].Slug, articles[2].Slug)
	}
}
