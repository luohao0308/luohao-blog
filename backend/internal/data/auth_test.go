package data

import (
	"context"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

// jwtTestConfig builds the auth config slice the issuer consumes.
func jwtTestConfig(secret string, ttl time.Duration) *conf.Auth {
	a := &conf.Auth{
		Jwt: &conf.Auth_JWT{Secret: secret},
	}
	if ttl != 0 {
		a.Jwt.AccessTokenTtl = durationpb.New(ttl)
	}
	return a
}

// limiterTestConfig builds the auth config slice the rate limiter consumes.
func limiterTestConfig(attempts int64, window time.Duration) *conf.Auth {
	a := &conf.Auth{
		RateLimit: &conf.Auth_RateLimit{LoginAttempts: attempts},
	}
	if window != 0 {
		a.RateLimit.LoginWindow = durationpb.New(window)
	}
	return a
}

func newTestRedis(t *testing.T) (redis.UniversalClient, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

func newTestIssuer(t *testing.T, secret string, ttl time.Duration) biz.TokenIssuer {
	t.Helper()
	issuer, err := NewTokenIssuer(jwtTestConfig(secret, ttl))
	if err != nil {
		t.Fatalf("NewTokenIssuer() error = %v", err)
	}
	return issuer
}

func TestTokenIssuerRequiresSecret(t *testing.T) {
	if _, err := NewTokenIssuer(jwtTestConfig("", time.Minute)); err == nil {
		t.Fatal("NewTokenIssuer(empty secret) = nil error, want refusal to start")
	}
}

func TestTokenIssuerRoundTrip(t *testing.T) {
	issuer := newTestIssuer(t, "test-secret", 15*time.Minute)
	u := &biz.User{ID: uuid.Must(uuid.NewV7()), Role: biz.UserRoleAdmin}

	issued, err := issuer.Issue(context.Background(), u)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if issued.ExpiresAt.Before(time.Now()) {
		t.Fatal("Issue() expiry is in the past")
	}

	claims, err := issuer.Parse(issued.Token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if claims.UserID != u.ID || claims.Role != biz.UserRoleAdmin {
		t.Fatalf("Parse() = %+v, want uid %s role admin", claims, u.ID)
	}
}

func TestTokenIssuerExpired(t *testing.T) {
	// Built directly with a negative TTL: the wire constructor maps unset or
	// non-positive TTLs to the default and never yields one.
	issuer := &tokenIssuer{key: []byte("test-secret"), ttl: -time.Minute}
	u := &biz.User{ID: uuid.Must(uuid.NewV7()), Role: biz.UserRoleAdmin}

	issued, err := issuer.Issue(context.Background(), u)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if _, err := issuer.Parse(issued.Token); err != biz.ErrAuthTokenExpired {
		t.Fatalf("Parse(expired) error = %v, want ErrAuthTokenExpired", err)
	}
}

func TestTokenIssuerForged(t *testing.T) {
	// The token is well-formed but signed with a different key: it must not
	// pass as authorized, only as forged.
	other := newTestIssuer(t, "attacker-secret", 15*time.Minute)
	u := &biz.User{ID: uuid.Must(uuid.NewV7()), Role: biz.UserRoleAdmin}
	issued, err := other.Issue(context.Background(), u)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	issuer := newTestIssuer(t, "test-secret", 15*time.Minute)
	if _, err := issuer.Parse(issued.Token); err != biz.ErrAuthUnauthorized {
		t.Fatalf("Parse(forged) error = %v, want ErrAuthUnauthorized", err)
	}
}

func TestTokenIssuerRejectsUnexpectedTokens(t *testing.T) {
	issuer := newTestIssuer(t, "test-secret", 15*time.Minute)
	uid := uuid.Must(uuid.NewV7()).String()

	// Alg confusion: a token claiming "none" must be rejected before the key
	// function is consulted.
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, accessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uid,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	})
	noneToken, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none token: %v", err)
	}
	if _, err := issuer.Parse(noneToken); err != biz.ErrAuthUnauthorized {
		t.Fatalf("Parse(alg none) error = %v, want ErrAuthUnauthorized", err)
	}

	// A token without exp must be rejected outright.
	noExp := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: uid},
	})
	noExpToken, err := noExp.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign no-exp token: %v", err)
	}
	if _, err := issuer.Parse(noExpToken); err != biz.ErrAuthUnauthorized {
		t.Fatalf("Parse(no exp) error = %v, want ErrAuthUnauthorized", err)
	}

	if _, err := issuer.Parse("not-a-token"); err != biz.ErrAuthUnauthorized {
		t.Fatalf("Parse(garbage) error = %v, want ErrAuthUnauthorized", err)
	}
}

func TestSessionRepoRotation(t *testing.T) {
	ctx := context.Background()
	rdb, mr := newTestRedis(t)
	repo := NewSessionRepo(rdb)
	uid := uuid.Must(uuid.NewV7())

	session, err := repo.Create(ctx, uid, time.Hour)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got := mr.TTL(refreshKeyPrefix + session.Token); got <= 0 {
		t.Fatalf("session TTL = %v, want the create TTL", got)
	}

	consumed, err := repo.Consume(ctx, session.Token)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if consumed.UserID != uid {
		t.Fatalf("Consume() = %s, want %s", consumed.UserID, uid)
	}
	if _, err := repo.Consume(ctx, session.Token); err != biz.ErrAuthInvalidRefreshToken {
		t.Fatalf("Consume(replayed) error = %v, want invalid refresh token", err)
	}
	if _, err := repo.Consume(ctx, "never-issued"); err != biz.ErrAuthInvalidRefreshToken {
		t.Fatalf("Consume(unknown) error = %v, want invalid refresh token", err)
	}
}

func TestSessionRepoExpiryAndDelete(t *testing.T) {
	ctx := context.Background()
	rdb, mr := newTestRedis(t)
	repo := NewSessionRepo(rdb)
	uid := uuid.Must(uuid.NewV7())

	session, err := repo.Create(ctx, uid, time.Hour)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	// Sliding window semantics come from the TTL: a session created later
	// with a shorter lifetime expires in lockstep with the store.
	mr.FastForward(2 * time.Hour)
	if _, err := repo.Consume(ctx, session.Token); err != biz.ErrAuthInvalidRefreshToken {
		t.Fatalf("Consume(expired) error = %v, want invalid refresh token", err)
	}

	session, err = repo.Create(ctx, uid, time.Hour)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Delete(ctx, session.Token); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Consume(ctx, session.Token); err != biz.ErrAuthInvalidRefreshToken {
		t.Fatalf("Consume(revoked) error = %v, want invalid refresh token", err)
	}
	// Deleting an unknown token is a no-op, not an error.
	if err := repo.Delete(ctx, session.Token); err != nil {
		t.Fatalf("Delete(unknown) error = %v", err)
	}
}

func TestRateLimiterWindow(t *testing.T) {
	ctx := context.Background()
	rdb, mr := newTestRedis(t)
	limiter := NewRateLimiter(rdb, limiterTestConfig(2, time.Minute))

	for i := 0; i < 2; i++ {
		ok, err := limiter.Allow(ctx, "login:10.0.0.1")
		if err != nil || !ok {
			t.Fatalf("Allow(#%d) = %v, %v; want allowed", i+1, ok, err)
		}
	}
	if ok, err := limiter.Allow(ctx, "login:10.0.0.1"); err != nil || ok {
		t.Fatalf("Allow(#3) = %v, %v; want denied", ok, err)
	}
	// A new fixed window resets the counter.
	mr.FastForward(2 * time.Minute)
	if ok, err := limiter.Allow(ctx, "login:10.0.0.1"); err != nil || !ok {
		t.Fatalf("Allow(next window) = %v, %v; want allowed", ok, err)
	}
	// Defaults apply when config is empty: attempts must still be bounded.
	defaultLimiter := NewRateLimiter(rdb, limiterTestConfig(0, 0))
	var allowed int
	for allowed = 0; allowed < 100; allowed++ {
		ok, err := defaultLimiter.Allow(ctx, "login:10.0.0.2")
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !ok {
			break
		}
	}
	if allowed != biz.DefaultLoginAttempts {
		t.Fatalf("default limiter allowed %d attempts, want %d", allowed, biz.DefaultLoginAttempts)
	}
}
