package data

import (
	"context"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/tag"

	"github.com/go-kratos/aip-go/ents"
	"github.com/redis/go-redis/v9"
)

// viewDedupPrefix namespaces the per-client view dedup keys; viewDedupWindow
// is how long a (slug, client) pair stays counted after its first view.
const (
	viewDedupPrefix = "blog:view:"
	viewDedupWindow = 24 * time.Hour
)

// toBiz converts a persisted article and its tag rows into the domain
// representation. The status column is bound to biz.ArticleStatus, so it
// carries over as-is.
func toBiz(po *ent.Article, tagNames []string) *biz.Article {
	if po == nil {
		return nil
	}
	return &biz.Article{
		ID:          po.ID,
		Slug:        po.Slug,
		Title:       po.Title,
		Summary:     po.Summary,
		ContentMD:   po.ContentMd,
		ContentHTML: po.ContentHTML,
		Tags:        tagNames,
		Status:      po.Status,
		PublishedAt: po.PublishedAt,
		CreatedAt:   po.CreatedAt,
		UpdatedAt:   po.UpdatedAt,
		ViewCount:   po.ViewCount,
	}
}

type articleRepo struct {
	data *Data
	rdb  redis.UniversalClient
}

// NewArticleRepo creates a new ArticleRepo instance.
func NewArticleRepo(data *Data, rdb redis.UniversalClient) biz.ArticleRepo {
	return &articleRepo{data: data, rdb: rdb}
}

// tagNamesOf collects the names of eagerly loaded tag rows.
func tagNamesOf(po *ent.Article) []string {
	names := make([]string, 0, len(po.Edges.Tags))
	for _, t := range po.Edges.Tags {
		names = append(names, t.Name)
	}
	return names
}

// tagNamesOfRows is the row-slice variant used by write responses: ent write
// results do not carry edges, so the names come from the ensured tag rows.
func tagNamesOfRows(tags []*ent.Tag) []string {
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	return names
}

// ensureTags resolves tag names to rows, creating the ones that do not exist
// yet. Names are stored verbatim; callers validate emptiness upstream.
func (r *articleRepo) ensureTags(ctx context.Context, names []string) ([]*ent.Tag, error) {
	if len(names) == 0 {
		return nil, nil
	}
	tags := make([]*ent.Tag, 0, len(names))
	for _, name := range names {
		po, err := r.data.db.Tag.Query().
			Where(tag.NameEQ(name)).
			Only(ctx)
		if ent.IsNotFound(err) {
			if po, err = r.data.db.Tag.Create().SetName(name).Save(ctx); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
		tags = append(tags, po)
	}
	return tags, nil
}

func (r *articleRepo) FindBySlug(ctx context.Context, slug string) (*biz.Article, error) {
	po, err := r.data.db.Article.Query().
		Where(article.SlugEQ(slug), article.StatusNEQ(biz.ArticleStatusDeleted)).
		WithTags().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrArticleNotFound
		}
		return nil, err
	}
	return toBiz(po, tagNamesOf(po)), nil
}

func (r *articleRepo) ListArticles(ctx context.Context, opts ...biz.ListOption) ([]*biz.Article, error) {
	options := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&options)
	}
	if options.Offset < 0 || options.Limit <= 0 {
		return nil, biz.ErrArticleInvalidArgument
	}
	// Offset pagination needs a total order, so id is always appended as the
	// last sort key; UUIDv7 ids are time-ordered, which keeps unpaged queries
	// stable.
	query := r.data.db.Article.Query().
		Where(article.StatusNEQ(biz.ArticleStatusDeleted)).
		Where(ents.ApplyFilter(options.Filter))
	if options.Public {
		query = query.Where(article.StatusEQ(biz.ArticleStatusPublished))
	}
	pos, err := query.
		Order(ents.ApplyOrderBy(options.OrderBy), article.ByID()).
		Offset(options.Offset).
		Limit(options.Limit).
		WithTags().
		All(ctx)
	if err != nil {
		return nil, err
	}
	articles := make([]*biz.Article, 0, len(pos))
	for _, po := range pos {
		articles = append(articles, toBiz(po, tagNamesOf(po)))
	}
	return articles, nil
}

func (r *articleRepo) CreateArticle(ctx context.Context, a *biz.Article) (*biz.Article, error) {
	tags, err := r.ensureTags(ctx, a.Tags)
	if err != nil {
		return nil, err
	}
	creator := r.data.db.Article.Create().
		SetSlug(a.Slug).
		SetTitle(a.Title).
		SetSummary(a.Summary).
		SetContentMd(a.ContentMD).
		SetContentHTML(a.ContentHTML).
		SetStatus(biz.ArticleStatusDraft)
	if len(tags) > 0 {
		creator = creator.AddTags(tags...)
	}
	po, err := creator.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, biz.ErrArticleSlugConflict
		}
		return nil, err
	}
	return toBiz(po, tagNamesOfRows(tags)), nil
}

func (r *articleRepo) UpdateArticle(ctx context.Context, a *biz.Article) (*biz.Article, error) {
	// status moves through this path (draft <-> published); DELETED only
	// happens through DeleteArticle, and every path hides deleted rows.
	current, err := r.data.db.Article.Query().
		Where(article.SlugEQ(a.Slug), article.StatusNEQ(biz.ArticleStatusDeleted)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrArticleNotFound
		}
		return nil, err
	}
	tags, err := r.ensureTags(ctx, a.Tags)
	if err != nil {
		return nil, err
	}
	updater := r.data.db.Article.UpdateOneID(current.ID).
		SetTitle(a.Title).
		SetSummary(a.Summary).
		SetContentMd(a.ContentMD).
		SetContentHTML(a.ContentHTML).
		SetStatus(a.Status).
		ClearTags()
	if a.PublishedAt != nil {
		updater = updater.SetPublishedAt(*a.PublishedAt)
	} else {
		updater = updater.ClearPublishedAt()
	}
	if len(tags) > 0 {
		updater = updater.AddTags(tags...)
	}
	po, err := updater.Save(ctx)
	if err != nil {
		return nil, err
	}
	return toBiz(po, tagNamesOfRows(tags)), nil
}

// DeleteArticle soft-deletes an article by flipping its status to deleted.
// The row is kept so it stays auditable, and every read path filters it out.
func (r *articleRepo) DeleteArticle(ctx context.Context, slug string) error {
	affected, err := r.data.db.Article.Update().
		Where(article.SlugEQ(slug), article.StatusNEQ(biz.ArticleStatusDeleted)).
		SetStatus(biz.ArticleStatusDeleted).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return biz.ErrArticleNotFound
	}
	return nil
}

// IncrementView adds one view unless clientKey already counted inside the
// dedup window. Redis holds the window; when it is unavailable the call
// degrades to counting every view (fail-open on dedup, the counter itself is
// never lost), because under- or not-counting would corrupt the metric
// permanently while over-counting self-heals as windows expire.
func (r *articleRepo) IncrementView(ctx context.Context, slug, clientKey string) (uint64, bool, error) {
	counted := true
	ok, err := r.rdb.SetNX(ctx, viewDedupPrefix+slug+":"+clientKey, 1, viewDedupWindow).Result()
	if err == nil {
		// A present key means this client already counted inside the window.
		counted = ok
	}
	// On a Redis error keep counted=true: fail-open on dedup so the counter
	// itself is never lost. Over-counting self-heals as windows expire.
	if !counted {
		po, err := r.data.db.Article.Query().
			Where(article.SlugEQ(slug), article.StatusNEQ(biz.ArticleStatusDeleted)).
			Only(ctx)
		if err != nil {
			return 0, false, err
		}
		return po.ViewCount, false, nil
	}
	affected, err := r.data.db.Article.Update().
		Where(article.SlugEQ(slug), article.StatusNEQ(biz.ArticleStatusDeleted)).
		AddViewCount(1).
		Save(ctx)
	if err != nil {
		return 0, false, err
	}
	if affected == 0 {
		return 0, false, biz.ErrArticleNotFound
	}
	po, err := r.data.db.Article.Query().
		Where(article.SlugEQ(slug)).
		Only(ctx)
	if err != nil {
		return 0, false, err
	}
	return po.ViewCount, true, nil
}
