package data

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/enttest"

	"go.einride.tech/aip/filtering"

	// The ent SQLite dialect string is "sqlite3"; modernc.org/sqlite
	// registers itself as "sqlite", so the driver is aliased here for
	// enttest. This is the only SQLite usage in the codebase.
	_ "modernc.org/sqlite"
	ms "modernc.org/sqlite"
)

func init() {
	for _, d := range sql.Drivers() {
		if d == "sqlite3" {
			return
		}
	}
	sql.Register("sqlite3", &ms.Driver{})
}

// newTestRepo opens an in-file SQLite database through enttest (auto
// migration from the ent schema) and returns repos over it.
func newTestRepo(t *testing.T) (biz.CategoryRepo, biz.ArticleRepo) {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"
	client := enttest.Open(t, "sqlite3", dsn)
	t.Cleanup(func() { _ = client.Close() })
	data := &Data{db: client}
	return NewCategoryRepo(data), NewArticleRepo(data, nil)
}

func mustCreateCategory(t *testing.T, repo biz.CategoryRepo, slug, name string, sort int32) *biz.Category {
	t.Helper()
	c, err := repo.CreateCategory(context.Background(), &biz.Category{Slug: slug, Name: name, Sort: sort})
	if err != nil {
		t.Fatalf("create category %s: %v", slug, err)
	}
	return c
}

func TestCategoryCRUDAndConflict(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepo(t)

	_ = mustCreateCategory(t, repo, "engineering", "工程实践", 1)
	if _, err := repo.CreateCategory(ctx, &biz.Category{Slug: "engineering", Name: "dup"}); err != biz.ErrCategorySlugConflict {
		t.Fatalf("duplicate create = %v, want category slug conflict", err)
	}
	got, err := repo.FindBySlug(ctx, "engineering")
	if err != nil || got.Name != "工程实践" {
		t.Fatalf("FindBySlug = %+v, %v", got, err)
	}
	updated, err := repo.UpdateCategory(ctx, &biz.Category{Slug: "engineering", Name: "工程", Sort: 3})
	if err != nil || updated.Name != "工程" || updated.Sort != 3 {
		t.Fatalf("UpdateCategory = %+v, %v", updated, err)
	}
}

func TestListCategoriesCountsPublishedOnly(t *testing.T) {
	ctx := context.Background()
	catRepo, articleRepo := newTestRepo(t)

	mustCreateCategory(t, catRepo, "engineering", "工程实践", 1)
	mustCreateCategory(t, catRepo, "notes", "随笔", 2)
	seedArticle := func(slug string, status biz.ArticleStatus, category string) {
		t.Helper()
		a, err := articleRepo.CreateArticle(ctx, &biz.Article{Slug: slug, Title: slug, ContentMD: "md", Status: status, CategorySlug: category})
		if err != nil {
			t.Fatalf("seed article %s: %v", slug, err)
		}
		if status == biz.ArticleStatusPublished {
			a.Status = biz.ArticleStatusPublished
			if _, err := articleRepo.UpdateArticle(ctx, a); err != nil {
				t.Fatalf("publish article %s: %v", slug, err)
			}
		}
	}
	seedArticle("pub-a", biz.ArticleStatusPublished, "engineering")
	seedArticle("draft-a", biz.ArticleStatusDraft, "engineering")
	seedArticle("pub-b", biz.ArticleStatusPublished, "")

	list, err := catRepo.ListCategories(ctx, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("categories = %d, want 2", len(list))
	}
	// Ordered by sort: engineering first; only published articles count.
	if list[0].Slug != "engineering" || list[0].ArticleCount != 1 {
		t.Fatalf("first = %+v, want engineering with 1 published article", list[0])
	}
	if list[1].Slug != "notes" || list[1].ArticleCount != 0 {
		t.Fatalf("second = %+v, want notes with 0 published articles", list[1])
	}
}

func TestArticleCategoryAttachClearAndFilter(t *testing.T) {
	ctx := context.Background()
	catRepo, articleRepo := newTestRepo(t)

	mustCreateCategory(t, catRepo, "engineering", "工程实践", 1)
	mustCreateCategory(t, catRepo, "notes", "随笔", 2)
	a, err := articleRepo.CreateArticle(ctx, &biz.Article{Slug: "with-cat", Title: "t", ContentMD: "md", CategorySlug: "engineering"})
	if err != nil {
		t.Fatal(err)
	}
	a.Status = biz.ArticleStatusPublished
	if _, err := articleRepo.UpdateArticle(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, err := articleRepo.FindBySlug(ctx, "with-cat")
	if err != nil {
		t.Fatal(err)
	}
	if got.CategorySlug != "engineering" || got.CategoryName != "工程实践" {
		t.Fatalf("category = %q/%q, want engineering/工程实践", got.CategorySlug, got.CategoryName)
	}
	if _, err := articleRepo.CreateArticle(ctx, &biz.Article{Slug: "bad-cat", Title: "t", ContentMD: "md", CategorySlug: "missing"}); err == nil {
		t.Fatal("unknown category slug accepted, want NOT_FOUND")
	}

	// category filter over the public list path.
	listByCategory := func(query string) []*biz.Article {
		t.Helper()
		decls, err := filtering.NewDeclarations(filtering.DeclareStandardFunctions(), filtering.DeclareIdent("category", filtering.TypeString))
		if err != nil {
			t.Fatal(err)
		}
		filter, err := filtering.ParseFilter(&v1.ListArticlesRequest{Filter: query}, decls)
		if err != nil {
			t.Fatal(err)
		}
		articles, err := articleRepo.ListArticles(ctx, biz.ListFilter(filter), biz.ListPublic())
		if err != nil {
			t.Fatal(err)
		}
		return articles
	}
	if got := listByCategory(`category:"engineering"`); len(got) != 1 || got[0].Slug != "with-cat" {
		t.Fatalf("category filter = %+v, want only with-cat", got)
	}
	if got := listByCategory(`category:"notes"`); len(got) != 0 {
		t.Fatalf("empty category filter = %+v, want none", got)
	}
	if got := listByCategory(`category:"missing"`); len(got) != 0 {
		t.Fatalf("unknown slug filter = %+v, want none", got)
	}
	if got := listByCategory(`category!="notes"`); len(got) != 1 || got[0].Slug != "with-cat" {
		t.Fatalf("negated filter = %+v, want only with-cat (nothing uncategorized here)", got)
	}

	// Deleting the category detaches its articles.
	if err := catRepo.DeleteCategory(ctx, "engineering"); err != nil {
		t.Fatal(err)
	}
	got, err = articleRepo.FindBySlug(ctx, "with-cat")
	if err != nil || got.CategorySlug != "" {
		t.Fatalf("after delete category = %q, %v; want detached", got.CategorySlug, err)
	}
	if _, err := catRepo.FindBySlug(ctx, "engineering"); err != biz.ErrCategoryNotFound {
		t.Fatalf("deleted category lookup = %v, want not found", err)
	}
}
