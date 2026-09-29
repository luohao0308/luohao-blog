package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ArticleService is an article service.
type ArticleService struct {
	v1.UnimplementedArticleServiceServer
}

// NewArticleService new an article service.
func NewArticleService() *ArticleService {
	return &ArticleService{}
}

// CreateArticle creates an article. The storage layer lands in S2 of the M1
// plan; until then every method reports Unimplemented.
func (s *ArticleService) CreateArticle(ctx context.Context, req *v1.CreateArticleRequest) (*v1.Article, error) {
	return nil, status.Error(codes.Unimplemented, "article storage lands in M1/S2")
}

// ListArticles lists articles.
func (s *ArticleService) ListArticles(ctx context.Context, req *v1.ListArticlesRequest) (*v1.ArticleSet, error) {
	return nil, status.Error(codes.Unimplemented, "article storage lands in M1/S2")
}

// GetArticle returns an article by slug.
func (s *ArticleService) GetArticle(ctx context.Context, req *v1.GetArticleRequest) (*v1.Article, error) {
	return nil, status.Error(codes.Unimplemented, "article storage lands in M1/S2")
}

// UpdateArticle updates an article.
func (s *ArticleService) UpdateArticle(ctx context.Context, req *v1.UpdateArticleRequest) (*v1.Article, error) {
	return nil, status.Error(codes.Unimplemented, "article storage lands in M1/S2")
}

// DeleteArticle soft-deletes an article.
func (s *ArticleService) DeleteArticle(ctx context.Context, req *v1.DeleteArticleRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "article storage lands in M1/S2")
}
