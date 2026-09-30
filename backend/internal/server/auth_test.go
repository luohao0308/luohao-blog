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

// fakeTransporter satisfies the Transporter interface with just the two
// fields the middleware reads; the embedded nil interface covers the rest.
type fakeTransporter struct {
	transport.Transporter
	operation string
	header    transport.Header
}

func (f *fakeTransporter) Operation() string               { return f.operation }
func (f *fakeTransporter) RequestHeader() transport.Header { return f.header }

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

// runAuthJWT runs the middleware with a terminal handler that reports the
// claims it saw.
func runAuthJWT(t *testing.T, issuer biz.TokenIssuer, ctx context.Context) (*biz.Claims, error) {
	t.Helper()
	var seen *biz.Claims
	handler := middleware.Handler(func(ctx context.Context, _ any) (any, error) {
		seen, _ = biz.AuthFromContext(ctx)
		return nil, nil
	})
	wrapped := AuthJWT(issuer)(handler)
	_, err := wrapped(ctx, struct{}{})
	return seen, err
}

func TestAuthJWTMissingTokenOnProtected(t *testing.T) {
	if _, err := runAuthJWT(t, newMapIssuer(), newTestTransport("/v1/auth/me", mapHeader{})); err != biz.ErrAuthUnauthorized {
		t.Fatalf("protected without token error = %v, want unauthorized", err)
	}
}

func TestAuthJWTValidTokenOnProtected(t *testing.T) {
	issuer := newMapIssuer()
	uid := uuid.Must(uuid.NewV7())
	token, err := issuer.Issue(context.Background(), &biz.User{ID: uid, Role: biz.UserRoleAdmin})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	header := mapHeader{"Authorization": {"Bearer " + token.Token}}

	claims, err := runAuthJWT(t, issuer, newTestTransport("/blog.v1.AuthService/GetMe", header))
	if err != nil {
		t.Fatalf("protected with token error = %v", err)
	}
	if claims == nil || claims.UserID != uid {
		t.Fatalf("claims = %+v, want the token's account", claims)
	}
}

func TestAuthJWTExpiredAndForgedOnProtected(t *testing.T) {
	issuer := newMapIssuer()

	if _, err := runAuthJWT(t, issuer, newTestTransport("/v1/auth/me", mapHeader{"Authorization": {"Bearer expired"}})); err != biz.ErrAuthTokenExpired {
		t.Fatalf("expired token error = %v, want token expired", err)
	}
	if _, err := runAuthJWT(t, issuer, newTestTransport("/v1/auth/me", mapHeader{"Authorization": {"Bearer forged"}})); err != biz.ErrAuthUnauthorized {
		t.Fatalf("forged token error = %v, want unauthorized", err)
	}
	if _, err := runAuthJWT(t, issuer, newTestTransport("/v1/auth/me", mapHeader{"Authorization": {"Basic dXNlcjpwYXNz"}})); err != biz.ErrAuthUnauthorized {
		t.Fatalf("non-bearer scheme error = %v, want unauthorized", err)
	}
}

func TestAuthJWTPublicOperations(t *testing.T) {
	issuer := newMapIssuer()
	uid := uuid.Must(uuid.NewV7())
	token, err := issuer.Issue(context.Background(), &biz.User{ID: uid, Role: biz.UserRoleAdmin})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	// A garbage token on a public operation must not break the read.
	claims, err := runAuthJWT(t, issuer, newTestTransport("/v1/articles/list", mapHeader{"Authorization": {"Bearer garbage"}}))
	if err != nil {
		t.Fatalf("public with garbage token error = %v, want anonymous pass-through", err)
	}
	if claims != nil {
		t.Fatalf("claims = %+v, want none", claims)
	}

	// A valid token on a public operation still upgrades the request.
	claims, err = runAuthJWT(t, issuer, newTestTransport("/v1/articles/list", mapHeader{"Authorization": {"Bearer " + token.Token}}))
	if err != nil {
		t.Fatalf("public with valid token error = %v", err)
	}
	if claims == nil || claims.UserID != uid {
		t.Fatalf("claims = %+v, want the token's account", claims)
	}

	// No header at all: anonymous, as always for reads.
	if _, err := runAuthJWT(t, issuer, newTestTransport("/v1/articles/list", mapHeader{})); err != nil {
		t.Fatalf("public without token error = %v, want nil", err)
	}
}
