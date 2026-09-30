package data

import (
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

// The tests assert the actual embedded policy: every row must hold, and the
// negatives prove the gates. Adding an endpoint without a policy row fails
// here (and at boot via validatePolicyCoverage).
func newTestAuthorizer(t *testing.T) biz.Authorizer {
	t.Helper()
	authz, err := NewAuthorizer()
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}
	return authz
}

func TestPolicyMatrix(t *testing.T) {
	authz := newTestAuthorizer(t)
	cases := []struct {
		name      string
		subject   string
		operation string
		act       string
		want      bool
	}{
		// Public reads and the auth flow.
		{"public list", "public", "/v1/articles/list", "GET", true},
		{"public get", "public", "/v1/articles/{slug}", "GET", true},
		{"public login", "public", "/v1/auth/login", "POST", true},
		{"public refresh", "public", "/v1/auth/refresh", "POST", true},
		{"public logout", "public", "/v1/auth/logout", "POST", true},
		// Writes are closed to the public, including by method mismatch.
		{"public create denied", "public", "/v1/articles/create", "POST", false},
		{"public update denied", "public", "/v1/articles/update", "PUT", false},
		{"public delete denied", "public", "/v1/articles/{slug}", "DELETE", false},
		{"public wrong method on list", "public", "/v1/articles/list", "POST", false},
		{"public me denied", "public", "/v1/auth/me", "GET", false},
		// Authenticated: any named role reaches /me via inheritance.
		{"admin me", "admin", "/v1/auth/me", "GET", true},
		{"reader me", "reader", "/v1/auth/me", "GET", true},
		// Admin writes.
		{"admin create", "admin", "/v1/articles/create", "POST", true},
		{"admin update", "admin", "/v1/articles/update", "PUT", true},
		{"admin delete", "admin", "/v1/articles/{slug}", "DELETE", true},
		// Reader inherits reads only.
		{"reader create denied", "reader", "/v1/articles/create", "POST", false},
		{"reader update denied", "reader", "/v1/articles/update", "PUT", false},
		{"reader delete denied", "reader", "/v1/articles/{slug}", "DELETE", false},
		// Reader may still read.
		{"reader list", "reader", "/v1/articles/list", "GET", true},
		// Method must match exactly for HTTP rows.
		{"admin create via GET denied", "admin", "/v1/articles/create", "GET", false},
		// gRPC forms.
		{"grpc public list", "public", "/blog.v1.ArticleService/ListArticles", "*", true},
		{"grpc admin create", "admin", "/blog.v1.ArticleService/CreateArticle", "*", true},
		{"grpc reader create denied", "reader", "/blog.v1.ArticleService/CreateArticle", "*", false},
		{"grpc reader get", "reader", "/blog.v1.ArticleService/GetArticle", "*", true},
		{"grpc admin me", "admin", "/blog.v1.AuthService/GetMe", "*", true},
		// Default deny: unknown operations and subjects.
		{"unknown operation", "admin", "/v1/articles/batch", "POST", false},
		{"unknown subject", "editor", "/v1/articles/create", "POST", false},
		{"empty subject", "", "/v1/articles/create", "POST", false},
	}
	for _, tc := range cases {
		if got := authz.Allowed(tc.subject, tc.operation, tc.act); got != tc.want {
			t.Errorf("Allowed(%q, %q, %q) = %v, want %v", tc.subject, tc.operation, tc.act, got, tc.want)
		}
	}
}

func TestIsPublic(t *testing.T) {
	authz := newTestAuthorizer(t)
	for _, tc := range []struct {
		operation string
		act       string
		want      bool
	}{
		{"/v1/articles/list", "GET", true},
		{"/v1/articles/{slug}", "GET", true},
		{"/v1/auth/login", "POST", true},
		{"/v1/auth/me", "GET", false},
		{"/v1/articles/create", "POST", false},
	} {
		if got := authz.IsPublic(tc.operation, tc.act); got != tc.want {
			t.Errorf("IsPublic(%q, %q) = %v, want %v", tc.operation, tc.act, got, tc.want)
		}
	}
}

func TestRoleSubject(t *testing.T) {
	for _, tc := range []struct {
		role biz.UserRole
		want string
	}{
		{biz.UserRoleAdmin, "admin"},
		{biz.UserRoleReader, "reader"},
		{biz.UserRoleUnspecified, ""},
		{biz.UserRole(99), ""},
	} {
		if got := biz.RoleSubject(tc.role); got != tc.want {
			t.Errorf("RoleSubject(%d) = %q, want %q", tc.role, got, tc.want)
		}
	}
}
