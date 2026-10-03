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
		// Same treatment as the article list: AIP parse errors are plain
		// errors that Kratos would map to 500; the contract promises 400.
		return nil, invalidListArgument(err)
	}
	if !pageOffsetWithinWindow(pageToken.Offset) {
		// ES rejects from+size beyond its max_result_window with an error
		// that would surface as a 500 — reject the offset up front.
		return nil, invalidListArgument(errPageOffsetOutOfRange)
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	clampPageSize(&req.PageSize)
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
