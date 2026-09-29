package biz

import (
	"context"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
)

var (
	// ErrArticleNotFound is returned when an article does not exist.
	ErrArticleNotFound = errors.NotFound(v1.ErrorReason_ARTICLE_NOT_FOUND.String(), "article not found")
	// ErrArticleInvalidArgument is returned when an article request is invalid.
	ErrArticleInvalidArgument = errors.BadRequest(v1.ErrorReason_ARTICLE_INVALID_ARGUMENT.String(), "invalid article argument")
	// ErrArticleSlugConflict is returned when a slug is already taken.
	ErrArticleSlugConflict = errors.Conflict(v1.ErrorReason_ARTICLE_SLUG_CONFLICT.String(), "article slug conflict")
)

// ArticleStatus is the lifecycle state of an article. Values match the api
// enum so the two can be mapped without a lookup table.
type ArticleStatus int32

const (
	// ArticleStatusUnspecified is the zero value; it is never persisted.
	ArticleStatusUnspecified ArticleStatus = 0
	// ArticleStatusDraft marks an article that is being written and hidden
	// from public reads.
	ArticleStatusDraft ArticleStatus = 1
	// ArticleStatusPublished marks a published article.
	ArticleStatusPublished ArticleStatus = 2
	// ArticleStatusDeleted marks a soft-deleted article. The record is
	// retained but hidden from reads.
	ArticleStatusDeleted ArticleStatus = 3
)

// Article is an Article model.
type Article struct {
	ID          uuid.UUID
	Slug        string
	Title       string
	Summary     string
	ContentMD   string
	ContentHTML string
	Tags        []string
	Status      ArticleStatus
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ArticleRepo is an article repo.
type ArticleRepo interface {
	FindBySlug(context.Context, string) (*Article, error)
	ListArticles(context.Context, ...ListOption) ([]*Article, error)
	CreateArticle(context.Context, *Article) (*Article, error)
	UpdateArticle(context.Context, *Article) (*Article, error)
	DeleteArticle(context.Context, string) error
}

// ListOption configures article list queries.
type ListOption func(*ListOptions)

// ListOptions are article list query options.
type ListOptions struct {
	Filter  filtering.Filter
	OrderBy ordering.OrderBy
	Offset  int
	Limit   int
}

// ListFilter sets a standard AIP filter.
func ListFilter(filter filtering.Filter) ListOption {
	return func(o *ListOptions) {
		o.Filter = filter
	}
}

// ListOrderBy sets a standard AIP order_by value.
func ListOrderBy(orderBy ordering.OrderBy) ListOption {
	return func(o *ListOptions) {
		o.OrderBy = orderBy
	}
}

// ListOffset sets an offset.
func ListOffset(offset int) ListOption {
	return func(o *ListOptions) {
		o.Offset = offset
	}
}

// ListLimit sets a limit.
func ListLimit(limit int) ListOption {
	return func(o *ListOptions) {
		o.Limit = limit
	}
}
