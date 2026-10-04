package data

import (
	"context"
	"fmt"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/category"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/tag"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/aip-go/ents"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.einride.tech/aip/filtering"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// viewDedupPrefix namespaces the per-client view dedup keys; viewDedupWindow
// is how long a (slug, client) pair stays counted after its first view.
const (
	viewDedupPrefix = "blog:view:"
	likeDedupPrefix = "blog:like:"
	viewDedupWindow = 24 * time.Hour
)

// toBiz converts a persisted article, its tag rows and its optional category
// row into the domain representation. The status column is bound to
// biz.ArticleStatus, so it carries over as-is.
func toBiz(po *ent.Article, tagNames []string, cat *ent.Category) *biz.Article {
	if po == nil {
		return nil
	}
	out := &biz.Article{
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
		LikeCount:   po.LikeCount,
	}
	if cat != nil {
		out.CategorySlug = cat.Slug
		out.CategoryName = cat.Name
	}
	return out
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

// ensureCategory resolves a category slug to its row. An empty slug means
// uncategorized; an unknown slug is a client-facing NOT_FOUND so a bad
// category reference never silently drops.
func (r *articleRepo) ensureCategory(ctx context.Context, slug string) (*ent.Category, error) {
	if slug == "" {
		return nil, nil
	}
	po, err := r.data.db.Category.Query().
		Where(category.SlugEQ(slug)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrCategoryNotFound
		}
		return nil, err
	}
	return po, nil
}

func (r *articleRepo) FindBySlug(ctx context.Context, slug string) (*biz.Article, error) {
	po, err := r.data.db.Article.Query().
		Where(article.SlugEQ(slug), article.StatusNEQ(biz.ArticleStatusDeleted)).
		WithTags().
		WithCategory().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrArticleNotFound
		}
		return nil, err
	}
	return toBiz(po, tagNamesOf(po), po.Edges.Category), nil
}

// articleStatusByName maps the filter-facing enum names onto the stored enum
// values. The column is the int enum (biz.ArticleStatus), while the documented
// filter contract expresses status as a quoted name: status:"PUBLISHED".
var articleStatusByName = map[string]biz.ArticleStatus{
	"DRAFT":     biz.ArticleStatusDraft,
	"PUBLISHED": biz.ArticleStatusPublished,
	"DELETED":   biz.ArticleStatusDeleted,
}

// validateStatusFilter rejects status comparisons the repo cannot translate
// before any query is built. ent flattens builder errors into message-only
// errors, which would turn a client mistake into 500; raised here, the
// INVALID_ARGUMENT survives to the transport.
func validateStatusFilter(filter filtering.Filter) error {
	if filter.CheckedExpr == nil || filter.CheckedExpr.Expr == nil {
		return nil
	}
	var invalid error
	filtering.Walk(func(curr, _ *expr.Expr) bool {
		if invalid != nil {
			return false
		}
		call, ok := curr.GetExprKind().(*expr.Expr_CallExpr)
		if !ok {
			return true
		}
		function := call.CallExpr.GetFunction()
		args := call.CallExpr.GetArgs()
		for i, arg := range args {
			ident, ok := arg.GetExprKind().(*expr.Expr_IdentExpr)
			if !ok || ident.IdentExpr.GetName() != "status" {
				continue
			}
			invalid = statusComparisonError(function, args[len(args)-1-i])
			return false
		}
		return true
	}, filter.CheckedExpr.Expr)
	return invalid
}

// statusComparisonError validates one status comparison against the filter
// contract: the ':' and '=' operators assert an enum name (documented form),
// '!=' negates it, and anything else — including orderings, which have no
// meaning on enum names — is a client error.
func statusComparisonError(function string, value *expr.Expr) error {
	switch function {
	case filtering.FunctionHas, filtering.FunctionEquals, filtering.FunctionNotEquals:
		constExpr, ok := value.GetExprKind().(*expr.Expr_ConstExpr)
		if !ok {
			return fmt.Errorf("%w: status filter expects an enum name literal", biz.ErrArticleInvalidArgument)
		}
		name := constExpr.ConstExpr.GetStringValue()
		if _, ok := articleStatusByName[name]; !ok {
			return fmt.Errorf("%w: unknown status name %q", biz.ErrArticleInvalidArgument, name)
		}
		return nil
	default:
		return fmt.Errorf("%w: operator %q is not supported for status", biz.ErrArticleInvalidArgument, function)
	}
}

// articleFilterResolver translates the validated status comparisons: the
// generic column mapping would compare the enum-name string against the int
// column, so the name resolves to its stored value first. Unknown fields
// report handled=false so the generic translation applies. Reaching an error
// here despite validateStatusFilter would be an internal bug, surfaced as 500.
func articleFilterResolver(sel *sql.Selector, c ents.Comparison) (*sql.Predicate, bool, error) {
	if c.Field != "status" {
		return nil, false, nil
	}
	switch c.Function {
	case filtering.FunctionHas, filtering.FunctionEquals, filtering.FunctionNotEquals:
	default:
		return nil, true, fmt.Errorf("operator %q is not supported for status", c.Function)
	}
	name, ok := c.Value.(string)
	if !ok {
		return nil, true, fmt.Errorf("status filter expects an enum name string, got %T", c.Value)
	}
	status, ok := articleStatusByName[name]
	if !ok {
		return nil, true, fmt.Errorf("unknown status name %q", name)
	}
	column := sel.C(article.FieldStatus)
	if c.Function == filtering.FunctionNotEquals {
		return sql.NEQ(column, status), true, nil
	}
	return sql.EQ(column, status), true, nil
}

// categoryFilterEntry is one category comparison extracted from the filter.
type categoryFilterEntry struct {
	slug    string
	negated bool
}

// extractCategoryFilter collects category comparisons the way
// validateStatusFilter does for status: client errors are raised before any
// query is built, so a bad filter maps to INVALID_ARGUMENT instead of a
// flattened ent 500. Supported forms: `category:"x"` (and `category!= "x"`);
// any other operator or a non-string literal is rejected.
func extractCategoryFilter(filter filtering.Filter) ([]categoryFilterEntry, error) {
	if filter.CheckedExpr == nil || filter.CheckedExpr.Expr == nil {
		return nil, nil
	}
	var (
		out     []categoryFilterEntry
		invalid error
	)
	filtering.Walk(func(curr, _ *expr.Expr) bool {
		if invalid != nil {
			return false
		}
		call, ok := curr.GetExprKind().(*expr.Expr_CallExpr)
		if !ok {
			return true
		}
		function := call.CallExpr.GetFunction()
		args := call.CallExpr.GetArgs()
		for i, arg := range args {
			ident, ok := arg.GetExprKind().(*expr.Expr_IdentExpr)
			if !ok || ident.IdentExpr.GetName() != "category" {
				continue
			}
			value, ok := args[len(args)-1-i].GetExprKind().(*expr.Expr_ConstExpr)
			if !ok {
				invalid = fmt.Errorf("%w: category filter expects a slug string literal", biz.ErrArticleInvalidArgument)
				return false
			}
			entry := categoryFilterEntry{slug: value.ConstExpr.GetStringValue(), negated: function == filtering.FunctionNotEquals}
			if !entry.negated && function != filtering.FunctionHas && function != filtering.FunctionEquals {
				invalid = fmt.Errorf("%w: operator %q is not supported for category", biz.ErrArticleInvalidArgument, function)
				return false
			}
			out = append(out, entry)
			return true
		}
		return true
	}, filter.CheckedExpr.Expr)
	if invalid != nil {
		return nil, invalid
	}
	return out, nil
}

// categoryFilterPredicate translates one category comparison against the
// pre-resolved slug→id map. Missing slugs resolve to the zero UUID, which no
// row carries, so equality matches nothing while its negation matches
// everything — including uncategorized articles.
func categoryFilterPredicate(sel *sql.Selector, c ents.Comparison, ids map[string]uuid.UUID) (*sql.Predicate, bool, error) {
	slug, ok := c.Value.(string)
	if !ok {
		return nil, true, fmt.Errorf("category filter expects a slug string, got %T", c.Value)
	}
	id := ids[slug]
	column := sel.C(article.CategoryColumn)
	switch c.Function {
	case filtering.FunctionHas, filtering.FunctionEquals:
		if id == uuid.Nil {
			return sql.False(), true, nil
		}
		return sql.EQ(column, id), true, nil
	case filtering.FunctionNotEquals:
		if id == uuid.Nil {
			return nil, true, nil
		}
		// Uncategorized articles (NULL category) also match "not this
		// category".
		return sql.Or(sql.NEQ(column, id), sql.IsNull(column)), true, nil
	default:
		return nil, true, fmt.Errorf("operator %q is not supported for category", c.Function)
	}
}

func (r *articleRepo) ListArticles(ctx context.Context, opts ...biz.ListOption) ([]*biz.Article, error) {
	options := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&options)
	}
	if options.Offset < 0 || options.Limit <= 0 {
		return nil, biz.ErrArticleInvalidArgument
	}
	if err := validateStatusFilter(options.Filter); err != nil {
		return nil, err
	}
	catFilters, err := extractCategoryFilter(options.Filter)
	if err != nil {
		return nil, err
	}
	slugs := make([]string, 0, len(catFilters))
	for _, f := range catFilters {
		slugs = append(slugs, f.slug)
	}
	catIDs, err := categoryIDsBySlugs(ctx, r.data.db, slugs)
	if err != nil {
		return nil, err
	}
	// Offset pagination needs a total order, so id is always appended as the
	// last sort key; UUIDv7 ids are time-ordered, which keeps unpaged queries
	// stable.
	query := r.data.db.Article.Query().
		Where(article.StatusNEQ(biz.ArticleStatusDeleted)).
		Where(ents.ApplyFilter(options.Filter, ents.WithFilterResolver(func(sel *sql.Selector, c ents.Comparison) (*sql.Predicate, bool, error) {
			if c.Field == "category" {
				return categoryFilterPredicate(sel, c, catIDs)
			}
			return articleFilterResolver(sel, c)
		})))
	if options.Public {
		query = query.Where(article.StatusEQ(biz.ArticleStatusPublished))
	}
	pos, err := query.
		Order(ents.ApplyOrderBy(options.OrderBy), article.ByID()).
		Offset(options.Offset).
		Limit(options.Limit).
		WithTags().
		WithCategory().
		All(ctx)
	if err != nil {
		return nil, err
	}
	articles := make([]*biz.Article, 0, len(pos))
	for _, po := range pos {
		articles = append(articles, toBiz(po, tagNamesOf(po), po.Edges.Category))
	}
	return articles, nil
}

func (r *articleRepo) CreateArticle(ctx context.Context, a *biz.Article) (*biz.Article, error) {
	tags, err := r.ensureTags(ctx, a.Tags)
	if err != nil {
		return nil, err
	}
	cat, err := r.ensureCategory(ctx, a.CategorySlug)
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
	if cat != nil {
		creator = creator.SetCategoryID(cat.ID)
	}
	po, err := creator.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, biz.ErrArticleSlugConflict
		}
		return nil, err
	}
	return toBiz(po, tagNamesOfRows(tags), cat), nil
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
	cat, err := r.ensureCategory(ctx, a.CategorySlug)
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
	// The service layer merges the field-mask patch before converting, so an
	// empty CategorySlug here genuinely means "uncategorized".
	if cat != nil {
		updater = updater.SetCategoryID(cat.ID)
	} else {
		updater = updater.ClearCategory()
	}
	po, err := updater.Save(ctx)
	if err != nil {
		return nil, err
	}
	return toBiz(po, tagNamesOfRows(tags), cat), nil
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

// counterDedup reports whether this (prefix, slug, client) pair may count
// inside the dedup window. Redis holds the window; when it is unavailable
// the call degrades to counting every request (fail-open on dedup, the
// counter itself is never lost), because under- or not-counting would
// corrupt the metric permanently while over-counting self-heals as windows
// expire.
func (r *articleRepo) counterDedup(ctx context.Context, prefix, slug, clientKey string) bool {
	ok, err := r.rdb.SetNX(ctx, prefix+slug+":"+clientKey, 1, viewDedupWindow).Result()
	if err == nil {
		// A present key means this client already counted inside the window.
		return ok
	}
	// On a Redis error keep counted=true: fail-open on dedup so the counter
	// itself is never lost. Over-counting self-heals as windows expire.
	return true
}

// IncrementView adds one view unless clientKey already counted inside the
// dedup window. Reads skip the status filter so an in-flight delete cannot
// lose an admitted count.
func (r *articleRepo) IncrementView(ctx context.Context, slug, clientKey string) (uint64, bool, error) {
	if !r.counterDedup(ctx, viewDedupPrefix, slug, clientKey) {
		po, err := r.data.db.Article.Query().
			Where(article.SlugEQ(slug)).
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

// IncrementLike mirrors IncrementView for the public like counter.
func (r *articleRepo) IncrementLike(ctx context.Context, slug, clientKey string) (uint64, bool, error) {
	if !r.counterDedup(ctx, likeDedupPrefix, slug, clientKey) {
		po, err := r.data.db.Article.Query().
			Where(article.SlugEQ(slug)).
			Only(ctx)
		if err != nil {
			return 0, false, err
		}
		return po.LikeCount, false, nil
	}
	affected, err := r.data.db.Article.Update().
		Where(article.SlugEQ(slug), article.StatusNEQ(biz.ArticleStatusDeleted)).
		AddLikeCount(1).
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
	return po.LikeCount, true, nil
}
