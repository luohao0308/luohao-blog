package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

// bearerPrefix is the only accepted Authorization scheme.
const bearerPrefix = "Bearer "

// jwtOutcome carries the token-parse outcome between the middlewares: claims
// on success, the typed error otherwise, neither when no token was presented.
type jwtOutcome struct {
	err error
}

type jwtOutcomeKey struct{}

// bearerToken extracts the raw access token from the Authorization header.
func bearerToken(header transport.Header) string {
	authz := header.Get("Authorization")
	if !strings.HasPrefix(authz, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authz, bearerPrefix))
}

// AuthJWT verifies the bearer token when one is presented and injects the
// claims into the context. It does not reject anything by itself: whether an
// operation tolerates an anonymous or invalid token is the policy's call,
// made by the downstream Authorize middleware.
func AuthJWT(issuer biz.TokenIssuer) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				token := bearerToken(tr.RequestHeader())
				switch token {
				case "":
					ctx = context.WithValue(ctx, jwtOutcomeKey{}, &jwtOutcome{})
				default:
					claims, err := issuer.Parse(token)
					if err != nil {
						ctx = context.WithValue(ctx, jwtOutcomeKey{}, &jwtOutcome{err: err})
					} else {
						ctx = biz.NewAuthContext(ctx, claims)
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

// Authorize enforces the authorization policy on every operation. The policy
// is default deny: an operation missing from the policy is unreachable, which
// validatePolicyCoverage turns into a boot failure rather than a runtime 403.
//
// On public operations a presented-but-invalid token degrades to anonymous
// (a stale token must not break public reads); elsewhere the parse error
// decides between 401 (and its expired refinement) and 403 for a valid token
// whose role lacks permission.
func Authorize(authz biz.Authorizer) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, biz.ErrAuthUnauthorized
			}
			operation := tr.Operation()
			act := requestAct(ctx, tr)
			if authz.IsPublic(operation, act) {
				return handler(ctx, req)
			}
			claims, authenticated := biz.AuthFromContext(ctx)
			if !authenticated {
				if outcome, ok := ctx.Value(jwtOutcomeKey{}).(*jwtOutcome); ok && outcome.err != nil {
					return nil, outcome.err
				}
				return nil, biz.ErrAuthUnauthorized
			}
			if !authz.Allowed(biz.RoleSubject(claims.Role), operation, act) {
				return nil, biz.ErrAuthForbidden
			}
			return handler(ctx, req)
		}
	}
}

// requestAct derives the policy act from the transport: the HTTP method,
// or "*" on gRPC where the operation name already encodes the action.
func requestAct(ctx context.Context, tr transport.Transporter) string {
	if tr.Kind() != transport.KindHTTP {
		return "*"
	}
	if req, ok := kratoshttp.RequestFromServerContext(ctx); ok {
		return req.Method
	}
	return "*"
}

// validatePolicyCoverage refuses to boot when a registered HTTP route is
// absent from the policy: default deny would make it unreachable, and a
// 403-on-every-call deployment bug is better caught at startup.
func validatePolicyCoverage(srv *kratoshttp.Server, authz biz.Authorizer) error {
	return srv.WalkRoute(func(r kratoshttp.RouteInfo) error {
		for _, subject := range []string{biz.SubjectPublic, "admin", "reader"} {
			if authz.Allowed(subject, r.Path, r.Method) {
				return nil
			}
		}
		return fmt.Errorf("route %s %s is not covered by the authorization policy; default deny would make it unreachable", r.Method, r.Path)
	})
}
