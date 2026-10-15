package service

import (
	"context"
	"testing"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type serviceCollectionRepo struct {
	userID        uuid.UUID
	slug          string
	limit, offset int
}

func (r *serviceCollectionRepo) Save(_ context.Context, userID uuid.UUID, slug string) (*biz.Collection, error) {
	r.userID, r.slug = userID, slug
	return &biz.Collection{Article: &biz.Article{Slug: slug, Status: biz.ArticleStatusPublished}, SavedAt: time.Now()}, nil
}
func (r *serviceCollectionRepo) List(_ context.Context, userID uuid.UUID, limit, offset int) ([]*biz.Collection, error) {
	r.userID, r.limit, r.offset = userID, limit, offset
	return []*biz.Collection{{Article: &biz.Article{Slug: "saved", Status: biz.ArticleStatusPublished}, SavedAt: time.Now()}}, nil
}
func (r *serviceCollectionRepo) Delete(_ context.Context, userID uuid.UUID, slug string) error {
	r.userID, r.slug = userID, slug
	return nil
}

func TestCollectionServiceUsesClaimsIdentity(t *testing.T) {
	repo := &serviceCollectionRepo{}
	svc := NewCollectionService(biz.NewCollectionUsecase(repo))
	readerID := uuid.New()
	ctx := biz.NewAuthContext(context.Background(), &biz.Claims{UserID: readerID, Role: biz.UserRoleReader})
	if _, err := svc.SaveCollection(context.Background(), &v1.SaveCollectionRequest{Slug: "saved"}); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("anonymous save=%v", err)
	}
	out, err := svc.SaveCollection(ctx, &v1.SaveCollectionRequest{Slug: "saved"})
	if err != nil || out.GetArticle().GetSlug() != "saved" || repo.userID != readerID {
		t.Fatalf("save=%+v %v user=%s", out, err, repo.userID)
	}
	if _, err = svc.DeleteCollection(ctx, &v1.DeleteCollectionRequest{Slug: "saved"}); err != nil || repo.userID != readerID {
		t.Fatalf("delete=%v user=%s", err, repo.userID)
	}
}

func TestCollectionServicePaginationAndBounds(t *testing.T) {
	repo := &serviceCollectionRepo{}
	svc := NewCollectionService(biz.NewCollectionUsecase(repo))
	ctx := biz.NewAuthContext(context.Background(), &biz.Claims{UserID: uuid.New(), Role: biz.UserRoleAdmin})
	set, err := svc.ListCollections(ctx, &v1.ListCollectionsRequest{PageSize: 150})
	if err != nil {
		t.Fatal(err)
	}
	if repo.limit != 100 || repo.offset != 0 || len(set.Collections) != 1 {
		t.Fatalf("pagination=%d/%d %+v", repo.limit, repo.offset, set)
	}
	if _, err = svc.ListCollections(context.Background(), &v1.ListCollectionsRequest{}); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("anonymous list=%v", err)
	}
	if _, err = svc.ListCollections(ctx, &v1.ListCollectionsRequest{PageToken: "invalid"}); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("bad token=%v", err)
	}
}
