package data

import (
	"context"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/category"

	"github.com/google/uuid"
)

// toBizCategory converts a persisted category (and its published-article
// count when computed) into the domain representation.
func toBizCategory(po *ent.Category, count uint64) *biz.Category {
	if po == nil {
		return nil
	}
	return &biz.Category{
		ID:           po.ID,
		Slug:         po.Slug,
		Name:         po.Name,
		Sort:         po.Sort,
		ArticleCount: count,
		CreatedAt:    po.CreatedAt,
		UpdatedAt:    po.UpdatedAt,
	}
}

type categoryRepo struct {
	data *Data
}

// NewCategoryRepo creates a new CategoryRepo instance.
func NewCategoryRepo(data *Data) biz.CategoryRepo {
	return &categoryRepo{data: data}
}

func (r *categoryRepo) FindBySlug(ctx context.Context, slug string) (*biz.Category, error) {
	po, err := r.data.db.Category.Query().
		Where(category.SlugEQ(slug)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrCategoryNotFound
		}
		return nil, err
	}
	return toBizCategory(po, 0), nil
}

// publishedCount counts the published articles of one category. Categories
// are few and count queries are indexed, so the per-category round trip is
// cheaper to reason about than an aggregate join through the ent client.
func (r *categoryRepo) publishedCount(ctx context.Context, po *ent.Category) (uint64, error) {
	n, err := po.QueryArticles().
		Where(article.StatusEQ(biz.ArticleStatusPublished)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(n), nil
}

func (r *categoryRepo) ListCategories(ctx context.Context, offset, limit int) ([]*biz.Category, error) {
	pos, err := r.data.db.Category.Query().
		Order(category.BySort(), category.BySlug()).
		Offset(offset).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*biz.Category, 0, len(pos))
	for _, po := range pos {
		count, err := r.publishedCount(ctx, po)
		if err != nil {
			return nil, err
		}
		out = append(out, toBizCategory(po, count))
	}
	return out, nil
}

func (r *categoryRepo) CreateCategory(ctx context.Context, c *biz.Category) (*biz.Category, error) {
	po, err := r.data.db.Category.Create().
		SetSlug(c.Slug).
		SetName(c.Name).
		SetSort(c.Sort).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, biz.ErrCategorySlugConflict
		}
		return nil, err
	}
	return toBizCategory(po, 0), nil
}

func (r *categoryRepo) UpdateCategory(ctx context.Context, c *biz.Category) (*biz.Category, error) {
	current, err := r.data.db.Category.Query().
		Where(category.SlugEQ(c.Slug)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrCategoryNotFound
		}
		return nil, err
	}
	po, err := r.data.db.Category.UpdateOneID(current.ID).
		SetName(c.Name).
		SetSort(c.Sort).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return toBizCategory(po, 0), nil
}

// DeleteCategory detaches every referencing article and removes the row in
// one transaction, so a concurrent article write cannot re-attach the
// category between the two statements.
func (r *categoryRepo) DeleteCategory(ctx context.Context, slug string) error {
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Article.Update().
		Where(article.HasCategoryWith(category.SlugEQ(slug))).
		ClearCategory().
		Save(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	affected, err := tx.Category.Delete().
		Where(category.SlugEQ(slug)).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if affected == 0 {
		_ = tx.Rollback()
		return biz.ErrCategoryNotFound
	}
	return tx.Commit()
}

// categoryIDsBySlugs resolves slugs to category ids in one query. Missing
// slugs are simply absent from the result; callers decide the semantics.
func categoryIDsBySlugs(ctx context.Context, client *ent.Client, slugs []string) (map[string]uuid.UUID, error) {
	if len(slugs) == 0 {
		return nil, nil
	}
	pos, err := client.Category.Query().
		Where(category.SlugIn(slugs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]uuid.UUID, len(pos))
	for _, po := range pos {
		out[po.Slug] = po.ID
	}
	return out, nil
}
