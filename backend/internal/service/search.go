package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"go.einride.tech/aip/pagination"
)

// ArticleSearchService is the full-text search service.
type ArticleSearchService struct {
	v1.UnimplementedArticleSearchServiceServer

	uc *biz.ArticleUsecase
}

// NewArticleSearchService new an article search service.
func NewArticleSearchService(uc *biz.ArticleUsecase) *ArticleSearchService {
	return &ArticleSearchService{uc: uc}
}

// SearchArticles full-text searches published articles. Search backend
// absence or failure degrades to an empty page (the usecase handles that);
// only the query validation surfaces as an error.
func (s *ArticleSearchService) SearchArticles(ctx context.Context, req *v1.SearchArticlesRequest) (*v1.ArticleSet, error) {
	if req.GetQuery() == "" {
		return nil, biz.ErrArticleInvalidArgument
	}
	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, err
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	articles, err := s.uc.SearchArticles(ctx, req.GetQuery(), int(req.PageSize), int(pageToken.Offset))
	if err != nil {
		return nil, err
	}
	set := &v1.ArticleSet{
		Articles: make([]*v1.Article, 0, len(articles)),
	}
	if len(articles) >= int(req.PageSize) {
		set.NextPageToken = pageToken.Next(req).String()
	}
	for _, a := range articles {
		set.Articles = append(set.Articles, convertArticleReply(a))
	}
	return set, nil
}
