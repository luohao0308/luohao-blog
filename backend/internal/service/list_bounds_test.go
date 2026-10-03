package service

import (
	"context"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"go.einride.tech/aip/pagination"
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

// synthToken 产出带合法 checksum、可指定 offset 的 page token：模拟客户端
// 伪造"格式正确但 offset 超窗"的令牌（不需要碰服务端就能构造）。
func synthToken(t *testing.T, req pagination.Request, offset int64) string {
	t.Helper()
	pt, err := pagination.ParsePageToken(req)
	if err != nil {
		t.Fatalf("seed page token: %v", err)
	}
	pt.Offset = offset
	return pt.String()
}

// 语法合法但 offset 超窗的 token 必须以 400 拒绝：ES 的 from+size 硬上限与
// MySQL 的深分页慢扫都不该以 500 或慢查询的形式暴露给匿名调用方。
func TestListPageOffsetBeyondWindow(t *testing.T) {
	ctx := context.Background()
	t.Run("search", func(t *testing.T) {
		svc := NewArticleSearchService(biz.NewArticleUsecase(&stubArticleRepo{}, &stubSearchIndex{}))
		req := &v1.SearchArticlesRequest{Query: "go", PageToken: synthToken(t, &v1.SearchArticlesRequest{Query: "go"}, 20000)}
		if _, err := svc.SearchArticles(ctx, req); !kratoserrors.IsBadRequest(err) {
			t.Fatalf("huge offset error = %v, want bad request", err)
		}
	})
	t.Run("negative offset", func(t *testing.T) {
		svc := NewArticleSearchService(biz.NewArticleUsecase(&stubArticleRepo{}, &stubSearchIndex{}))
		req := &v1.SearchArticlesRequest{Query: "go", PageToken: synthToken(t, &v1.SearchArticlesRequest{Query: "go"}, -5)}
		if _, err := svc.SearchArticles(ctx, req); !kratoserrors.IsBadRequest(err) {
			t.Fatalf("negative offset error = %v, want bad request", err)
		}
	})
	t.Run("article list", func(t *testing.T) {
		svc := NewArticleService(biz.NewArticleUsecase(&stubArticleRepo{}, nil))
		req := &v1.ListArticlesRequest{PageToken: synthToken(t, &v1.ListArticlesRequest{}, 20000)}
		if _, err := svc.ListArticles(ctx, req); !kratoserrors.IsBadRequest(err) {
			t.Fatalf("huge offset error = %v, want bad request", err)
		}
	})
	t.Run("public comments", func(t *testing.T) {
		svc := NewCommentService(biz.NewCommentUsecase(&stubCommentRepo{}, nil, nil))
		req := &v1.ListArticleCommentsRequest{Slug: "some-slug", PageToken: synthToken(t, &v1.ListArticleCommentsRequest{Slug: "some-slug"}, 20000)}
		if _, err := svc.ListArticleComments(ctx, req); !kratoserrors.IsBadRequest(err) {
			t.Fatalf("huge offset error = %v, want bad request", err)
		}
	})
	t.Run("offset at window edge is accepted", func(t *testing.T) {
		svc := NewArticleSearchService(biz.NewArticleUsecase(&stubArticleRepo{}, &stubSearchIndex{}))
		req := &v1.SearchArticlesRequest{Query: "go", PageToken: synthToken(t, &v1.SearchArticlesRequest{Query: "go"}, 9900)}
		if _, err := svc.SearchArticles(ctx, req); err != nil {
			t.Fatalf("edge offset should be accepted, got %v", err)
		}
	})
}
