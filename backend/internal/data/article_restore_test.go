package data

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"testing"
)

func TestRestoreCreatesNewRevision(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestArticleRepo(t)
	uc := biz.NewArticleUsecase(repo, nil)
	a, e := uc.CreateArticle(ctx, &biz.Article{Slug: "restore", Title: "original", ContentMD: "old"})
	if e != nil {
		t.Fatal(e)
	}
	a.Title = "edited"
	a, e = uc.UpdateArticle(ctx, a)
	if e != nil {
		t.Fatal(e)
	}
	a, e = uc.RestoreArticleRevision(ctx, a.Slug, 1, 2)
	if e != nil || a.Version != 3 || a.Title != "original" {
		t.Fatalf("%+v %v", a, e)
	}
}

func TestRestorePreservesPublicationAndCounters(t *testing.T) {
	actor := uuid.New()
	ctx := biz.NewAuthContext(context.Background(), &biz.Claims{UserID: actor, Role: biz.UserRoleAdmin})
	r, _ := newTestArticleRepo(t)
	uc := biz.NewArticleUsecase(r, nil)
	a, e := uc.CreateArticle(ctx, &biz.Article{Slug: "preserve", Title: "original", ContentMD: "old"})
	if e != nil {
		t.Fatal(e)
	}
	a.Title = "edited"
	a.Status = biz.ArticleStatusPublished
	a, e = uc.UpdateArticle(ctx, a)
	if e != nil {
		t.Fatal(e)
	}
	published := a.PublishedAt
	if _, _, e = uc.MarkViewed(ctx, a.Slug, "fixture"); e != nil {
		t.Fatal(e)
	}
	if _, _, e = uc.MarkLiked(ctx, a.Slug, "fixture"); e != nil {
		t.Fatal(e)
	}
	a, e = uc.RestoreArticleRevision(ctx, a.Slug, 1, 2)
	if e != nil || a.Version != 3 || a.Status != biz.ArticleStatusPublished || !a.PublishedAt.Equal(*published) || a.ViewCount != 1 || a.LikeCount != 1 {
		t.Fatalf("publication changed %+v %v", a, e)
	}
	h, e := r.(biz.ArticleRevisionRepo).GetArticleRevision(ctx, a.Slug, 3)
	if e != nil || h.Action != "restore" || h.ActorID == nil || *h.ActorID != actor {
		t.Fatalf("%+v %v", h, e)
	}
	if _, e = uc.RestoreArticleRevision(ctx, a.Slug, 1, 2); !biz.ErrArticleVersionConflict.Is(e) {
		t.Fatal(e)
	}
	if _, e = uc.RestoreArticleRevision(ctx, a.Slug, 99, 3); !biz.ErrArticleNotFound.Is(e) {
		t.Fatal(e)
	}
	if _, e = uc.RestoreArticleRevision(ctx, a.Slug, 1, 0); !biz.ErrArticleVersionRequired.Is(e) {
		t.Fatal(e)
	}
	if _, e = uc.RestoreArticleRevision(ctx, "../bad", 1, 3); !biz.ErrArticleInvalidArgument.Is(e) {
		t.Fatal(e)
	}
	if e = uc.DeleteArticle(ctx, a.Slug, 3); e != nil {
		t.Fatal(e)
	}
	if _, e = uc.RestoreArticleRevision(ctx, a.Slug, 1, 4); !biz.ErrArticleNotFound.Is(e) {
		t.Fatal(e)
	}
}
func TestRestoreRejectsRecreatedCategory(t *testing.T) {
	ctx := context.Background()
	cats, r, _ := newTestRepo(t)
	if _, e := cats.CreateCategory(ctx, &biz.Category{Slug: "cat", Name: "Old"}); e != nil {
		t.Fatal(e)
	}
	a, e := r.CreateArticle(ctx, &biz.Article{Slug: "category-restore", Title: "old", ContentMD: "old", CategorySlug: "cat"})
	if e != nil {
		t.Fatal(e)
	}
	if e = cats.DeleteCategory(ctx, "cat"); e != nil {
		t.Fatal(e)
	}
	if _, e = cats.CreateCategory(ctx, &biz.Category{Slug: "cat", Name: "New entity"}); e != nil {
		t.Fatal(e)
	}
	if _, e = biz.NewArticleUsecase(r, nil).RestoreArticleRevision(ctx, a.Slug, 1, 2); !biz.ErrArticleRevisionCategoryUnavailable.Is(e) {
		t.Fatal(e)
	}
	fresh, e := r.FindBySlug(ctx, a.Slug)
	if e != nil || fresh.Version != 2 || fresh.CategorySlug != "" {
		t.Fatalf("partial restore %+v %v", fresh, e)
	}
}

type restoreMediaStore struct{ missing bool }

func (restoreMediaStore) Save(context.Context, string, []byte) (string, error) {
	return "", errors.New("unused")
}
func (s restoreMediaStore) Load(context.Context, string) ([]byte, string, error) {
	if s.missing {
		return nil, "", errors.New("private filesystem error")
	}
	return []byte("fixture image"), "image/webp", nil
}
func TestRestoreMediaAndRevisionFailure(t *testing.T) {
	ctx := context.Background()
	r, db := newTestArticleRepo(t)
	a, e := r.CreateArticle(ctx, &biz.Article{Slug: "media-restore", Title: "old", ContentMD: "old", CoverURL: "/v1/assets/media/11111111111111111111111111111111.webp", Featured: true})
	if e != nil {
		t.Fatal(e)
	}
	uc := biz.NewArticleUsecase(r, nil)
	if _, e = uc.RestoreArticleRevision(ctx, a.Slug, 1, 1); !biz.ErrArticleRevisionMediaUnavailable.Is(e) {
		t.Fatal(e)
	}
	uc = biz.NewArticleUsecaseWithWechat(r, nil, nil, biz.NewMediaUsecase(restoreMediaStore{missing: true}))
	if _, e = uc.RestoreArticleRevision(ctx, a.Slug, 1, 1); !biz.ErrArticleRevisionMediaUnavailable.Is(e) {
		t.Fatal(e)
	}
	uc = biz.NewArticleUsecaseWithWechat(r, nil, nil, biz.NewMediaUsecase(restoreMediaStore{}))
	a, e = uc.RestoreArticleRevision(ctx, a.Slug, 1, 1)
	if e != nil || a.Version != 2 || !a.Featured || a.CoverURL == "" {
		t.Fatalf("%+v %v", a, e)
	}
	db.ArticleRevision.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(context.Context, ent.Mutation) (ent.Value, error) {
			return nil, errors.New("fixture revision failure")
		})
	})
	if _, e = uc.RestoreArticleRevision(ctx, a.Slug, 1, 2); e == nil {
		t.Fatal("restore committed without audit")
	}
	fresh, e := r.FindBySlug(ctx, a.Slug)
	if e != nil || fresh.Version != 2 {
		t.Fatalf("partial restore %+v %v", fresh, e)
	}
}

type restoreFailIndex struct{ calls int }

func (f *restoreFailIndex) IndexArticle(context.Context, *biz.Article) error {
	f.calls++
	return errors.New("fixture ES unavailable")
}
func (*restoreFailIndex) RemoveArticle(context.Context, string) error {
	return errors.New("fixture ES unavailable")
}
func (*restoreFailIndex) Search(context.Context, string, int, int) ([]string, error) { return nil, nil }
func (*restoreFailIndex) RecreateIndex(context.Context) error                        { return nil }
func TestRestoreCommitsDespiteIndexFailureAndScopesRevision(t *testing.T) {
	ctx := context.Background()
	r, _ := newTestArticleRepo(t)
	a, e := r.CreateArticle(ctx, &biz.Article{Slug: "own", Title: "old", ContentMD: "body"})
	if e != nil {
		t.Fatal(e)
	}
	other, e := r.CreateArticle(ctx, &biz.Article{Slug: "other", Title: "foreign", ContentMD: "foreign"})
	if e != nil {
		t.Fatal(e)
	}
	other.Title = "other v2"
	if _, e = r.UpdateArticle(ctx, other); e != nil {
		t.Fatal(e)
	}
	idx := &restoreFailIndex{}
	uc := biz.NewArticleUsecase(r, idx)
	if _, e = uc.RestoreArticleRevision(ctx, a.Slug, 2, 1); !biz.ErrArticleNotFound.Is(e) {
		t.Fatal("foreign version accepted", e)
	}
	if _, e = uc.RestoreArticleRevision(ctx, a.Slug, 0, 1); !biz.ErrArticleInvalidArgument.Is(e) {
		t.Fatal(e)
	}
	a, e = uc.RestoreArticleRevision(ctx, a.Slug, 1, 1)
	if e != nil || a.Version != 2 || idx.calls != 1 {
		t.Fatalf("index failure rolled back restore %+v %v %d", a, e, idx.calls)
	}
	h, e := r.(biz.ArticleRevisionRepo).GetArticleRevision(ctx, a.Slug, 2)
	if e != nil || h.Action != "restore" {
		t.Fatalf("missing committed history %+v %v", h, e)
	}
}
