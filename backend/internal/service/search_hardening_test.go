package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

// stubSearchRateLimiter is a scripted limiter: it returns the canned allow
// decision and error, recording the keys it was asked about.
type stubSearchRateLimiter struct {
	allow bool
	err   error
	keys  []string
}

func (s *stubSearchRateLimiter) Allow(_ context.Context, key string) (bool, error) {
	s.keys = append(s.keys, key)
	return s.allow, s.err
}

func TestSearchQueryLengthCapped(t *testing.T) {
	svc := NewArticleSearchService(biz.NewArticleUsecase(&stubArticleRepo{}, &stubSearchIndex{}), nil)
	if _, err := svc.SearchArticles(context.Background(), &v1.SearchArticlesRequest{
		Query: strings.Repeat("问", biz.SearchQueryMaxRunes+1),
	}); err == nil {
		t.Fatal("query beyond the rune cap must be rejected")
	}
	if _, err := svc.SearchArticles(context.Background(), &v1.SearchArticlesRequest{
		Query: strings.Repeat("问", biz.SearchQueryMaxRunes),
	}); err != nil {
		t.Fatalf("query at the rune cap must pass: %v", err)
	}
}

func TestSearchRateLimited(t *testing.T) {
	limiter := &stubSearchRateLimiter{allow: false}
	svc := NewArticleSearchService(biz.NewArticleUsecase(&stubArticleRepo{}, &stubSearchIndex{}), limiter)
	_, err := svc.SearchArticles(context.Background(), &v1.SearchArticlesRequest{Query: "go"})
	if err == nil {
		t.Fatal("denied search must fail")
	}
	if kratoserrors.Code(err) != 429 {
		t.Fatalf("denied search code = %d, want 429", kratoserrors.Code(err))
	}
	if len(limiter.keys) != 1 || limiter.keys[0] != "search:unknown" {
		t.Fatalf("limiter keys = %v, want one search:unknown probe", limiter.keys)
	}
}

// The limiter fails open: an infrastructure error in the counter must not
// take search down (same contract as the login/comment/chat throttlers).
func TestSearchLimiterFailsOpen(t *testing.T) {
	limiter := &stubSearchRateLimiter{allow: false, err: errors.New("redis down")}
	svc := NewArticleSearchService(biz.NewArticleUsecase(&stubArticleRepo{}, &stubSearchIndex{slugs: []string{"nope"}}), limiter)
	set, err := svc.SearchArticles(context.Background(), &v1.SearchArticlesRequest{Query: "go"})
	if err != nil {
		t.Fatalf("limiter outage must fail open, got %v", err)
	}
	if set == nil || len(set.Articles) != 0 {
		t.Fatalf("set = %+v, want empty page", set)
	}
}
