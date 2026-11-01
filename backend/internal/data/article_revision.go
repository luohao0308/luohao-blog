package data

import (
	"context"
	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/articlerevision"
)

func revisionSnapshot(a *ent.Article) *biz.ArticleRevisionSnapshot {
	s := &biz.ArticleRevisionSnapshot{Title: a.Title, Summary: a.Summary, ContentMD: a.ContentMd, Tags: tagNamesOf(a), CoverURL: a.CoverURL, Featured: a.Featured, Status: a.Status, PublishedAt: a.PublishedAt}
	if c := a.Edges.Category; c != nil {
		s.CategoryID = c.ID.String()
		s.CategorySlug = c.Slug
		s.CategoryName = c.Name
	}
	return s
}
func loadRevisionArticle(ctx context.Context, c *ent.Client, id uuid.UUID) (*ent.Article, error) {
	return c.Article.Query().Where(article.IDEQ(id)).WithTags().WithCategory().Only(ctx)
}
func insertArticleRevision(ctx context.Context, c *ent.Client, a *ent.Article, action string, baseline bool) error {
	q := c.ArticleRevision.Create().SetArticleID(a.ID).SetVersion(a.Version).SetAction(action).SetSnapshot(revisionSnapshot(a))
	if baseline {
		if !a.UpdatedAt.IsZero() {
			q = q.SetCreatedAt(a.UpdatedAt)
		}
	} else if claims, ok := biz.AuthFromContext(ctx); ok && claims.UserID != uuid.Nil {
		q = q.SetActorID(claims.UserID)
	}
	_, err := q.Save(ctx)
	return err
}
func preserveRevision(ctx context.Context, c *ent.Client, before, after *ent.Article, action string) error {
	if before != nil {
		exists, err := c.ArticleRevision.Query().Where(articlerevision.ArticleIDEQ(before.ID), articlerevision.VersionEQ(before.Version)).Exist(ctx)
		if err != nil {
			return err
		}
		if !exists {
			if err = insertArticleRevision(ctx, c, before, "baseline", true); err != nil {
				return err
			}
		}
	}
	return insertArticleRevision(ctx, c, after, action, false)
}
func revisionDO(row *ent.ArticleRevision, slug string) *biz.ArticleRevision {
	return &biz.ArticleRevision{ArticleSlug: slug, Version: row.Version, Action: row.Action, ActorID: row.ActorID, CreatedAt: row.CreatedAt, Snapshot: row.Snapshot}
}
func (r *articleRepo) ListArticleRevisions(ctx context.Context, slug string, limit, offset int) ([]*biz.ArticleRevision, error) {
	a, e := r.FindBySlug(ctx, slug)
	if e != nil {
		return nil, e
	}
	rows, e := r.data.db.ArticleRevision.Query().Where(articlerevision.ArticleIDEQ(a.ID)).Order(ent.Desc(articlerevision.FieldVersion)).Offset(offset).Limit(limit).Select(articlerevision.FieldArticleID, articlerevision.FieldVersion, articlerevision.FieldAction, articlerevision.FieldActorID, articlerevision.FieldCreatedAt).All(ctx)
	if e != nil {
		return nil, e
	}
	out := make([]*biz.ArticleRevision, 0, len(rows))
	for _, row := range rows {
		out = append(out, revisionDO(row, slug))
	}
	return out, nil
}
func (r *articleRepo) GetArticleRevision(ctx context.Context, slug string, version uint64) (*biz.ArticleRevision, error) {
	a, e := r.FindBySlug(ctx, slug)
	if e != nil {
		return nil, e
	}
	row, e := r.data.db.ArticleRevision.Query().Where(articlerevision.ArticleIDEQ(a.ID), articlerevision.VersionEQ(version)).Only(ctx)
	if ent.IsNotFound(e) {
		return nil, biz.ErrArticleNotFound
	}
	if e != nil {
		return nil, e
	}
	return revisionDO(row, slug), nil
}
