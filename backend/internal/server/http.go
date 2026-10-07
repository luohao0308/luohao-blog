package server

import (
	nethttp "net/http"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/service"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport/http"

	"go.einride.tech/aip/fieldbehavior"
	"google.golang.org/protobuf/proto"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, issuer biz.TokenIssuer, authz biz.Authorizer, article *service.ArticleService, category *service.CategoryService, auth *service.AuthService, user *service.UserService, comment *service.CommentService, subscriber *service.SubscriberService, search *service.ArticleSearchService, chat *service.ChatService) (*http.Server, error) {
	var opts = []http.ServerOption{
		http.Middleware(
			Metrics(),
			recovery.Recovery(),
			AuthJWT(issuer),
			Authorize(authz),
			validate.Validator(func(req any) error {
				if msg, ok := req.(proto.Message); ok {
					if err := fieldbehavior.ValidateRequiredFields(msg); err != nil {
						return err
					}
				}
				return nil
			}),
		),
	}
	opts = append(opts, http.RequestDecoder(limitedBodyDecoder))
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	// Prometheus scrape endpoint: compose-internal only, bypasses API
	// middleware (see metrics.go).
	srv.Handle("/metrics", metricsHandler())
	v1.RegisterArticleServiceHTTPServer(srv, article)
	v1.RegisterCategoryServiceHTTPServer(srv, category)
	v1.RegisterSubscriberServiceHTTPServer(srv, subscriber)
	v1.RegisterAuthServiceHTTPServer(srv, auth)
	v1.RegisterUserServiceHTTPServer(srv, user)
	v1.RegisterCommentServiceHTTPServer(srv, comment)
	v1.RegisterArticleSearchServiceHTTPServer(srv, search)
	v1.RegisterChatServiceHTTPServer(srv, chat)
	if err := validatePolicyCoverage(srv, authz); err != nil {
		return nil, err
	}
	return srv, nil
}

// Request body caps. The Kratos default decoder buffers the entire request
// body in memory before any validation or rate limiting runs, so without a
// cap every endpoint is an unauthenticated memory-amplification vector: a
// single oversized POST is read in full no matter how large. The 1 MiB
// default covers every JSON payload the API defines (comments cap at 1000
// runes, chat questions at 500, article markdown stays far below); the avatar
// upload is the one large payload — base64 of the 2 MiB image cap is ~2.7 MiB
// — so its route gets 4 MiB.
const (
	defaultBodyLimit int64 = 1 << 20
	avatarBodyLimit  int64 = 4 << 20
)

// avatarBodyPath is the single route permitted the larger body cap; it must
// match the proto HTTP binding of UserService.UploadAvatar.
const avatarBodyPath = "/v1/user/avatar"

// limitedBodyDecoder wraps the default Kratos decoder with a hard cap on how
// much of the request body is buffered. A nil ResponseWriter is safe: it only
// skips the stdlib's close-connection hint on overflow, which is the edge
// proxy's job here anyway.
func limitedBodyDecoder(r *nethttp.Request, v any) error {
	limit := defaultBodyLimit
	if r.URL.Path == avatarBodyPath {
		limit = avatarBodyLimit
	}
	r.Body = nethttp.MaxBytesReader(nil, r.Body, limit)
	return http.DefaultRequestDecoder(r, v)
}
