package server

import (
	"context"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/google/uuid"
)

// mapHeader is a minimal transport.Header over a plain map.
type mapHeader map[string][]string

func (m mapHeader) Get(key string) string {
	if vs, ok := m[key]; ok && len(vs) > 0 {
		return vs[0]
	}
	return ""
}

func (m mapHeader) Set(key, value string) { m[key] = []string{value} }
func (m mapHeader) Add(key, value string) { m[key] = append(m[key], value) }
func (m mapHeader) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
func (m mapHeader) Values(key string) []string { return m[key] }

// fakeTransporter satisfies the Transporter interface with just the fields
// the middlewares read. Tests run on gRPC semantics (act "*"); HTTP method
// matching is covered against the real policy in internal/data.
type fakeTransporter struct {
	transport.Transporter
	operation string
	header    transport.Header
}

func (f *fakeTransporter) Operation() string               { return f.operation }
func (f *fakeTransporter) RequestHeader() transport.Header { return f.header }
func (f *fakeTransporter) Kind() transport.Kind            { return transport.KindGRPC }

func newTestTransport(operation string, header transport.Header) context.Context {
	return transport.NewServerContext(context.Background(), &fakeTransporter{operation: operation, header: header})
}

// mapIssuer verifies tokens exactly like the real issuer verifies
// signatures: only tokens it issued parse, expired ones get their own error.
type mapIssuer struct {
	tokens map[string]*biz.Claims
}

func newMapIssuer() *mapIssuer { return &mapIssuer{tokens: map[string]*biz.Claims{}} }

func (m *mapIssuer) Issue(_ context.Context, u *biz.User) (*biz.IssuedToken, error) {
	token := "token-for-" + u.ID.String()
	m.tokens[token] = &biz.Claims{UserID: u.ID, Role: u.Role}
	return &biz.IssuedToken{Token: token, ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (m *mapIssuer) Parse(token string) (*biz.Claims, error) {
	if token == "expired" {
		return nil, biz.ErrAuthTokenExpired
	}
	claims, ok := m.tokens[token]
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	return claims, nil
}

// fakeAuthorizer mirrors the shape of the real policy table, including the
// role hierarchy: admin and reader both inherit the authenticated subject.
type fakeAuthorizer struct {
	allowed map[string]bool
}

func (f *fakeAuthorizer) Allowed(subject, operation, act string) bool {
	if f.allowed[subject+"|"+operation+"|"+act] {
		return true
	}
	if subject == "admin" || subject == "reader" {
		return f.allowed[biz.SubjectAuthenticated+"|"+operation+"|"+act]
	}
	return false
}

func (f *fakeAuthorizer) IsPublic(operation, act string) bool {
	return f.Allowed(biz.SubjectPublic, operation, act)
}

// newAuthzChain wires the middlewares in registration order: AuthJWT runs
// first (outer), Authorize consumes its outcome. The terminal handler
// reports whether claims reached it and what they carry.
func newAuthzChain(issuer biz.TokenIssuer, authz biz.Authorizer) middleware.Handler {
	handler := middleware.Handler(func(ctx context.Context, _ any) (any, error) {
		claims, ok := biz.AuthFromContext(ctx)
		if !ok {
			return "anonymous", nil
		}
		return claims.UserID.String(), nil
	})
	return AuthJWT(issuer)(Authorize(authz)(handler))
}

func TestAuthorizePublicOperations(t *testing.T) {
	issuer := newMapIssuer()
	authz := &fakeAuthorizer{allowed: map[string]bool{
		"public|/blog.v1.ArticleService/ListArticles|*": true,
	}}

	// Anonymous read passes.
	got, err := newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.ArticleService/ListArticles", mapHeader{}), nil)
	if err != nil {
		t.Fatalf("public anonymous error = %v", err)
	}
	if got != "anonymous" {
		t.Fatalf("public anonymous reached handler as %v, want anonymous", got)
	}

	// A garbage token on a public operation degrades to anonymous: a stale
	// token must not break public reads.
	got, err = newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.ArticleService/ListArticles", mapHeader{"Authorization": {"Bearer garbage"}}), nil)
	if err != nil {
		t.Fatalf("public with garbage token error = %v", err)
	}
	if got != "anonymous" {
		t.Fatalf("public with garbage token reached handler as %v, want anonymous", got)
	}
}

func TestAuthorizeProtectedRejects(t *testing.T) {
	issuer := newMapIssuer()
	authz := &fakeAuthorizer{allowed: map[string]bool{
		"public|/blog.v1.ArticleService/ListArticles|*": true,
		"authenticated|/blog.v1.AuthService/GetMe|*":    true,
	}}

	// No token on an authenticated operation.
	if _, err := newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.AuthService/GetMe", mapHeader{}), nil); err != biz.ErrAuthUnauthorized {
		t.Fatalf("me without token error = %v, want unauthorized", err)
	}
	// Expired keeps its own reason so clients can trigger refresh.
	if _, err := newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.AuthService/GetMe", mapHeader{"Authorization": {"Bearer expired"}}), nil); err != biz.ErrAuthTokenExpired {
		t.Fatalf("me with expired token error = %v, want token expired", err)
	}
	// Forged token on a protected operation.
	if _, err := newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.AuthService/GetMe", mapHeader{"Authorization": {"Bearer forged"}}), nil); err != biz.ErrAuthUnauthorized {
		t.Fatalf("me with forged token error = %v, want unauthorized", err)
	}
	// No transport context at all.
	if _, err := newAuthzChain(issuer, authz)(context.Background(), nil); err != biz.ErrAuthUnauthorized {
		t.Fatalf("no transport error = %v, want unauthorized", err)
	}
}

func TestAuthorizeRoleGates(t *testing.T) {
	issuer := newMapIssuer()
	authz := &fakeAuthorizer{allowed: map[string]bool{
		"authenticated|/blog.v1.AuthService/GetMe|*":    true,
		"admin|/blog.v1.ArticleService/CreateArticle|*": true,
	}}
	admin := mustIssue(t, issuer, biz.UserRoleAdmin)
	reader := mustIssue(t, issuer, biz.UserRoleReader)

	// Any authenticated role reaches GetMe (role inheritance).
	if _, err := newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.AuthService/GetMe", mapHeader{"Authorization": {"Bearer " + reader}}), nil); err != nil {
		t.Fatalf("me as reader error = %v", err)
	}
	// Admin may write.
	if _, err := newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.ArticleService/CreateArticle", mapHeader{"Authorization": {"Bearer " + admin}}), nil); err != nil {
		t.Fatalf("create as admin error = %v", err)
	}
	// Reader may not write: 403, not 401 — the credentials are fine.
	if _, err := newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.ArticleService/CreateArticle", mapHeader{"Authorization": {"Bearer " + reader}}), nil); err != biz.ErrAuthForbidden {
		t.Fatalf("create as reader error = %v, want forbidden", err)
	}
}

func TestAuthorizeDefaultDeny(t *testing.T) {
	issuer := newMapIssuer()
	authz := &fakeAuthorizer{allowed: map[string]bool{}} // empty policy
	admin := mustIssue(t, issuer, biz.UserRoleAdmin)

	// An operation absent from the policy denies every subject at runtime;
	// for registered routes this is caught at boot by validatePolicyCoverage.
	if _, err := newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.ArticleService/CreateArticle", mapHeader{"Authorization": {"Bearer " + admin}}), nil); err != biz.ErrAuthForbidden {
		t.Fatalf("uncovered operation error = %v, want forbidden", err)
	}
	// A token with a role outside the vocabulary maps to no subject and
	// fails closed too.
	unknown := mustIssue(t, issuer, biz.UserRole(99))
	if _, err := newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.AuthService/GetMe", mapHeader{"Authorization": {"Bearer " + unknown}}), nil); err != biz.ErrAuthForbidden {
		t.Fatalf("unknown role error = %v, want forbidden", err)
	}
}

func mustIssue(t *testing.T, issuer *mapIssuer, role biz.UserRole) string {
	t.Helper()
	token, err := issuer.Issue(context.Background(), &biz.User{ID: uuid.Must(uuid.NewV7()), Role: role})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	return token.Token
}
