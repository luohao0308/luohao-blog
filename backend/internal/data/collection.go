package data

import (
	"context"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/collection"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
)

type collectionRepo struct{ data *Data }

func NewCollectionRepo(data *Data) biz.CollectionRepo { return &collectionRepo{data: data} }

func collectionToBiz(po *ent.Collection, articlePO *ent.Article) *biz.Collection {
	if po == nil || articlePO == nil {
		return nil
	}
	return &biz.Collection{Article: toBiz(articlePO, tagNamesOf(articlePO), articlePO.Edges.Category), SavedAt: po.SavedAt}
}

func (r *collectionRepo) Save(ctx context.Context, userID uuid.UUID, slug string) (*biz.Collection, error) {
	articlePO, err := r.data.db.Article.Query().Where(article.SlugEQ(slug), article.StatusEQ(biz.ArticleStatusPublished)).WithTags().WithCategory().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrArticleNotFound
		}
		return nil, err
	}
	po, err := r.data.db.Collection.Create().SetSavedAt(time.Now().UTC().Truncate(time.Second)).SetUserID(userID).SetArticleID(articlePO.ID).SetArticleSlug(articlePO.Slug).Save(ctx)
	if err == nil {
		return collectionToBiz(po, articlePO), nil
	}
	if !ent.IsConstraintError(err) {
		return nil, err
	}
	existing, findErr := r.data.db.Collection.Query().Where(collection.UserIDEQ(userID), collection.ArticleIDEQ(articlePO.ID)).Only(ctx)
	if findErr != nil {
		return nil, err
	}
	return collectionToBiz(existing, articlePO), nil
}

func (r *collectionRepo) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*biz.Collection, error) {
	pos, err := r.data.db.Collection.Query().
		Where(collection.UserIDEQ(userID), collection.HasArticleWith(article.StatusEQ(biz.ArticleStatusPublished))).
		Order(collection.BySavedAt(sql.OrderDesc()), collection.ByArticleSlug(), collection.ByID()).
		Offset(offset).Limit(limit).
		WithArticle(func(query *ent.ArticleQuery) { query.WithTags().WithCategory() }).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*biz.Collection, 0, len(pos))
	for _, po := range pos {
		out = append(out, collectionToBiz(po, po.Edges.Article))
	}
	return out, nil
}

func (r *collectionRepo) Delete(ctx context.Context, userID uuid.UUID, slug string) error {
	_, err := r.data.db.Collection.Delete().Where(collection.UserIDEQ(userID), collection.HasArticleWith(article.SlugEQ(slug))).Exec(ctx)
	return err
}
