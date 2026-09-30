package biz

import (
	"context"
	"regexp"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz/article/render"

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
	Public  bool
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

// ListPublic restricts the query to published articles for anonymous reads.
func ListPublic() ListOption {
	return func(o *ListOptions) {
		o.Public = true
	}
}

// slugPattern constrains the public identifier: lowercase letters, digits and
// single hyphens between segments, 1-64 bytes total. It is URL-safe and stable
// under escaping.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidSlug reports whether s is an acceptable article slug.
func ValidSlug(s string) bool {
	return len(s) >= 1 && len(s) <= 64 && slugPattern.MatchString(s)
}

// ArticleUsecase is an Article usecase.
type ArticleUsecase struct {
	repo ArticleRepo
}

// NewArticleUsecase new an Article usecase.
func NewArticleUsecase(repo ArticleRepo) *ArticleUsecase {
	return &ArticleUsecase{repo: repo}
}

// CreateArticle creates an article. New articles always start as DRAFT:
// the caller-requested status and published_at are ignored. content_html is
// rendered from content_md here so readers never parse Markdown.
func (uc *ArticleUsecase) CreateArticle(ctx context.Context, a *Article) (*Article, error) {
	if err := validateArticle(a); err != nil {
		return nil, err
	}
	html, err := render.HTML(a.ContentMD)
	if err != nil {
		return nil, ErrArticleInvalidArgument
	}
	a.ContentHTML = html
	a.Status = ArticleStatusDraft
	a.PublishedAt = nil
	return uc.repo.CreateArticle(ctx, a)
}

// GetArticle returns a non-deleted article by slug.
func (uc *ArticleUsecase) GetArticle(ctx context.Context, slug string) (*Article, error) {
	if !ValidSlug(slug) {
		return nil, ErrArticleInvalidArgument
	}
	return uc.repo.FindBySlug(ctx, slug)
}

// GetPublicArticle returns only published content. Public transport handlers
// must use this boundary so drafts never reach an anonymous response, even if
// the repository also serves authenticated management reads.
func (uc *ArticleUsecase) GetPublicArticle(ctx context.Context, slug string) (*Article, error) {
	a, err := uc.GetArticle(ctx, slug)
	if err != nil {
		return nil, err
	}
	if a.Status != ArticleStatusPublished {
		return nil, ErrArticleNotFound
	}
	return a, nil
}

// ListArticles lists articles.
func (uc *ArticleUsecase) ListArticles(ctx context.Context, opts ...ListOption) ([]*Article, error) {
	return uc.repo.ListArticles(ctx, opts...)
}

// UpdateArticle updates an article identified by its immutable slug. Moving a
// non-published article to PUBLISHED stamps published_at; the transition to
// DELETED only happens through DeleteArticle.
func (uc *ArticleUsecase) UpdateArticle(ctx context.Context, a *Article) (*Article, error) {
	if err := validateArticle(a); err != nil {
		return nil, err
	}
	if a.Status == ArticleStatusUnspecified || a.Status == ArticleStatusDeleted {
		return nil, ErrArticleInvalidArgument
	}
	current, err := uc.repo.FindBySlug(ctx, a.Slug)
	if err != nil {
		return nil, err
	}
	if a.Status == ArticleStatusPublished {
		if a.PublishedAt == nil {
			if current.Status == ArticleStatusPublished && current.PublishedAt != nil {
				// The article is already published: keep the original stamp.
				a.PublishedAt = current.PublishedAt
			} else {
				// First publish.
				now := time.Now()
				a.PublishedAt = &now
			}
		}
	} else {
		// A non-published state never carries a publication timestamp.
		a.PublishedAt = nil
	}
	// content_html is a write-time cache: re-render on every content change.
	html, err := render.HTML(a.ContentMD)
	if err != nil {
		return nil, ErrArticleInvalidArgument
	}
	a.ContentHTML = html
	return uc.repo.UpdateArticle(ctx, a)
}

// DeleteArticle soft-deletes an article by slug.
func (uc *ArticleUsecase) DeleteArticle(ctx context.Context, slug string) error {
	if !ValidSlug(slug) {
		return ErrArticleInvalidArgument
	}
	return uc.repo.DeleteArticle(ctx, slug)
}

// validateArticle checks the mutable fields of an article at the biz boundary.
func validateArticle(a *Article) error {
	if a == nil {
		return ErrArticleInvalidArgument
	}
	if !ValidSlug(a.Slug) {
		return ErrArticleInvalidArgument
	}
	if len([]rune(a.Title)) == 0 || len([]rune(a.Title)) > 128 {
		return ErrArticleInvalidArgument
	}
	if len([]rune(a.ContentMD)) == 0 {
		return ErrArticleInvalidArgument
	}
	return nil
}
