package data

import (
	"context"
	stdsql "database/sql"
	"sort"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"go.einride.tech/aip/filtering"
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
	rdb, _ := newTestRedis(t)
	return NewArticleRepo(&Data{db: client}, rdb), client
}

// orderByCreatedAt keeps list assertions deterministic.
func orderByCreatedAt() biz.ListOption {
	return biz.ListOrderBy(ordering.OrderBy{
		Fields: []ordering.Field{{Path: "created_at"}},
	})
}

// listFilter parses a filter expression against the status declaration the
// service layer exposes, so repo tests exercise the exact CEL form clients
// send instead of hand-built ASTs.
func listFilter(t *testing.T, expr string) biz.ListOption {
	t.Helper()
	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("status", filtering.TypeString),
	)
	if err != nil {
		t.Fatalf("NewDeclarations() error = %v", err)
	}
	filter, err := filtering.ParseFilterString(expr, declarations)
	if err != nil {
		t.Fatalf("ParseFilterString(%q) error = %v", expr, err)
	}
	return biz.ListFilter(filter)
}

// listSlugs runs a list query and returns the result slugs in stable order.
func listSlugs(t *testing.T, repo biz.ArticleRepo, opts ...biz.ListOption) []string {
	t.Helper()
	articles, err := repo.ListArticles(context.Background(), append(opts, biz.ListLimit(10))...)
	if err != nil {
		t.Fatalf("ListArticles() error = %v", err)
	}
	slugs := make([]string, 0, len(articles))
	for _, a := range articles {
		slugs = append(slugs, a.Slug)
	}
	sort.Strings(slugs)
	return slugs
}

// The admin panel filters articles by enum name (status:"PUBLISHED"); the
// column stores the int enum, so the resolver must bridge the two. Before the
// resolver existed this query failed the declaration check in the service
// (500) or, unchecked, would have compared a string against the int column.
func TestArticleRepoListStatusFilter(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestArticleRepo(t)
	for _, slug := range []string{"draft-a", "published-a", "published-b"} {
		created, err := repo.CreateArticle(ctx, &biz.Article{Slug: slug, Title: slug, ContentMD: "c"})
		if err != nil {
			t.Fatalf("CreateArticle(%q) error = %v", slug, err)
		}
		if slug != "draft-a" {
			created.Status = biz.ArticleStatusPublished
			if _, err := repo.UpdateArticle(ctx, created); err != nil {
				t.Fatalf("UpdateArticle(publish %q) error = %v", slug, err)
			}
		}
	}

	t.Run("has operator matches the documented form", func(t *testing.T) {
		got := listSlugs(t, repo, listFilter(t, `status:"PUBLISHED"`))
		want := []string{"published-a", "published-b"}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("status:\"PUBLISHED\" = %v, want %v", got, want)
		}
	})
	t.Run("equals operator", func(t *testing.T) {
		got := listSlugs(t, repo, listFilter(t, `status="DRAFT"`))
		if len(got) != 1 || got[0] != "draft-a" {
			t.Fatalf("status=\"DRAFT\" = %v, want [draft-a]", got)
		}
	})
	t.Run("not equals operator", func(t *testing.T) {
		got := listSlugs(t, repo, listFilter(t, `status!="PUBLISHED"`))
		if len(got) != 1 || got[0] != "draft-a" {
			t.Fatalf("status!=\"PUBLISHED\" = %v, want [draft-a]", got)
		}
	})
	t.Run("combined with public boundary", func(t *testing.T) {
		got := listSlugs(t, repo, biz.ListPublic(), listFilter(t, `status:"DRAFT"`))
		// The public boundary forces published; the agreeing filter must not
		// widen it back.
		if len(got) != 0 {
			t.Fatalf("public + status:\"DRAFT\" = %v, want empty", got)
		}
	})
	t.Run("deleted stays hidden even when requested", func(t *testing.T) {
		got := listSlugs(t, repo, listFilter(t, `status="DELETED"`))
		if len(got) != 0 {
			t.Fatalf("status=\"DELETED\" = %v, want empty", got)
		}
	})
	t.Run("unknown status name is a bad request", func(t *testing.T) {
		if _, err := repo.ListArticles(ctx, listFilter(t, `status:"ARCHIVED"`), biz.ListLimit(10)); !kratoserrors.IsBadRequest(err) {
			t.Fatalf("unknown name error = %v, want bad request", err)
		}
	})
	t.Run("ordering operator is rejected", func(t *testing.T) {
		if _, err := repo.ListArticles(ctx, listFilter(t, `status>"DRAFT"`), biz.ListLimit(10)); !kratoserrors.IsBadRequest(err) {
			t.Fatalf("ordering error = %v, want bad request", err)
		}
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

func TestArticleRepoPublicListFiltersDrafts(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestArticleRepo(t)
	for _, slug := range []string{"draft-post", "published-post"} {
		created, err := repo.CreateArticle(ctx, &biz.Article{Slug: slug, Title: slug, ContentMD: "c"})
		if err != nil {
			t.Fatalf("CreateArticle(%q) error = %v", slug, err)
		}
		if slug == "published-post" {
			created.Status = biz.ArticleStatusPublished
			if _, err := repo.UpdateArticle(ctx, created); err != nil {
				t.Fatalf("UpdateArticle(publish) error = %v", err)
			}
		}
	}

	articles, err := repo.ListArticles(ctx, biz.ListPublic(), biz.ListLimit(10))
	if err != nil {
		t.Fatalf("ListArticles(public) error = %v", err)
	}
	if len(articles) != 1 || articles[0].Slug != "published-post" {
		t.Fatalf("ListArticles(public) = %+v, want only published-post", articles)
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

func TestArticleRepoIncrementViewDedup(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestArticleRepo(t)
	if _, err := repo.CreateArticle(ctx, &biz.Article{Slug: "viewed", Title: "t", ContentMD: "c"}); err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}
	if _, err := repo.UpdateArticle(ctx, &biz.Article{Slug: "viewed", Title: "t", ContentMD: "c", Status: biz.ArticleStatusPublished}); err != nil {
		t.Fatalf("UpdateArticle(publish) error = %v", err)
	}

	count, counted, err := repo.IncrementView(ctx, "viewed", "ip-1")
	if err != nil || !counted || count != 1 {
		t.Fatalf("IncrementView(first) = (%d, %v, %v), want (1, true, nil)", count, counted, err)
	}
	// Same client inside the window: read-only, no double counting.
	count, counted, err = repo.IncrementView(ctx, "viewed", "ip-1")
	if err != nil || counted || count != 1 {
		t.Fatalf("IncrementView(dedup) = (%d, %v, %v), want (1, false, nil)", count, counted, err)
	}
	// A different client counts again.
	count, counted, err = repo.IncrementView(ctx, "viewed", "ip-2")
	if err != nil || !counted || count != 2 {
		t.Fatalf("IncrementView(second client) = (%d, %v, %v), want (2, true, nil)", count, counted, err)
	}
}
