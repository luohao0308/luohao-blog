package data

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/tag"
	"testing"
)

func TestArticleHistoryTransaction(t *testing.T) {
	ctx := context.Background()
	repo, db := newTestArticleRepo(t)
	a, e := repo.CreateArticle(ctx, &biz.Article{Slug: "history", Title: "old", ContentMD: "body"})
	if e != nil {
		t.Fatal(e)
	}
	if n, e := db.ArticleRevision.Query().Count(ctx); e != nil || n != 1 {
		t.Fatalf("create snapshot %d %v", n, e)
	}
	a.Title = "new"
	a, e = repo.UpdateArticle(ctx, a)
	if e != nil {
		t.Fatal(e)
	}
	if n, e := db.ArticleRevision.Query().Count(ctx); e != nil || n != 2 {
		t.Fatalf("update snapshots %d %v", n, e)
	}
	h := repo.(biz.ArticleRevisionRepo)
	v, e := h.GetArticleRevision(ctx, a.Slug, 1)
	if e != nil || v.Snapshot.Title != "old" {
		t.Fatalf("%+v %v", v, e)
	}
}

func TestArticleHistoryBaselineAndMetadata(t *testing.T) {
	ctx := context.Background()
	r, db := newTestArticleRepo(t)
	a, e := r.CreateArticle(ctx, &biz.Article{Slug: "baseline", Title: "before", ContentMD: "private body", CoverURL: "/test", Featured: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.ArticleRevision.Delete().Exec(ctx); e != nil {
		t.Fatal(e)
	}
	a.Title = "after"
	if _, e = r.UpdateArticle(ctx, a); e != nil {
		t.Fatal(e)
	}
	h := r.(biz.ArticleRevisionRepo)
	v, e := h.GetArticleRevision(ctx, a.Slug, 1)
	if e != nil || v.Action != "baseline" || v.ActorID != nil || v.Snapshot.Title != "before" || !v.Snapshot.Featured || v.Snapshot.CoverURL != "/test" {
		t.Fatalf("%+v %v", v, e)
	}
	list, e := h.ListArticleRevisions(ctx, a.Slug, 1, 0)
	if e != nil || len(list) != 1 || list[0].Version != 2 || list[0].Snapshot != nil {
		t.Fatalf("list exposes body %+v %v", list, e)
	}
}

func TestArticleRevisionFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	r, db := newTestArticleRepo(t)
	categories := NewCategoryRepo(&Data{db: db})
	if _, e := categories.CreateCategory(ctx, &biz.Category{Slug: "category", Name: "Category"}); e != nil {
		t.Fatal(e)
	}
	a, e := r.CreateArticle(ctx, &biz.Article{Slug: "atomic", Title: "before", ContentMD: "old", CategorySlug: "category"})
	if e != nil {
		t.Fatal(e)
	}
	db.ArticleRevision.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(context.Context, ent.Mutation) (ent.Value, error) {
			return nil, errors.New("fixture revision insert failure")
		})
	})
	a.Title = "after"
	a.Tags = []string{"must-rollback"}
	if _, e = r.UpdateArticle(ctx, a); e == nil {
		t.Fatal("update committed without snapshot")
	}
	if e = r.DeleteArticle(ctx, a.Slug, 1); e == nil {
		t.Fatal("delete committed without snapshot")
	}
	if e = categories.DeleteCategory(ctx, "category"); e == nil {
		t.Fatal("category detach committed without snapshot")
	}
	got, e := r.FindBySlug(ctx, a.Slug)
	if e != nil || got.Title != "before" || got.Version != 1 || got.CategorySlug != "category" {
		t.Fatalf("partial commit %+v %v", got, e)
	}
	if n, e := db.Tag.Query().Where(tag.NameEQ("must-rollback")).Count(ctx); e != nil || n != 0 {
		t.Fatalf("orphan tags %d %v", n, e)
	}
	if _, e = r.CreateArticle(ctx, &biz.Article{Slug: "failed-create", Title: "body", ContentMD: "body"}); e == nil {
		t.Fatal("create committed without snapshot")
	}
	if n, e := db.Article.Query().Where(article.SlugEQ("failed-create")).Count(ctx); e != nil || n != 0 {
		t.Fatal("create not rolled back", n, e)
	}
}
func TestArticleRevisionTrustedActor(t *testing.T) {
	id := uuid.New()
	ctx := biz.NewAuthContext(context.Background(), &biz.Claims{UserID: id, Role: biz.UserRoleAdmin})
	r, db := newTestArticleRepo(t)
	a, e := r.CreateArticle(ctx, &biz.Article{Slug: "actor", Title: "body", ContentMD: "body"})
	if e != nil {
		t.Fatal(e)
	}
	row, e := r.(biz.ArticleRevisionRepo).GetArticleRevision(ctx, a.Slug, 1)
	if e != nil || row.ActorID == nil || *row.ActorID != id {
		t.Fatalf("actor %+v %v", row, e)
	}
	if _, e = db.ArticleRevision.Delete().Exec(ctx); e != nil {
		t.Fatal(e)
	}
	a.Title = "new"
	if _, e = r.UpdateArticle(ctx, a); e != nil {
		t.Fatal(e)
	}
	row, e = r.(biz.ArticleRevisionRepo).GetArticleRevision(ctx, a.Slug, 1)
	if e != nil || row.ActorID != nil || row.Action != "baseline" {
		t.Fatalf("baseline attributed to writer %+v %v", row, e)
	}
}
