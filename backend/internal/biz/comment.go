package biz

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

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
	// CommentStatusApproved marks a comment approved automatically or by an admin; it is
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
	// DefaultCommentAttempts/DefaultCommentWindow bound submissions per
	// client IP when the config leaves them unset.
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
	// UserID is the authoring account; nil only for legacy rows from the
	// anonymous era. New comments always carry one.
	UserID *uuid.UUID
	// AvatarURL is resolved at read time from the author account and is
	// never persisted; empty for legacy rows and accounts without an upload.
	AvatarURL string
	CreatedAt time.Time
	UpdatedAt time.Time
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

// CommentRateLimiter is the fixed-window limiter for comment submissions.
// Deliberately a distinct type from RateLimiter so wire can wire login and
// comment throttling with separate budgets.
type CommentRateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// CommentUsecase is a Comment usecase.
type CommentUsecase struct {
	repo       CommentRepo
	articles   *ArticleUsecase
	users      *UserUsecase
	limiter    CommentRateLimiter
	moderation CommentModerationPolicy
}

// NewCommentUsecase new a Comment usecase.
func NewCommentUsecase(repo CommentRepo, articles *ArticleUsecase, users *UserUsecase, limiter CommentRateLimiter, moderation CommentModerationPolicy) *CommentUsecase {
	return &CommentUsecase{repo: repo, articles: articles, users: users, limiter: limiter, moderation: moderation}
}

// Submit assigns identity from the account and moderates before persisting.
// IP and account budgets are independent. A limiter outage saves the comment
// for review instead of publishing without spam protection.
func (uc *CommentUsecase) Submit(ctx context.Context, c *Comment, claims *Claims, clientIP string) (*Comment, error) {
	if claims == nil {
		return nil, ErrAuthUnauthorized
	}
	if c == nil {
		return nil, ErrCommentInvalidArgument
	}
	author, err := uc.users.ByID(ctx, claims.UserID)
	if err != nil {
		return nil, ErrAuthUnauthorized
	}
	c.ID = uuid.Nil
	c.UserID = &author.ID
	c.DisplayName = author.DisplayName
	c.Content = strings.TrimSpace(c.Content)
	if err := validateComment(c); err != nil {
		return nil, err
	}
	if _, err := uc.articles.GetPublicArticle(ctx, c.ArticleSlug); err != nil {
		if errors.IsNotFound(err) {
			return nil, ErrCommentArticleNotPublic
		}
		return nil, err
	}
	c.Status = uc.moderation.status(c.DisplayName, c.Content)
	if uc.limiter != nil {
		keys := []string{"comment:user:" + author.ID.String()}
		if clientIP != "" {
			keys = append(keys, "comment:"+clientIP)
		}
		for _, key := range keys {
			ok, err := uc.limiter.Allow(ctx, key)
			if err != nil {
				log.Warn("comment: rate limiter unavailable, holding for review", "err", err)
				c.Status = CommentStatusPending
			} else if !ok {
				return nil, ErrCommentTooManyAttempts
			}
		}
	}
	c, err = uc.repo.Create(ctx, c)
	if err != nil {
		return nil, err
	}
	c.AvatarURL = author.AvatarURL
	return c, nil
}

// ListPublic returns approved comments of an article, newest first, with the
// authors' avatars attached for display. Reads of a non-public article are
// indistinguishable from an empty list: they carry no information about the
// article's existence.
func (uc *CommentUsecase) ListPublic(ctx context.Context, slug string, limit, offset int) ([]*Comment, error) {
	if !ValidSlug(slug) {
		return nil, ErrCommentInvalidArgument
	}
	if _, err := uc.articles.GetPublicArticle(ctx, slug); err != nil {
		if errors.IsNotFound(err) {
			return []*Comment{}, nil
		}
		return nil, err
	}
	comments, err := uc.repo.ListApprovedBySlug(ctx, slug, limit, offset)
	if err != nil {
		return nil, err
	}
	uc.attachAvatars(ctx, comments)
	return comments, nil
}

// ListAdmin returns comments in any moderation state, avatars attached for
// the moderation view.
func (uc *CommentUsecase) ListAdmin(ctx context.Context, opts ...CommentListOption) ([]*Comment, error) {
	comments, err := uc.repo.List(ctx, opts...)
	if err != nil {
		return nil, err
	}
	uc.attachAvatars(ctx, comments)
	return comments, nil
}

// attachAvatars fills the transient AvatarURL of account-authored comments
// in one batched lookup. Legacy anonymous comments and unknown accounts keep
// the empty avatar; the display name column remains the source of identity
// text for them.
func (uc *CommentUsecase) attachAvatars(ctx context.Context, comments []*Comment) {
	if uc.users == nil || len(comments) == 0 {
		return
	}
	seen := make(map[uuid.UUID]struct{}, len(comments))
	ids := make([]uuid.UUID, 0, len(comments))
	for _, c := range comments {
		if c.UserID == nil {
			continue
		}
		if _, ok := seen[*c.UserID]; ok {
			continue
		}
		seen[*c.UserID] = struct{}{}
		ids = append(ids, *c.UserID)
	}
	if len(ids) == 0 {
		return
	}
	users, err := uc.users.ByIDs(ctx, ids)
	if err != nil {
		// Avatar decoration must never break the comment listing; degrade to
		// the initials fallback on the frontend.
		log.Warn("comment: avatar lookup failed, serving without avatars", "err", err)
		return
	}
	avatars := make(map[uuid.UUID]string, len(users))
	for _, u := range users {
		avatars[u.ID] = u.AvatarURL
	}
	for _, c := range comments {
		if c.UserID != nil {
			c.AvatarURL = avatars[*c.UserID]
		}
	}
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
	if content == 0 || content > CommentContentMaxRunes || !utf8.ValidString(c.Content) {
		return ErrCommentInvalidArgument
	}
	return nil
}
