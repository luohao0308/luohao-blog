package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/uuid"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
)

var (
	// ErrAuthUnauthorized is returned when an endpoint requires an access
	// token and none is presented, or the presented one is malformed or
	// forged.
	ErrAuthUnauthorized = errors.Unauthorized(v1.ErrorReason_AUTH_UNAUTHORIZED.String(), "missing or invalid access token")
	// ErrAuthTokenExpired is returned when the access token signature is
	// valid but its expiry has passed. Clients should refresh on this reason.
	ErrAuthTokenExpired = errors.Unauthorized(v1.ErrorReason_AUTH_TOKEN_EXPIRED.String(), "access token expired")
	// ErrAuthInvalidRefreshToken is returned when a refresh token is missing,
	// unknown, revoked, replayed after rotation, or belongs to a deleted
	// account. One reason for all of these: refresh tokens are opaque, so
	// there is nothing useful to distinguish.
	ErrAuthInvalidRefreshToken = errors.Unauthorized(v1.ErrorReason_AUTH_INVALID_REFRESH_TOKEN.String(), "invalid or expired refresh token")
	// ErrAuthTooManyAttempts is returned when the login rate limit is hit.
	ErrAuthTooManyAttempts = errors.TooManyRequests(v1.ErrorReason_AUTH_TOO_MANY_ATTEMPTS.String(), "too many login attempts, try again later")
)

// Defaults for the auth policy knobs when config leaves them unset.
const (
	DefaultAccessTokenTTL  = 15 * time.Minute
	DefaultRefreshTokenTTL = 7 * 24 * time.Hour
	DefaultLoginAttempts   = 10
	DefaultLoginWindow     = 5 * time.Minute
)

// Claims is the payload extracted from a verified access token.
type Claims struct {
	UserID uuid.UUID
	Role   UserRole
}

type authContextKey struct{}

// NewAuthContext stores verified claims on the request context.
func NewAuthContext(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, authContextKey{}, claims)
}

// AuthFromContext returns the verified claims, or false when the request
// carried no valid access token.
func AuthFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(authContextKey{}).(*Claims)
	return claims, ok && claims != nil
}

// IssuedToken is a freshly signed access token.
type IssuedToken struct {
	Token     string
	ExpiresAt time.Time
}

// TokenIssuer signs and verifies access tokens. The implementation lives in
// the data layer next to the other infrastructure clients; biz only sees the
// capability.
type TokenIssuer interface {
	// Issue signs an access token for the account with the configured TTL.
	Issue(ctx context.Context, u *User) (*IssuedToken, error)
	// Parse verifies signature, algorithm, and expiry, and returns the
	// claims. Expired tokens return an error that errors.IsUnauthorized
	// maps to ErrAuthTokenExpired.
	Parse(token string) (*Claims, error)
}

// RefreshSession is a server-side login session addressed by an opaque
// refresh token.
type RefreshSession struct {
	Token     string
	UserID    uuid.UUID
	ExpiresAt time.Time
}

// SessionRepo stores refresh sessions in Redis. The token is the key; the
// server holds the only authoritative copy, so revocation is immediate.
type SessionRepo interface {
	// Create starts a session for the account, returning a fresh opaque token.
	Create(ctx context.Context, userID uuid.UUID, ttl time.Duration) (*RefreshSession, error)
	// Consume atomically reads and removes the session, so a rotated token can
	// never be redeemed twice. Missing sessions return ErrAuthInvalidRefreshToken.
	Consume(ctx context.Context, token string) (*RefreshSession, error)
	// Delete revokes the session. It is a no-op for unknown tokens.
	Delete(ctx context.Context, token string) error
}

// RateLimiter is a fixed-window counter used to throttle brute-forceable
// endpoints. The caller supplies a namespaced key such as "login:<ip>".
type RateLimiter interface {
	// Allow records one attempt under key and reports whether the caller may
	// proceed. Infrastructure failures are reported as errors.
	Allow(ctx context.Context, key string) (bool, error)
}

// TokenPair bundles what a login or refresh round hands to the client: the
// short-lived access token plus the opaque refresh token (delivered as a
// cookie by the transport layer).
type TokenPair struct {
	Access  *IssuedToken
	Refresh *RefreshSession
}

// AuthUsecase is the authentication usecase: credential login with
// brute-force throttling, refresh rotation, and revocation.
type AuthUsecase struct {
	users      *UserUsecase
	sessions   SessionRepo
	issuer     TokenIssuer
	limiter    RateLimiter
	refreshTTL time.Duration
}

// NewAuthUsecase new an Auth usecase.
func NewAuthUsecase(users *UserUsecase, sessions SessionRepo, issuer TokenIssuer, limiter RateLimiter, refreshTTL time.Duration) *AuthUsecase {
	if refreshTTL <= 0 {
		refreshTTL = DefaultRefreshTokenTTL
	}
	return &AuthUsecase{users: users, sessions: sessions, issuer: issuer, limiter: limiter, refreshTTL: refreshTTL}
}

// Login verifies credentials and starts a session. Throttling is per client
// IP; the limiter fails open so a Redis outage cannot lock every author out.
func (uc *AuthUsecase) Login(ctx context.Context, email, password, clientIP string) (*User, *TokenPair, error) {
	if uc.limiter != nil {
		ok, err := uc.limiter.Allow(ctx, "login:"+clientIP)
		if err != nil {
			log.Warn("auth: rate limiter unavailable, failing open", "err", err)
		} else if !ok {
			return nil, nil, ErrAuthTooManyAttempts
		}
	}
	u, err := uc.users.Authenticate(ctx, email, password)
	if err != nil {
		return nil, nil, err
	}
	pair, err := uc.issuePair(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

// Refresh rotates the session: the presented token is consumed atomically
// (replays fail), the account is reloaded so a deleted account or a role
// change takes effect, and a brand-new pair is issued.
func (uc *AuthUsecase) Refresh(ctx context.Context, refreshToken string) (*User, *TokenPair, error) {
	if refreshToken == "" {
		return nil, nil, ErrAuthInvalidRefreshToken
	}
	session, err := uc.sessions.Consume(ctx, refreshToken)
	if err != nil {
		return nil, nil, err
	}
	u, err := uc.users.ByID(ctx, session.UserID)
	if err != nil {
		return nil, nil, ErrAuthInvalidRefreshToken
	}
	pair, err := uc.issuePair(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

// Logout revokes the session. Unknown tokens are still a successful logout.
func (uc *AuthUsecase) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return uc.sessions.Delete(ctx, refreshToken)
}

// CurrentUser resolves the account behind verified claims, for endpoints like
// GetMe. A token for an account that has since been deleted is rejected.
func (uc *AuthUsecase) CurrentUser(ctx context.Context, claims *Claims) (*User, error) {
	if claims == nil {
		return nil, ErrAuthUnauthorized
	}
	return uc.users.ByID(ctx, claims.UserID)
}

// issuePair signs the access token and starts a fresh refresh session.
func (uc *AuthUsecase) issuePair(ctx context.Context, u *User) (*TokenPair, error) {
	access, err := uc.issuer.Issue(ctx, u)
	if err != nil {
		return nil, err
	}
	refresh, err := uc.sessions.Create(ctx, u.ID, uc.refreshTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{Access: access, Refresh: refresh}, nil
}
