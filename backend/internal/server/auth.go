package server

import (
	"context"
	"strings"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
)

// protectedOperations are the endpoints that require a valid access token.
// The operation id is the mux path template on HTTP (e.g. "/v1/auth/me") and
// the fully-qualified method on gRPC; both forms are listed so the two
// transports behave identically. S3 extends this set with the article write
// endpoints.
var protectedOperations = map[string]bool{
	"/v1/auth/me":                true,
	"/blog.v1.AuthService/GetMe": true,
}

// bearerPrefix is the only accepted Authorization scheme.
const bearerPrefix = "Bearer "

// bearerToken extracts the raw access token from the Authorization header.
func bearerToken(header transport.Header) string {
	authz := header.Get("Authorization")
	if !strings.HasPrefix(authz, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authz, bearerPrefix))
}

// AuthJWT verifies the access token when one is presented and injects the
// claims into the context. Requests without a token proceed anonymously
// unless the operation is protected. A token that fails verification aborts
// protected operations with 401 (expired tokens get their own reason so
// clients can trigger refresh) but never blocks public operations: a stale
// token must not break anonymous reads.
func AuthJWT(issuer biz.TokenIssuer) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				token := bearerToken(tr.RequestHeader())
				protected := protectedOperations[tr.Operation()]
				switch token {
				case "":
					if protected {
						return nil, biz.ErrAuthUnauthorized
					}
				default:
					claims, err := issuer.Parse(token)
					if err != nil {
						if protected {
							return nil, err
						}
						break
					}
					ctx = biz.NewAuthContext(ctx, claims)
				}
			}
			return handler(ctx, req)
		}
	}
}
