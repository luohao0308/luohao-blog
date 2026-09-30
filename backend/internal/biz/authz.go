package biz

import (
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"

	"github.com/go-kratos/kratos/v3/errors"
)

// ErrAuthForbidden is returned when an authenticated account's role is not
// permitted to perform the operation. Distinct from ErrAuthUnauthorized,
// which is about missing or invalid credentials.
var ErrAuthForbidden = errors.Forbidden(v1.ErrorReason_AUTH_FORBIDDEN.String(), "role is not permitted for this operation")

// Authorization subjects. "public" stands in for unauthenticated requests;
// "authenticated" is the inheritance target every named role attaches to.
// These strings are the casbin policy vocabulary in
// internal/data/authz/policy.csv — renaming one is a policy change.
const (
	// SubjectPublic represents an unauthenticated request.
	SubjectPublic = "public"
	// SubjectAuthenticated is inherited by every named role.
	SubjectAuthenticated = "authenticated"
)

// RoleSubject maps an account role onto its policy subject. Unknown roles
// map to "" which no policy row matches: a stale token from before a role
// rename fails closed (403) rather than leaking access.
func RoleSubject(r UserRole) string {
	switch r {
	case UserRoleAdmin:
		return "admin"
	case UserRoleReader:
		return "reader"
	default:
		return ""
	}
}

// Authorizer consults the authorization policy. operation is the transport
// operation id (HTTP route template or gRPC full method); act is the HTTP
// method on HTTP and "*" on gRPC, where the method name already encodes the
// action.
type Authorizer interface {
	// Allowed reports whether the subject may perform the operation.
	Allowed(subject string, operation string, act string) bool
	// IsPublic reports whether the operation is reachable without
	// credentials.
	IsPublic(operation string, act string) bool
}
