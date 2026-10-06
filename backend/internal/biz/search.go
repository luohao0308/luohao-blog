package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
)

// Search domain errors.
var (
	// ErrSearchTooManyAttempts guards the Elasticsearch budget: search is
	// public and every query hits the cluster.
	ErrSearchTooManyAttempts = errors.TooManyRequests(v1.ErrorReason_SEARCH_TOO_MANY_ATTEMPTS.String(), "too many search requests, try again later")
)

// SearchQueryMaxRunes bounds one public search query. Real queries are
// phrases; anything longer only wastes ES cycles on tokens nobody reads.
const SearchQueryMaxRunes = 200

// DefaultSearchAttempts/DefaultSearchWindow bound public search queries per
// client IP when the config leaves them unset. Generous by design: a browsing
// reader never comes close, while scripted hammering stays bounded.
const (
	DefaultSearchAttempts int64 = 60
	DefaultSearchWindow         = time.Minute
)

// SearchRateLimiter is the fixed-window limiter for public search queries.
// Distinct from the other limiters so wire can give search its own budget.
type SearchRateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}
