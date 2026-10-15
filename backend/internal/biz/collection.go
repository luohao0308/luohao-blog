package biz

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Collection struct {
	Article *Article
	SavedAt time.Time
}

type CollectionRepo interface {
	Save(context.Context, uuid.UUID, string) (*Collection, error)
	List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Collection, error)
	Delete(context.Context, uuid.UUID, string) error
}

type CollectionUsecase struct{ repo CollectionRepo }

func NewCollectionUsecase(repo CollectionRepo) *CollectionUsecase {
	return &CollectionUsecase{repo: repo}
}

func (uc *CollectionUsecase) Save(ctx context.Context, userID uuid.UUID, slug string) (*Collection, error) {
	if userID == uuid.Nil {
		return nil, ErrAuthUnauthorized
	}
	if !ValidSlug(slug) {
		return nil, ErrArticleInvalidArgument
	}
	return uc.repo.Save(ctx, userID, slug)
}
func (uc *CollectionUsecase) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Collection, error) {
	if userID == uuid.Nil {
		return nil, ErrAuthUnauthorized
	}
	if limit <= 0 || offset < 0 {
		return nil, ErrArticleInvalidArgument
	}
	return uc.repo.List(ctx, userID, limit, offset)
}
func (uc *CollectionUsecase) Delete(ctx context.Context, userID uuid.UUID, slug string) error {
	if userID == uuid.Nil {
		return ErrAuthUnauthorized
	}
	if !ValidSlug(slug) {
		return ErrArticleInvalidArgument
	}
	return uc.repo.Delete(ctx, userID, slug)
}
