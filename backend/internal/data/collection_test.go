package data

import (
	"context"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/collection"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

func TestCollectionRepoIsolationIdempotencyAndPublicPagination(t *testing.T) {
	ctx := context.Background()
	_, articleRepo, data := newTestRepo(t)
	repo := NewCollectionRepo(data)
	users := biz.NewUserUsecase(NewUserRepo(data))
	u1, _ := users.CreateAccount(ctx, "c1@example.com", "longenough1", "C1", biz.UserRoleReader)
	u2, _ := users.CreateAccount(ctx, "c2@example.com", "longenough1", "C2", biz.UserRoleReader)
	publish := func(slug string) *biz.Article {
		a, err := articleRepo.CreateArticle(ctx, &biz.Article{Slug: slug, Title: slug, ContentMD: "body"})
		if err != nil {
			t.Fatal(err)
		}
		a.Status = biz.ArticleStatusPublished
		a.PublishedAt = &[]time.Time{time.Now()}[0]
		a, err = articleRepo.UpdateArticle(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	for _, slug := range []string{"collection-a", "collection-b", "collection-aa-hidden"} {
		publish(slug)
	}
	first, err := repo.Save(ctx, u1.ID, "collection-a")
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := repo.Save(ctx, u1.ID, "collection-a")
	if err != nil || !duplicate.SavedAt.Equal(first.SavedAt) {
		t.Fatalf("duplicate=%+v %v", duplicate, err)
	}
	if _, err = repo.Save(ctx, u2.ID, "collection-a"); err != nil {
		t.Fatal(err)
	}
	tie := time.Now().UTC().Truncate(time.Second)
	if _, err = data.db.Collection.Delete().Where(collection.UserIDEQ(u1.ID)).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"collection-a", "collection-aa-hidden", "collection-b"} {
		articlePO, queryErr := data.db.Article.Query().Where(article.SlugEQ(slug)).Only(ctx)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		if _, createErr := data.db.Collection.Create().SetSavedAt(tie).SetUserID(u1.ID).SetArticleID(articlePO.ID).SetArticleSlug(slug).Save(ctx); createErr != nil {
			t.Fatal(createErr)
		}
	}
	hidden, _ := articleRepo.FindBySlug(ctx, "collection-aa-hidden")
	hidden.Status = biz.ArticleStatusDraft
	hidden.PublishedAt = nil
	if _, err = articleRepo.UpdateArticle(ctx, hidden); err != nil {
		t.Fatal(err)
	}
	page1, err := repo.List(ctx, u1.ID, 1, 0)
	if err != nil || len(page1) != 1 || page1[0].Article.Slug != "collection-a" {
		t.Fatalf("page1=%+v %v", page1, err)
	}
	page2, err := repo.List(ctx, u1.ID, 1, 1)
	if err != nil || len(page2) != 1 || page2[0].Article.Slug != "collection-b" {
		t.Fatalf("page2=%+v %v", page2, err)
	}
	page3, err := repo.List(ctx, u1.ID, 1, 2)
	if err != nil || len(page3) != 0 {
		t.Fatalf("page3=%+v %v", page3, err)
	}
	other, err := repo.List(ctx, u2.ID, 10, 0)
	if err != nil || len(other) != 1 {
		t.Fatalf("other=%+v %v", other, err)
	}
	if _, err = repo.Save(ctx, u1.ID, "collection-aa-hidden"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("draft save=%v", err)
	}
	if err = repo.Delete(ctx, u2.ID, "collection-b"); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.List(ctx, u1.ID, 10, 0); len(got) != 2 {
		t.Fatalf("cross delete affected owner=%+v", got)
	}
	if err = repo.Delete(ctx, u1.ID, "collection-aa-hidden"); err != nil {
		t.Fatal(err)
	}
	if err = repo.Delete(ctx, u1.ID, "collection-aa-hidden"); err != nil {
		t.Fatal(err)
	}
	if _, err = data.db.Article.Update().Where(article.SlugEQ("collection-b")).SetStatus(biz.ArticleStatusDeleted).Save(ctx); err != nil {
		t.Fatal(err)
	}
	if err = repo.Delete(ctx, u1.ID, "collection-b"); err != nil {
		t.Fatal(err)
	}
}
