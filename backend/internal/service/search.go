package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"errors"

	"go.einride.tech/aip/pagination"
)

// errSearchQueryTooLong is a plain error mapped to INVALID_ARGUMENT by
// invalidListArgument, same treatment as the other list-bound sentinels.
var errSearchQueryTooLong = errors.New("search query beyond the length limit")

// ArticleSearchService is the full-text search service.
type ArticleSearchService struct {
	v1.UnimplementedArticleSearchServiceServer

	uc      *biz.ArticleUsecase
	limiter biz.SearchRateLimiter
}

// NewArticleSearchService new an article search service.
func NewArticleSearchService(uc *biz.ArticleUsecase, limiter biz.SearchRateLimiter) *ArticleSearchService {
	return &ArticleSearchService{uc: uc, limiter: limiter}
}

// SearchArticles full-text searches published articles. The public boundary
// validates the query shape (non-empty, bounded length) and throttles per
// client IP — this is the only search entry that spends the visitor's budget:
// the chat usecase retrieves through the same usecase method but answers under
// its own throttle. Search backend absence or failure degrades to an empty
// page (the usecase handles that); validation and throttling surface as
// errors. The limiter fails open like the other throttlers: an outage in the
// counter must not take search down.
func (s *ArticleSearchService) SearchArticles(ctx context.Context, req *v1.SearchArticlesRequest) (*v1.ArticleSet, error) {
	if req.GetQuery() == "" {
		return nil, biz.ErrArticleInvalidArgument
	}
	if runes := []rune(req.GetQuery()); len(runes) > biz.SearchQueryMaxRunes {
		return nil, invalidListArgument(errSearchQueryTooLong)
	}
	if s.limiter != nil {
		ok, err := s.limiter.Allow(ctx, "search:"+clientIP(ctx))
		if err != nil {
			// limiter failure fails open, mirroring login/comment/chat.
		} else if !ok {
			return nil, biz.ErrSearchTooManyAttempts
		}
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
