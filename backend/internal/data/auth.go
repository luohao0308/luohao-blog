package data

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// refreshKeyPrefix namespaces refresh session keys in Redis; rateKeyPrefix
// namespaces fixed-window counters.
const (
	refreshKeyPrefix = "blog:auth:refresh:"
	rateKeyPrefix    = "blog:ratelimit:"
)

// NewRedis opens the shared Redis client. The connection is established
// lazily: an unreachable Redis surfaces on the first command, keeping the
// seed command (which needs only MySQL) runnable without a broker.
func NewRedis(c *conf.Data) redis.UniversalClient {
	rc := c.GetRedis()
	network := rc.GetNetwork()
	if network == "" {
		network = "tcp"
	}
	opts := &redis.Options{
		Network:      network,
		Addr:         rc.GetAddr(),
		Password:     rc.GetPassword(),
		ReadTimeout:  rc.GetReadTimeout().AsDuration(),
		WriteTimeout: rc.GetWriteTimeout().AsDuration(),
	}
	return redis.NewClient(opts)
}

// accessTokenClaims is the JWT payload. The standard registered claims carry
// the subject (user id) and exp/iat; role rides along as a private claim.
type accessTokenClaims struct {
	jwt.RegisteredClaims
	Role int32 `json:"role"`
}

type tokenIssuer struct {
	key []byte
	ttl time.Duration
}

// NewTokenIssuer builds the HS256 access-token issuer. An empty signing key
// aborts wiring: the server must never boot with guessable credentials.
func NewTokenIssuer(a *conf.Auth) (biz.TokenIssuer, error) {
	secret := a.GetJwt().GetSecret()
	if secret == "" {
		return nil, fmt.Errorf("auth.jwt.secret is empty: export KRATOS_JWT_SECRET or set it in the config")
	}
	ttl := a.GetJwt().GetAccessTokenTtl().AsDuration()
	if ttl <= 0 {
		ttl = biz.DefaultAccessTokenTTL
	}
	return &tokenIssuer{key: []byte(secret), ttl: ttl}, nil
}

// Issue signs a short-lived HS256 access token for the account.
func (i *tokenIssuer) Issue(_ context.Context, u *biz.User) (*biz.IssuedToken, error) {
	now := time.Now()
	expiresAt := now.Add(i.ttl)
	claims := accessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Role: int32(u.Role),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.key)
	if err != nil {
		return nil, err
	}
	return &biz.IssuedToken{Token: signed, ExpiresAt: expiresAt}, nil
}

// Parse verifies signature, algorithm, and expiry. The allowed-algorithm
// pinning closes the alg-confusion attack: tokens claiming another algorithm
// are rejected before the key function runs.
func (i *tokenIssuer) Parse(token string) (*biz.Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &accessTokenClaims{}, func(*jwt.Token) (any, error) {
		return i.key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		if stderrors.Is(err, jwt.ErrTokenExpired) {
			return nil, biz.ErrAuthTokenExpired
		}
		return nil, biz.ErrAuthUnauthorized
	}
	claims, ok := parsed.Claims.(*accessTokenClaims)
	if !ok || !parsed.Valid {
		return nil, biz.ErrAuthUnauthorized
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, biz.ErrAuthUnauthorized
	}
	return &biz.Claims{UserID: uid, Role: biz.UserRole(claims.Role)}, nil
}

type sessionRepo struct {
	rdb redis.UniversalClient
}

// NewSessionRepo builds the Redis-backed refresh session store.
func NewSessionRepo(rdb redis.UniversalClient) biz.SessionRepo {
	return &sessionRepo{rdb: rdb}
}

// newOpaqueToken returns 256 bits of randomness, URL-safe and header-safe.
func newOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Create starts a refresh session with the given TTL. The TTL is applied
// fresh on every login and rotation, which is what makes the window sliding.
func (r *sessionRepo) Create(ctx context.Context, userID uuid.UUID, ttl time.Duration) (*biz.RefreshSession, error) {
	token, err := newOpaqueToken()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(ttl)
	if err := r.rdb.Set(ctx, refreshKeyPrefix+token, userID.String(), ttl).Err(); err != nil {
		return nil, err
	}
	return &biz.RefreshSession{Token: token, UserID: userID, ExpiresAt: expiresAt}, nil
}

// Consume atomically reads and deletes the session: two concurrent refreshes
// with the same token yield exactly one winner, so rotated tokens cannot be
// replayed.
func (r *sessionRepo) Consume(ctx context.Context, token string) (*biz.RefreshSession, error) {
	val, err := r.rdb.GetDel(ctx, refreshKeyPrefix+token).Result()
	if err != nil {
		if stderrors.Is(err, redis.Nil) {
			return nil, biz.ErrAuthInvalidRefreshToken
		}
		return nil, err
	}
	uid, err := uuid.Parse(val)
	if err != nil {
		return nil, biz.ErrAuthInvalidRefreshToken
	}
	return &biz.RefreshSession{Token: token, UserID: uid}, nil
}

// Delete revokes the session; deleting an unknown token is a no-op.
func (r *sessionRepo) Delete(ctx context.Context, token string) error {
	return r.rdb.Del(ctx, refreshKeyPrefix+token).Err()
}

type rateLimiter struct {
	rdb      redis.UniversalClient
	attempts int64
	window   time.Duration
}

// NewRateLimiter builds the login throttler from the auth config, applying
// the documented defaults when values are unset.
func NewRateLimiter(rdb redis.UniversalClient, a *conf.Auth) biz.RateLimiter {
	attempts := a.GetRateLimit().GetLoginAttempts()
	if attempts <= 0 {
		attempts = biz.DefaultLoginAttempts
	}
	window := a.GetRateLimit().GetLoginWindow().AsDuration()
	if window <= 0 {
		window = biz.DefaultLoginWindow
	}
	return &rateLimiter{rdb: rdb, attempts: attempts, window: window}
}

// Allow increments the fixed-window counter for key. The window TTL is set
// only on the first increment of a window so retries do not extend it.
func (r *rateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	k := rateKeyPrefix + key
	pipe := r.rdb.TxPipeline()
	incr := pipe.Incr(ctx, k)
	pipe.ExpireNX(ctx, k, r.window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	return incr.Val() <= r.attempts, nil
}

// NewRegisterRateLimiter builds the registration throttler from the auth
// config, applying the same defaults pattern as the login throttler.
func NewRegisterRateLimiter(rdb redis.UniversalClient, a *conf.Auth) biz.RegisterRateLimiter {
	attempts := a.GetRateLimit().GetRegisterAttempts()
	if attempts <= 0 {
		attempts = biz.DefaultRegisterAttempts
	}
	window := a.GetRateLimit().GetRegisterWindow().AsDuration()
	if window <= 0 {
		window = biz.DefaultRegisterWindow
	}
	return &rateLimiter{rdb: rdb, attempts: attempts, window: window}
}

// NewRefreshTokenTTL exposes the session TTL to wire as a plain value; the
// auth usecase takes primitives, not the whole config tree.
func NewRefreshTokenTTL(a *conf.Auth) time.Duration {
	ttl := a.GetRefreshTokenTtl().AsDuration()
	if ttl <= 0 {
		return biz.DefaultRefreshTokenTTL
	}
	return ttl
}
