package data

import (
	_ "embed"
	"fmt"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	stringadapter "github.com/casbin/casbin/v2/persist/string-adapter"
)

// The model and policy are embedded so the binary is self-contained and the
// reviewed files in this directory are the single source of truth.
var (
	//go:embed authz/model.conf
	authzModel string
	//go:embed authz/policy.csv
	authzPolicy string
)

type authorizer struct {
	enforcer *casbin.SyncedEnforcer
}

// NewAuthorizer builds the casbin enforcer from the embedded policy. A
// malformed model or policy aborts wiring: authorization must never boot in
// an unknown state.
func NewAuthorizer() (biz.Authorizer, error) {
	m, err := model.NewModelFromString(authzModel)
	if err != nil {
		return nil, fmt.Errorf("authz model: %w", err)
	}
	enforcer, err := casbin.NewSyncedEnforcer(m, stringadapter.NewAdapter(authzPolicy))
	if err != nil {
		return nil, fmt.Errorf("authz policy: %w", err)
	}
	return &authorizer{enforcer: enforcer}, nil
}

// Allowed evaluates the policy. Enforcer failures deny: the default is
// closed, so an infrastructure error can only reduce access, never grant it.
func (a *authorizer) Allowed(subject string, operation string, act string) bool {
	if subject == "" {
		return false
	}
	ok, err := a.enforcer.Enforce(subject, operation, act)
	return err == nil && ok
}

// IsPublic reports whether the operation is reachable unauthenticated.
func (a *authorizer) IsPublic(operation string, act string) bool {
	return a.Allowed(biz.SubjectPublic, operation, act)
}
