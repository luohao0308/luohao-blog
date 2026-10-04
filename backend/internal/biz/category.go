package biz

import (
	"context"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

var (
	// ErrCategoryNotFound is returned when a category does not exist.
	ErrCategoryNotFound = errors.NotFound(v1.ErrorReason_CATEGORY_NOT_FOUND.String(), "category not found")
	// ErrCategoryInvalidArgument is returned when a category request is invalid.
	ErrCategoryInvalidArgument = errors.BadRequest(v1.ErrorReason_CATEGORY_INVALID_ARGUMENT.String(), "invalid category argument")
	// ErrCategorySlugConflict is returned when a slug is already taken.
	ErrCategorySlugConflict = errors.Conflict(v1.ErrorReason_CATEGORY_SLUG_CONFLICT.String(), "category slug conflict")
)

// Category is a curated single-level grouping for articles. An article
// belongs to at most one category (CategorySlug == "" means uncategorized).
type Category struct {
	ID           uuid.UUID
	Slug         string
	Name         string
	Sort         int32
	ArticleCount uint64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CategoryRepo is a category repo.
type CategoryRepo interface {
	FindBySlug(ctx context.Context, slug string) (*Category, error)
	// ListCategories returns a page ordered by sort, then slug; each record
	// carries ArticleCount over published articles only.
	ListCategories(ctx context.Context, offset int, limit int) ([]*Category, error)
	CreateCategory(ctx context.Context, c *Category) (*Category, error)
	UpdateCategory(ctx context.Context, c *Category) (*Category, error)
	// DeleteCategory removes the category and detaches its articles.
	DeleteCategory(ctx context.Context, slug string) error
}

// CategoryUsecase is a Category usecase.
type CategoryUsecase struct {
	repo CategoryRepo
}

// NewCategoryUsecase new a Category usecase.
func NewCategoryUsecase(repo CategoryRepo) *CategoryUsecase {
	return &CategoryUsecase{repo: repo}
}

// Create creates a category. The slug must follow the article slug rules and
// is rejected when taken.
func (uc *CategoryUsecase) Create(ctx context.Context, c *Category) (*Category, error) {
	if err := validateCategory(c); err != nil {
		return nil, err
	}
	return uc.repo.CreateCategory(ctx, c)
}

// Get returns a category by slug.
func (uc *CategoryUsecase) Get(ctx context.Context, slug string) (*Category, error) {
	if !ValidSlug(slug) {
		return nil, ErrCategoryInvalidArgument
	}
	return uc.repo.FindBySlug(ctx, slug)
}

// List returns a page of categories ordered by sort, then slug.
func (uc *CategoryUsecase) List(ctx context.Context, limit, offset int) ([]*Category, error) {
	if limit <= 0 || offset < 0 {
		return nil, ErrCategoryInvalidArgument
	}
	return uc.repo.ListCategories(ctx, offset, limit)
}

// Update updates a category identified by its immutable slug.
func (uc *CategoryUsecase) Update(ctx context.Context, c *Category) (*Category, error) {
	if err := validateCategory(c); err != nil {
		return nil, err
	}
	return uc.repo.UpdateCategory(ctx, c)
}

// Delete removes a category; referencing articles become uncategorized.
func (uc *CategoryUsecase) Delete(ctx context.Context, slug string) error {
	if !ValidSlug(slug) {
		return ErrCategoryInvalidArgument
	}
	return uc.repo.DeleteCategory(ctx, slug)
}

// validateCategory checks the mutable fields of a category at the biz
// boundary. Names are display labels: 1-64 runes covers realistic labels
// while keeping table scans and layout bounded.
func validateCategory(c *Category) error {
	if c == nil {
		return ErrCategoryInvalidArgument
	}
	if !ValidSlug(c.Slug) {
		return ErrCategoryInvalidArgument
	}
	if runes := len([]rune(c.Name)); runes == 0 || runes > 64 {
		return ErrCategoryInvalidArgument
	}
	if c.Sort < 0 {
		return ErrCategoryInvalidArgument
	}
	return nil
}
