package service

import (
	"context"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

// stubSearchIndex records the limit/offset the service passes down and
// returns a fixed slug list.
type stubSearchIndex struct {
	limit, offset int
	slugs         []string
}

func (s *stubSearchIndex) IndexArticle(context.Context, *biz.Article) error { return nil }
func (s *stubSearchIndex) RemoveArticle(context.Context, string) error      { return nil }
func (s *stubSearchIndex) RecreateIndex(context.Context) error              { return nil }
func (s *stubSearchIndex) Search(_ context.Context, _ string, limit, offset int) ([]string, error) {
	s.limit, s.offset = limit, offset
	return s.slugs, nil
}

// 公开 list/search 接口的 page_size 必须有上限（proto 承诺 server may apply
// a maximum），否则一个请求即可拉穿 MySQL LIMIT 与 ES size。
func TestListPageSizeClamped(t *testing.T) {
	ctx := context.Background()
	t.Run("article list", func(t *testing.T) {
		repo := &stubArticleRepo{}
		svc := NewArticleService(biz.NewArticleUsecase(repo, nil))
		if _, err := svc.ListArticles(ctx, &v1.ListArticlesRequest{PageSize: 100000}); err != nil {
			t.Fatalf("ListArticles error = %v", err)
		}
		var opts biz.ListOptions
		for _, opt := range repo.listOpts {
			opt(&opts)
		}
		if opts.Limit != maxPageSize {
			t.Fatalf("limit = %d, want %d", opts.Limit, maxPageSize)
		}
	})
	t.Run("search", func(t *testing.T) {
		idx := &stubSearchIndex{slugs: []string{"a"}}
		svc := NewArticleSearchService(biz.NewArticleUsecase(&stubArticleRepo{}, idx))
		if _, err := svc.SearchArticles(ctx, &v1.SearchArticlesRequest{Query: "go", PageSize: 100000}); err != nil {
			t.Fatalf("SearchArticles error = %v", err)
		}
		if idx.limit != maxPageSize {
			t.Fatalf("search limit = %d, want %d", idx.limit, maxPageSize)
		}
	})
}

// 非法 page_token 是文档化的 INVALID_ARGUMENT（400），不是 500：article 列表
// 已修，这里是 search 与公开评论列表的同一契约（2026-10 review P2）。
func TestListPageTokenInvalidArgument(t *testing.T) {
	ctx := context.Background()
	t.Run("search", func(t *testing.T) {
		svc := NewArticleSearchService(biz.NewArticleUsecase(&stubArticleRepo{}, &stubSearchIndex{}))
		_, err := svc.SearchArticles(ctx, &v1.SearchArticlesRequest{Query: "go", PageToken: "bogus-token"})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("search bad token error = %v, want bad request", err)
		}
	})
	t.Run("public comments", func(t *testing.T) {
		svc := NewCommentService(biz.NewCommentUsecase(&stubCommentRepo{}, nil, nil))
		_, err := svc.ListArticleComments(ctx, &v1.ListArticleCommentsRequest{Slug: "some-slug", PageToken: "bogus-token"})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("comments bad token error = %v, want bad request", err)
		}
	})
}
