package biz

import (
	"context"
	"regexp"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz/article/render"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
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
	// CategorySlug and CategoryName describe the single optional curated
	// category. An empty slug means the article is uncategorized.
	CategorySlug string
	CategoryName string
	Status       ArticleStatus
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ViewCount    uint64
}

// ArticleSearchIndex is the full-text index the write path keeps in sync and
// the search path queries. Implementations must tolerate absence of the
// backend (search degrades to empty results) but the write path treats a
// sync failure as a warning, never as a failure of the article write itself.
type ArticleSearchIndex interface {
	// IndexArticle upserts the document of a published article; non-published
	// articles are removed from the index.
	IndexArticle(context.Context, *Article) error
	// RemoveArticle drops the article document (tolerates a missing one).
	RemoveArticle(context.Context, string) error
	// Search returns slugs of published articles matching the query,
	// best-match first.
	Search(context.Context, string, int, int) ([]string, error)
	// RecreateIndex drops and re-creates the index with the current mapping.
	// The index is derived state rebuilt from MySQL, so this is the documented
	// mapping-evolution path (no in-place migration).
	RecreateIndex(context.Context) error
}

// ArticleRepo is an article repo.
type ArticleRepo interface {
	FindBySlug(context.Context, string) (*Article, error)
	ListArticles(context.Context, ...ListOption) ([]*Article, error)
	CreateArticle(context.Context, *Article) (*Article, error)
	UpdateArticle(context.Context, *Article) (*Article, error)
	DeleteArticle(context.Context, string) error
	// IncrementView adds one public view for slug unless clientKey was
	// already counted inside the repo's dedup window. It returns the
	// article's current view count and whether this call counted.
	IncrementView(context.Context, string, string) (uint64, bool, error)
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
	repo    ArticleRepo
	indexer ArticleSearchIndex
}

// NewArticleUsecase new an Article usecase. The indexer is optional: nil
// disables search indexing (tests, ES-less deployments).
func NewArticleUsecase(repo ArticleRepo, indexer ArticleSearchIndex) *ArticleUsecase {
	return &ArticleUsecase{repo: repo, indexer: indexer}
}

// syncIndex mirrors one article into the search index. Failures are logged
// and swallowed: the article write already succeeded and the index can be
// rebuilt at any time.
func (uc *ArticleUsecase) syncIndex(ctx context.Context, a *Article) {
	if uc.indexer == nil || a == nil {
		return
	}
	if err := uc.indexer.IndexArticle(ctx, a); err != nil {
		log.Warn("article: search index sync failed for "+a.Slug, err)
	}
}

// dropIndex removes an article from the search index with the same
// best-effort semantics as syncIndex.
func (uc *ArticleUsecase) dropIndex(ctx context.Context, slug string) {
	if uc.indexer == nil || slug == "" {
		return
	}
	if err := uc.indexer.RemoveArticle(ctx, slug); err != nil {
		log.Warn("article: search index removal failed for "+slug, err)
	}
}

// SearchArticles full-text searches published articles via the search index,
// hydrating the hits through the repo. Unavailable or empty indexes yield an
// empty page: the public site keeps working without search.
func (uc *ArticleUsecase) SearchArticles(ctx context.Context, query string, limit, offset int) ([]*Article, error) {
	if uc.indexer == nil {
		return []*Article{}, nil
	}
	slugs, err := uc.indexer.Search(ctx, query, limit, offset)
	if err != nil {
		log.Warn("article: search query failed", err)
		return []*Article{}, nil
	}
	out := make([]*Article, 0, len(slugs))
	for _, slug := range slugs {
		a, err := uc.repo.FindBySlug(ctx, slug)
		if err != nil {
			continue
		}
		if a.Status == ArticleStatusPublished {
			out = append(out, a)
		}
	}
	return out, nil
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
	updated, err := uc.repo.UpdateArticle(ctx, a)
	if err != nil {
		return nil, err
	}
	uc.syncIndex(ctx, updated)
	return updated, nil
}

// DeleteArticle soft-deletes an article by slug.
func (uc *ArticleUsecase) DeleteArticle(ctx context.Context, slug string) error {
	if !ValidSlug(slug) {
		return ErrArticleInvalidArgument
	}
	if err := uc.repo.DeleteArticle(ctx, slug); err != nil {
		return err
	}
	uc.dropIndex(ctx, slug)
	return nil
}

// MarkViewed records one public view of a published article. The dedup
// decision lives in the repo; the usecase enforces that only well-formed
// slugs pointing at published articles can move the counter at all.
func (uc *ArticleUsecase) MarkViewed(ctx context.Context, slug, clientKey string) (uint64, bool, error) {
	if !ValidSlug(slug) {
		return 0, false, ErrArticleInvalidArgument
	}
	a, err := uc.repo.FindBySlug(ctx, slug)
	if err != nil {
		return 0, false, err
	}
	if a.Status != ArticleStatusPublished {
		return 0, false, ErrArticleNotFound
	}
	return uc.repo.IncrementView(ctx, slug, clientKey)
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
