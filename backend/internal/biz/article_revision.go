package biz

import (
	"context"
	"github.com/google/uuid"
	"time"
)

// ArticleRevisionSnapshot is immutable author state, independent of live taxonomies.
type ArticleRevisionSnapshot struct {
	Title        string        `json:"title"`
	Summary      string        `json:"summary"`
	ContentMD    string        `json:"content_md"`
	Tags         []string      `json:"tags"`
	CategoryID   string        `json:"category_id"`
	CategorySlug string        `json:"category_slug"`
	CategoryName string        `json:"category_name"`
	CoverURL     string        `json:"cover_url"`
	Featured     bool          `json:"featured"`
	Status       ArticleStatus `json:"status"`
	PublishedAt  *time.Time    `json:"published_at,omitempty"`
}
type ArticleRevision struct {
	ArticleSlug string
	Version     uint64
	Action      string
	ActorID     *uuid.UUID
	CreatedAt   time.Time
	Snapshot    *ArticleRevisionSnapshot
}
type ArticleRevisionRepo interface {
	ListArticleRevisions(context.Context, string, int, int) ([]*ArticleRevision, error)
	GetArticleRevision(context.Context, string, uint64) (*ArticleRevision, error)
}

func (uc *ArticleUsecase) ListArticleRevisions(ctx context.Context, slug string, limit, offset int) ([]*ArticleRevision, error) {
	if !ValidSlug(slug) || limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		return nil, ErrArticleInvalidArgument
	}
	if _, e := uc.GetArticle(ctx, slug); e != nil {
		return nil, e
	}
	r, ok := uc.repo.(ArticleRevisionRepo)
	if !ok {
		return nil, ErrArticleInvalidArgument
	}
	return r.ListArticleRevisions(ctx, slug, limit, offset)
}
func (uc *ArticleUsecase) GetArticleRevision(ctx context.Context, slug string, version uint64) (*ArticleRevision, error) {
	if !ValidSlug(slug) || version == 0 {
		return nil, ErrArticleInvalidArgument
	}
	if _, e := uc.GetArticle(ctx, slug); e != nil {
		return nil, e
	}
	r, ok := uc.repo.(ArticleRevisionRepo)
	if !ok {
		return nil, ErrArticleInvalidArgument
	}
	return r.GetArticleRevision(ctx, slug, version)
}
