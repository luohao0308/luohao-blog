package biz

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeSessionRepo keeps refresh sessions in memory and mimics the atomic
// consume semantics of Redis GETDEL: the first Consume removes the session,
// any later one misses.
type fakeSessionRepo struct {
	sessions map[string]*RefreshSession
	seq      int
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: map[string]*RefreshSession{}}
}

func (f *fakeSessionRepo) Create(_ context.Context, userID uuid.UUID, ttl time.Duration) (*RefreshSession, error) {
	f.seq++
	token := "refresh-" + string(rune('a'+f.seq-1))
	s := &RefreshSession{Token: token, UserID: userID, ExpiresAt: time.Now().Add(ttl)}
	f.sessions[token] = s
	return s, nil
}

func (f *fakeSessionRepo) Consume(_ context.Context, token string) (*RefreshSession, error) {
	s, ok := f.sessions[token]
	if !ok {
		return nil, ErrAuthInvalidRefreshToken
	}
	delete(f.sessions, token)
	return s, nil
}

func (f *fakeSessionRepo) Delete(_ context.Context, token string) error {
	delete(f.sessions, token)
	return nil
}

// fakeIssuer hands out deterministic tokens and verifies them against the
// ones it issued, like the real issuer verifies signatures.
type fakeIssuer struct {
	tokens map[string]*Claims
}

func newFakeIssuer() *fakeIssuer {
	return &fakeIssuer{tokens: map[string]*Claims{}}
}

func (f *fakeIssuer) Issue(_ context.Context, u *User) (*IssuedToken, error) {
	token := "access-" + u.ID.String()
	f.tokens[token] = &Claims{UserID: u.ID, Role: u.Role}
	return &IssuedToken{Token: token, ExpiresAt: time.Now().Add(DefaultAccessTokenTTL)}, nil
}

func (f *fakeIssuer) Parse(token string) (*Claims, error) {
	claims, ok := f.tokens[token]
	if !ok {
		return nil, ErrAuthUnauthorized
	}
	return claims, nil
}

// fakeLimiter allows the first n attempts per key, then denies. When fail
// withErr is set it reports an infrastructure error instead.
type fakeLimiter struct {
	n       int
	withErr error
	allowed map[string]int
}

func (f *fakeLimiter) Allow(_ context.Context, key string) (bool, error) {
	if f.withErr != nil {
		return false, f.withErr
	}
	if f.allowed == nil {
		f.allowed = map[string]int{}
	}
	f.allowed[key]++
	return f.allowed[key] <= f.n, nil
}

func newTestAuthUsecase() (*AuthUsecase, *fakeSessionRepo) {
	sessions := newFakeSessionRepo()
	uc := NewAuthUsecase(
		NewUserUsecase(newFakeUserRepo()),
		sessions,
		newFakeIssuer(),
		&fakeLimiter{n: 3},
		nil,
		time.Hour,
	)
	// The account every Login in this file authenticates against.
	if _, err := uc.users.CreateAuthor(context.Background(), "author@example.com", "longenough1", "作者"); err != nil {
		panic(err)
	}
	return uc, sessions
}

func loginTestAuthor(t *testing.T, uc *AuthUsecase) (*User, *TokenPair) {
	t.Helper()
	u, pair, err := uc.Login(context.Background(), "author@example.com", "longenough1", "10.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	return u, pair
}

func TestAuthUsecaseLoginIssuesPair(t *testing.T) {
	uc, sessions := newTestAuthUsecase()

	u, pair := loginTestAuthor(t, uc)
	if pair.Access.Token == "" || pair.Refresh.Token == "" {
		t.Fatalf("Login() pair = %+v, want both tokens issued", pair)
	}
	if pair.Access.ExpiresAt.Before(time.Now()) {
		t.Fatal("access token expires in the past")
	}
	session, ok := sessions.sessions[pair.Refresh.Token]
	if !ok || session.UserID != u.ID {
		t.Fatalf("refresh session not stored for the account: %+v", session)
	}
}

func TestAuthUsecaseLoginThrottled(t *testing.T) {
	uc, _ := newTestAuthUsecase()
	ctx := context.Background()

	// The limiter allows three attempts per IP; every login burns one, good
	// or bad, so brute forcing stops at the window boundary.
	for i := 0; i < 3; i++ {
		if _, _, err := uc.Login(ctx, "author@example.com", "wrong-password", "10.0.0.2"); err == nil {
			t.Fatalf("Login(wrong password #%d) = success, want credential error", i+1)
		}
	}
	if _, _, err := uc.Login(ctx, "author@example.com", "longenough1", "10.0.0.2"); !errors.Is(err, ErrAuthTooManyAttempts) {
		t.Fatalf("Login(throttled) error = %v, want too many attempts", err)
	}
	// A different IP is unaffected: the window is per client.
	if _, _, err := uc.Login(ctx, "author@example.com", "longenough1", "10.0.0.3"); err != nil {
		t.Fatalf("Login(other IP) error = %v, want success", err)
	}
}

func TestAuthUsecaseLoginLimiterFailOpen(t *testing.T) {
	users := NewUserUsecase(newFakeUserRepo())
	uc := NewAuthUsecase(
		users,
		newFakeSessionRepo(),
		newFakeIssuer(),
		&fakeLimiter{withErr: errors.New("redis down")},
		nil,
		time.Hour,
	)
	if _, err := users.CreateAuthor(context.Background(), "author@example.com", "longenough1", "作者"); err != nil {
		t.Fatalf("CreateAuthor() error = %v", err)
	}
	if _, _, err := uc.Login(context.Background(), "author@example.com", "longenough1", "10.0.0.1"); err != nil {
		t.Fatalf("Login(limiter down) error = %v, want fail-open success", err)
	}
}

func TestAuthUsecaseRefreshRotates(t *testing.T) {
	uc, _ := newTestAuthUsecase()
	ctx := context.Background()

	_, first := loginTestAuthor(t, uc)

	u, second, err := uc.Refresh(ctx, first.Refresh.Token)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if second.Refresh.Token == first.Refresh.Token {
		t.Fatal("Refresh() reused the refresh token; rotation is broken")
	}
	if u.Email != "author@example.com" {
		t.Fatalf("Refresh() user = %q, want the author", u.Email)
	}

	// The old token was consumed: replaying it must fail.
	if _, _, err := uc.Refresh(ctx, first.Refresh.Token); !errors.Is(err, ErrAuthInvalidRefreshToken) {
		t.Fatalf("Refresh(replayed) error = %v, want invalid refresh token", err)
	}
	// And the rotated one is still live.
	if _, _, err := uc.Refresh(ctx, second.Refresh.Token); err != nil {
		t.Fatalf("Refresh(rotated) error = %v, want success", err)
	}
}

func TestAuthUsecaseRefreshRejects(t *testing.T) {
	uc, sessions := newTestAuthUsecase()
	ctx := context.Background()

	if _, _, err := uc.Refresh(ctx, ""); !errors.Is(err, ErrAuthInvalidRefreshToken) {
		t.Fatalf("Refresh(empty) error = %v, want invalid refresh token", err)
	}
	if _, _, err := uc.Refresh(ctx, "never-issued"); !errors.Is(err, ErrAuthInvalidRefreshToken) {
		t.Fatalf("Refresh(unknown) error = %v, want invalid refresh token", err)
	}

	_, pair := loginTestAuthor(t, uc)
	delete(sessions.sessions, pair.Refresh.Token)
	if _, _, err := uc.Refresh(ctx, pair.Refresh.Token); !errors.Is(err, ErrAuthInvalidRefreshToken) {
		t.Fatalf("Refresh(revoked) error = %v, want invalid refresh token", err)
	}
}

func TestAuthUsecaseRefreshAfterAccountDeletion(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	sessions := newFakeSessionRepo()
	uc := NewAuthUsecase(NewUserUsecase(repo), sessions, newFakeIssuer(), &fakeLimiter{n: 100}, nil, time.Hour)

	if _, err := uc.users.CreateAuthor(ctx, "author@example.com", "longenough1", "作者"); err != nil {
		t.Fatalf("CreateAuthor() error = %v", err)
	}
	_, pair, err := uc.Login(ctx, "author@example.com", "longenough1", "10.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	delete(repo.users, "author@example.com")
	if _, _, err := uc.Refresh(ctx, pair.Refresh.Token); !errors.Is(err, ErrAuthInvalidRefreshToken) {
		t.Fatalf("Refresh(deleted account) error = %v, want invalid refresh token", err)
	}
}

func TestAuthUsecaseLogoutRevokes(t *testing.T) {
	uc, sessions := newTestAuthUsecase()
	ctx := context.Background()

	_, pair := loginTestAuthor(t, uc)
	if err := uc.Logout(ctx, pair.Refresh.Token); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, ok := sessions.sessions[pair.Refresh.Token]; ok {
		t.Fatal("Logout() left the session alive")
	}
	if _, _, err := uc.Refresh(ctx, pair.Refresh.Token); !errors.Is(err, ErrAuthInvalidRefreshToken) {
		t.Fatalf("Refresh(after logout) error = %v, want invalid refresh token", err)
	}
	// Logout without a session is still a successful logout.
	if err := uc.Logout(ctx, pair.Refresh.Token); err != nil {
		t.Fatalf("Logout(repeat) error = %v, want nil", err)
	}
	if err := uc.Logout(ctx, ""); err != nil {
		t.Fatalf("Logout(empty) error = %v, want nil", err)
	}
}

func TestAuthUsecaseRegisterSignsIn(t *testing.T) {
	uc, sessions := newTestAuthUsecase()
	ctx := context.Background()

	u, pair, err := uc.Register(ctx, "Reader@Example.COM", "longenough1", "  读者 ", "10.0.0.9")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if u.Role != UserRoleReader {
		t.Fatalf("Register() role = %d, want reader", u.Role)
	}
	if u.Email != "reader@example.com" || u.DisplayName != "读者" {
		t.Fatalf("Register() account = %q/%q, want normalized values", u.Email, u.DisplayName)
	}
	session, ok := sessions.sessions[pair.Refresh.Token]
	if !ok || session.UserID != u.ID {
		t.Fatalf("register did not start a session for the account: %+v", session)
	}
	// The fresh account refreshes like any login.
	if _, _, err := uc.Refresh(ctx, pair.Refresh.Token); err != nil {
		t.Fatalf("Refresh(after register) error = %v", err)
	}
}

func TestAuthUsecaseRegisterValidatesInput(t *testing.T) {
	uc, sessions := newTestAuthUsecase()
	ctx := context.Background()

	cases := []struct {
		name        string
		email       string
		password    string
		displayName string
	}{
		{"bad email", "not-an-email", "longenough1", "读者"},
		{"short password", "reader@example.com", "short", "读者"},
		{"empty name", "reader@example.com", "longenough1", "   "},
		{"name over 32 runes", "reader@example.com", "longenough1", strings.Repeat("名", MaxDisplayNameLen+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := uc.Register(ctx, tc.email, tc.password, tc.displayName, "10.0.0.9"); !errors.Is(err, ErrUserInvalidArgument) {
				t.Fatalf("Register(%s) error = %v, want invalid argument", tc.name, err)
			}
		})
	}
	if len(sessions.sessions) != 0 {
		t.Fatalf("rejected registrations left %d session(s) behind", len(sessions.sessions))
	}
}

func TestAuthUsecaseRegisterConflict(t *testing.T) {
	uc, _ := newTestAuthUsecase()

	// Emails match case-insensitively: the admin from the test fixture
	// blocks every casing variant.
	if _, _, err := uc.Register(context.Background(), "AUTHOR@EXAMPLE.COM", "longenough1", "读者", "10.0.0.9"); !errors.Is(err, ErrUserEmailConflict) {
		t.Fatalf("Register(conflict) error = %v, want email conflict", err)
	}
}

func TestAuthUsecaseRegisterThrottled(t *testing.T) {
	sessions := newFakeSessionRepo()
	uc := NewAuthUsecase(
		NewUserUsecase(newFakeUserRepo()),
		sessions,
		newFakeIssuer(),
		&fakeLimiter{n: 100},
		// One registration per IP; logins live on a separate budget.
		&fakeLimiter{n: 1},
		time.Hour,
	)
	ctx := context.Background()
	if _, err := uc.users.CreateAuthor(ctx, "author@example.com", "longenough1", "作者"); err != nil {
		t.Fatalf("CreateAuthor() error = %v", err)
	}

	if _, _, err := uc.Register(ctx, "reader1@example.com", "longenough1", "读者", "10.0.0.9"); err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	if _, _, err := uc.Register(ctx, "reader2@example.com", "longenough1", "读者", "10.0.0.9"); !errors.Is(err, ErrAuthTooManyAttempts) {
		t.Fatalf("Register(throttled) error = %v, want too many attempts", err)
	}
	// The register budget is independent: login attempts from the same IP
	// are still admitted.
	if _, _, err := uc.Login(ctx, "author@example.com", "longenough1", "10.0.0.9"); err != nil {
		t.Fatalf("Login(same IP) error = %v, want success", err)
	}
}
