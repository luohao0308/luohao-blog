package data

import (
	"context"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/articlerevision"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/category"
)

func (r *articleRepo) RestoreArticleRevision(ctx context.Context, slug string, version, expected uint64, prepare biz.ArticleRevisionPreparer) (*biz.Article, error) {
	if expected == 0 {
		return nil, biz.ErrArticleVersionRequired
	}
	if !biz.ValidSlug(slug) || version == 0 || prepare == nil {
		return nil, biz.ErrArticleInvalidArgument
	}
	tx, e := r.data.db.Tx(ctx)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	current, e := tx.Article.Query().Where(article.SlugEQ(slug), article.StatusNEQ(biz.ArticleStatusDeleted)).WithTags().WithCategory().Only(ctx)
	if ent.IsNotFound(e) {
		return nil, biz.ErrArticleNotFound
	}
	if e != nil {
		return nil, e
	}
	if current.Version != expected {
		return nil, biz.ErrArticleVersionConflict
	}
	revision, e := tx.ArticleRevision.Query().Where(articlerevision.ArticleIDEQ(current.ID), articlerevision.VersionEQ(version)).Only(ctx)
	if ent.IsNotFound(e) {
		return nil, biz.ErrArticleNotFound
	}
	if e != nil {
		return nil, e
	}
	s := revision.Snapshot
	if s == nil {
		return nil, biz.ErrArticleNotFound
	}
	if s.CategoryID != "" {
		id, e := uuid.Parse(s.CategoryID)
		if e != nil {
			return nil, biz.ErrArticleRevisionCategoryUnavailable
		}
		// Locking current read ignores any older RR read view. Keep the entity
		// stable through restore commit, serializing with category deletion.
		cat, e := tx.Category.Query().Where(category.IDEQ(id), category.SlugEQ(s.CategorySlug), func(selector *sql.Selector) {
			if selector.Dialect() == dialect.MySQL {
				selector.ForShare()
			}
		}).Only(ctx)
		if ent.IsNotFound(e) {
			return nil, biz.ErrArticleRevisionCategoryUnavailable
		}
		if e != nil {
			return nil, e
		}
		if cat.Slug != s.CategorySlug {
			return nil, biz.ErrArticleRevisionCategoryUnavailable
		}
	} else if s.CategorySlug != "" {
		return nil, biz.ErrArticleRevisionCategoryUnavailable
	}
	a := &biz.Article{ID: current.ID, Slug: current.Slug, Version: expected, Title: s.Title, Summary: s.Summary, ContentMD: s.ContentMD, Tags: append([]string(nil), s.Tags...), CategorySlug: s.CategorySlug, CoverURL: s.CoverURL, Featured: s.Featured, Status: current.Status, PublishedAt: current.PublishedAt}
	if e = prepare(ctx, a); e != nil {
		return nil, e
	}
	out, e := updateArticleWithAction(ctx, tx.Client(), a, "restore")
	if e != nil {
		return nil, e
	}
	// Validate the final identity as well as the historical precondition.
	// A same-slug category recreated during preparation must not be rebound.
	after, e := loadRevisionArticle(ctx, tx.Client(), current.ID)
	if e != nil {
		return nil, e
	}
	finalCategoryID := ""
	if after.Edges.Category != nil {
		finalCategoryID = after.Edges.Category.ID.String()
	}
	if finalCategoryID != s.CategoryID {
		return nil, biz.ErrArticleRevisionCategoryUnavailable
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return out, nil
}
