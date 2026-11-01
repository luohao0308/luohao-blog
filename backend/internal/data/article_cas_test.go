package data

import (
	"context"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"testing"
)

func TestArticleCAS(t *testing.T) {
	ctx := context.Background()
	r, _ := newTestArticleRepo(t)
	a, e := r.CreateArticle(ctx, &biz.Article{Slug: "cas", Title: "old", ContentMD: "body", Version: 1})
	if e != nil {
		t.Fatal(e)
	}
	old := *a
	a.Title = "new"
	next, e := r.UpdateArticle(ctx, a)
	if e != nil || next.Version != 2 {
		t.Fatalf("%+v %v", next, e)
	}
	old.Title = "stale"
	if _, e = r.UpdateArticle(ctx, &old); !biz.ErrArticleVersionConflict.Is(e) {
		t.Fatal(e)
	}
	if e = r.DeleteArticle(ctx, a.Slug, old.Version); !biz.ErrArticleVersionConflict.Is(e) {
		t.Fatal(e)
	}
}

func TestArticleCountersDoNotChangeVersion(t *testing.T) {
	ctx := context.Background()
	r, _ := newTestArticleRepo(t)
	a, e := r.CreateArticle(ctx, &biz.Article{Slug: "counter-cas", Title: "title", ContentMD: "body"})
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = r.IncrementView(ctx, a.Slug, "fixture"); e != nil {
		t.Fatal(e)
	}
	if _, _, e = r.IncrementLike(ctx, a.Slug, "fixture"); e != nil {
		t.Fatal(e)
	}
	got, e := r.FindBySlug(ctx, a.Slug)
	if e != nil || got.Version != a.Version {
		t.Fatalf("counter changed version: %+v %v", got, e)
	}
}

func TestDeletedCategoryInvalidatesArticleVersion(t *testing.T) {
	ctx := context.Background()
	categories, articles, _ := newTestRepo(t)
	if _, e := categories.CreateCategory(ctx, &biz.Category{Slug: "same-category", Name: "Old"}); e != nil {
		t.Fatal(e)
	}
	a, e := articles.CreateArticle(ctx, &biz.Article{Slug: "category-cas", Title: "old", ContentMD: "body", CategorySlug: "same-category"})
	if e != nil {
		t.Fatal(e)
	}
	if e = categories.DeleteCategory(ctx, "same-category"); e != nil {
		t.Fatal(e)
	}
	fresh, e := articles.FindBySlug(ctx, a.Slug)
	if e != nil || fresh.Version != 2 || fresh.CategorySlug != "" {
		t.Fatalf("%+v %v", fresh, e)
	}
	if _, e = categories.CreateCategory(ctx, &biz.Category{Slug: "same-category", Name: "New"}); e != nil {
		t.Fatal(e)
	}
	if _, e = articles.UpdateArticle(ctx, a); !biz.ErrArticleVersionConflict.Is(e) {
		t.Fatalf("stale category snapshot accepted: %v", e)
	}
}
