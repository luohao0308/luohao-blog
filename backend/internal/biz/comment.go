package biz

import (
	"context"
	"time"

	"go.einride.tech/aip/filtering"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/uuid"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
)

// Comment status values. They share numeric values with the api enum, but
// the mapping is written out in the service layer so neither side can drift.
const (
	// CommentStatusUnspecified is the zero value; it is never persisted.
	CommentStatusUnspecified CommentStatus = 0
	// CommentStatusPending marks a comment awaiting moderation; it is not
	// publicly visible.
	CommentStatusPending CommentStatus = 1
	// CommentStatusApproved marks a comment an admin has approved; it is
	// publicly visible.
	CommentStatusApproved CommentStatus = 2
)

// CommentStatus is the moderation state of a comment.
type CommentStatus int32

// Comment domain errors.
var (
	ErrCommentNotFound         = errors.NotFound(v1.ErrorReason_COMMENT_NOT_FOUND.String(), "comment not found")
	ErrCommentInvalidArgument  = errors.BadRequest(v1.ErrorReason_COMMENT_INVALID_ARGUMENT.String(), "invalid comment argument")
	ErrCommentTooManyAttempts  = errors.TooManyRequests(v1.ErrorReason_COMMENT_TOO_MANY_ATTEMPTS.String(), "too many comments, try again later")
	ErrCommentArticleNotPublic = ErrArticleNotFound
	CommentDisplayNameMaxRunes = 32
	CommentContentMaxRunes     = 1000
	// DefaultCommentAttempts/DefaultCommentWindow bound anonymous submissions
	// per client IP when the config leaves them unset.
	DefaultCommentAttempts int64 = 20
	DefaultCommentWindow         = 5 * time.Minute
)

// Comment is a visitor comment on an article.
type Comment struct {
	ID          uuid.UUID
	ArticleSlug string
	DisplayName string
	Content     string
	Status      CommentStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CommentFilter narrows admin comment listings. The CEL filter compiles
// into SQL in the data layer, exactly like the article list filter.
type CommentFilter struct {
	Filter filtering.Filter
}

// CommentListOptions are comment list query options.
type CommentListOptions struct {
	Filter CommentFilter
	Limit  int
	Offset int
}

// CommentListOption configures comment list queries.
type CommentListOption func(*CommentListOptions)

// CommentFilter passes a parsed filter to the listing.
func CommentListFilter(f CommentFilter) CommentListOption {
	return func(o *CommentListOptions) {
		o.Filter = f
	}
}

// CommentLimit caps the page size.
func CommentLimit(n int) CommentListOption {
	return func(o *CommentListOptions) {
		o.Limit = n
	}
}

// CommentOffset skips rows for pagination.
func CommentOffset(n int) CommentListOption {
	return func(o *CommentListOptions) {
		o.Offset = n
	}
}

// CommentRepo is a comment repo.
type CommentRepo interface {
	Create(context.Context, *Comment) (*Comment, error)
	FindByID(context.Context, uuid.UUID) (*Comment, error)
	// ListApprovedBySlug returns approved comments of an article, newest
	// first, for public reads.
	ListApprovedBySlug(context.Context, string, int, int) ([]*Comment, error)
	// List returns comments in any state for the admin panel.
	List(context.Context, ...CommentListOption) ([]*Comment, error)
	// Approve moves a comment to approved, idempotently for approved rows.
	Approve(context.Context, uuid.UUID) (*Comment, error)
	Delete(context.Context, uuid.UUID) error
}

// CommentRateLimiter is the fixed-window limiter for anonymous comment
// submissions. Deliberately a distinct type from RateLimiter so wire can
// wire login and comment throttling with separate budgets.
type CommentRateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// CommentUsecase is a Comment usecase.
type CommentUsecase struct {
	repo     CommentRepo
	articles *ArticleUsecase
	limiter  CommentRateLimiter
}

// NewCommentUsecase new a Comment usecase.
func NewCommentUsecase(repo CommentRepo, articles *ArticleUsecase, limiter CommentRateLimiter) *CommentUsecase {
	return &CommentUsecase{repo: repo, articles: articles, limiter: limiter}
}

// Submit validates and stores a visitor comment as pending. The target
// article must be publicly readable, and submissions are throttled per
// client IP; the limiter fails open like the login throttler.
func (uc *CommentUsecase) Submit(ctx context.Context, c *Comment, clientIP string) (*Comment, error) {
	if uc.limiter != nil {
		ok, err := uc.limiter.Allow(ctx, "comment:"+clientIP)
		if err != nil {
			log.Warn("comment: rate limiter unavailable, failing open", "err", err)
		} else if !ok {
			return nil, ErrCommentTooManyAttempts
		}
	}
	if err := validateComment(c); err != nil {
		return nil, err
	}
	if _, err := uc.articles.GetPublicArticle(ctx, c.ArticleSlug); err != nil {
		return nil, ErrCommentArticleNotPublic
	}
	c.ID = uuid.Nil // repo assigns a fresh UUIDv7
	c.Status = CommentStatusPending
	return uc.repo.Create(ctx, c)
}

// ListPublic returns approved comments of an article, newest first. Reads of
// a non-public article are indistinguishable from an empty list: they carry
// no information about the article's existence.
func (uc *CommentUsecase) ListPublic(ctx context.Context, slug string, limit, offset int) ([]*Comment, error) {
	if !ValidSlug(slug) {
		return nil, ErrCommentInvalidArgument
	}
	return uc.repo.ListApprovedBySlug(ctx, slug, limit, offset)
}

// ListAdmin returns comments in any moderation state.
func (uc *CommentUsecase) ListAdmin(ctx context.Context, opts ...CommentListOption) ([]*Comment, error) {
	return uc.repo.List(ctx, opts...)
}

// Approve moves a comment to approved. Approving an approved comment is
// idempotent.
func (uc *CommentUsecase) Approve(ctx context.Context, id uuid.UUID) (*Comment, error) {
	return uc.repo.Approve(ctx, id)
}

// Delete removes a comment permanently.
func (uc *CommentUsecase) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id)
}

// validateComment checks the mutable fields of a comment at the biz boundary.
func validateComment(c *Comment) error {
	if c == nil {
		return ErrCommentInvalidArgument
	}
	if !ValidSlug(c.ArticleSlug) {
		return ErrCommentInvalidArgument
	}
	name := len([]rune(c.DisplayName))
	if name == 0 || name > CommentDisplayNameMaxRunes {
		return ErrCommentInvalidArgument
	}
	content := len([]rune(c.Content))
	if content == 0 || content > CommentContentMaxRunes {
		return ErrCommentInvalidArgument
	}
	return nil
}
