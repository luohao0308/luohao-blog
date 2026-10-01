package service

import (
	"context"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

// The update flow merges the patch into the current record and converts it
// for the usecase; a dropped status would surface as UNSPECIFIED and get
// rejected by the biz layer (the bug that made every update a 400).
func TestConvertArticleCarriesStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status v1.ArticleStatus
		want   biz.ArticleStatus
	}{
		{"draft", v1.ArticleStatus_ARTICLE_STATUS_DRAFT, biz.ArticleStatusDraft},
		{"published", v1.ArticleStatus_ARTICLE_STATUS_PUBLISHED, biz.ArticleStatusPublished},
		{"unspecified stays zero", v1.ArticleStatus_ARTICLE_STATUS_UNSPECIFIED, biz.ArticleStatusUnspecified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := convertArticle(&v1.Article{Slug: "s", Title: "t", ContentMd: "md", Status: tc.status})
			if got.Status != tc.want {
				t.Fatalf("status = %d, want %d", got.Status, tc.want)
			}
		})
	}
}

// stubArticleRepo only records list options; the other repo methods are out
// of scope for the service-level list tests and fail loudly if reached.
type stubArticleRepo struct {
	listOpts []biz.ListOption
}

func (s *stubArticleRepo) FindBySlug(context.Context, string) (*biz.Article, error) {
	return nil, biz.ErrArticleNotFound
}
func (s *stubArticleRepo) ListArticles(_ context.Context, opts ...biz.ListOption) ([]*biz.Article, error) {
	s.listOpts = opts
	return nil, nil
}
func (s *stubArticleRepo) CreateArticle(context.Context, *biz.Article) (*biz.Article, error) {
	return nil, biz.ErrArticleInvalidArgument
}
func (s *stubArticleRepo) UpdateArticle(context.Context, *biz.Article) (*biz.Article, error) {
	return nil, biz.ErrArticleInvalidArgument
}
func (s *stubArticleRepo) DeleteArticle(context.Context, string) error {
	return biz.ErrArticleInvalidArgument
}
func (s *stubArticleRepo) IncrementView(context.Context, string, string) (uint64, bool, error) {
	return 0, false, biz.ErrArticleInvalidArgument
}

// The documented status filter (status:"PUBLISHED", the form the admin panel
// sends) used to fail the declaration check and surface as a 500: the AIP
// parse errors are plain errors that Kratos maps to Unknown. The contract
// promises INVALID_ARGUMENT for malformed list arguments instead.
func TestListArticlesFilterHandling(t *testing.T) {
	ctx := context.Background()
	repo := &stubArticleRepo{}
	svc := NewArticleService(biz.NewArticleUsecase(repo, nil))

	t.Run("documented status filter reaches the repo", func(t *testing.T) {
		repo.listOpts = nil
		if _, err := svc.ListArticles(ctx, &v1.ListArticlesRequest{Filter: `status:"PUBLISHED"`}); err != nil {
			t.Fatalf("ListArticles(status filter) error = %v, want success", err)
		}
		var opts biz.ListOptions
		for _, opt := range repo.listOpts {
			opt(&opts)
		}
		if opts.Filter.CheckedExpr == nil {
			t.Fatal("parsed filter did not reach the repo")
		}
	})
	t.Run("malformed filter is invalid argument", func(t *testing.T) {
		_, err := svc.ListArticles(ctx, &v1.ListArticlesRequest{Filter: `status:`})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("malformed filter error = %v, want bad request", err)
		}
	})
	t.Run("undeclared filter field is invalid argument", func(t *testing.T) {
		_, err := svc.ListArticles(ctx, &v1.ListArticlesRequest{Filter: `bogus="x"`})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("undeclared field error = %v, want bad request", err)
		}
	})
	t.Run("unsupported order_by field is invalid argument", func(t *testing.T) {
		_, err := svc.ListArticles(ctx, &v1.ListArticlesRequest{OrderBy: "content_md"})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("unsupported order_by error = %v, want bad request", err)
		}
	})
}
