package biz

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type fakeEmailTokenStore struct {
	issued  map[string]EmailTokenClaims
	revoked int
	fail    error
	users   *fakeUserRepo
}

func newFakeEmailTokenStore() *fakeEmailTokenStore {
	return &fakeEmailTokenStore{issued: map[string]EmailTokenClaims{}}
}
func (s *fakeEmailTokenStore) Issue(_ context.Context, purpose EmailPurpose, userID uuid.UUID, version uint64, _ time.Duration) (string, error) {
	if s.fail != nil {
		return "", s.fail
	}
	token := string(purpose) + "-token"
	s.issued[token] = EmailTokenClaims{Purpose: purpose, UserID: userID, CredentialVersion: version}
	return token, nil
}
func (s *fakeEmailTokenStore) Consume(_ context.Context, purpose EmailPurpose, token string) (*EmailTokenClaims, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	claims, ok := s.issued[token]
	if !ok || claims.Purpose != purpose {
		return nil, ErrEmailTokenInvalid
	}
	delete(s.issued, token)
	return &claims, nil
}
func (s *fakeEmailTokenStore) IsCurrent(_ context.Context, purpose EmailPurpose, userID uuid.UUID, version uint64, token string) (bool, error) {
	claims, ok := s.issued[token]
	return ok && claims.Purpose == purpose && claims.UserID == userID && claims.CredentialVersion == version, s.fail
}
func (s *fakeEmailTokenStore) Validate(_ context.Context, purpose EmailPurpose, token string) (*EmailTokenClaims, bool, error) {
	claims, ok := s.issued[token]
	if !ok || claims.Purpose != purpose {
		return nil, false, s.fail
	}
	copy := claims
	return &copy, true, s.fail
}
func (s *fakeEmailTokenStore) Revoke(_ context.Context, _ EmailPurpose, _ uuid.UUID, token string) error {
	delete(s.issued, token)
	s.revoked++
	return s.fail
}

func (s *fakeEmailTokenStore) RevokeUser(_ context.Context, userID uuid.UUID) error {
	for token, claims := range s.issued {
		if claims.UserID == userID {
			delete(s.issued, token)
		}
	}
	s.revoked++
	return s.fail
}
func (s *fakeEmailTokenStore) ConfirmVerification(_ context.Context, token string) (uuid.UUID, bool, error) {
	claims, ok := s.issued[token]
	if !ok || claims.Purpose != EmailPurposeVerify {
		return uuid.Nil, false, nil
	}
	delete(s.issued, token)
	for _, u := range s.users.users {
		if u.ID == claims.UserID && u.CredentialVersion == claims.CredentialVersion {
			u.EmailVerified = true
			return u.ID, true, nil
		}
	}
	return uuid.Nil, false, nil
}
func (s *fakeEmailTokenStore) ConfirmPasswordReset(_ context.Context, token, passwordHash string) (uuid.UUID, uint64, bool, error) {
	claims, ok := s.issued[token]
	if !ok || claims.Purpose != EmailPurposeReset {
		return uuid.Nil, 0, false, nil
	}
	delete(s.issued, token)
	for _, u := range s.users.users {
		if u.ID == claims.UserID && u.CredentialVersion == claims.CredentialVersion {
			u.PasswordHash = passwordHash
			u.EmailVerified = true
			u.CredentialVersion++
			return u.ID, u.CredentialVersion, true, nil
		}
	}
	return uuid.Nil, 0, false, nil
}

type fakeEmailLimiter struct {
	allowed bool
	err     error
	keys    []string
	limits  []int64
	windows []time.Duration
}

func (l *fakeEmailLimiter) Allow(_ context.Context, key string, limit int64, window time.Duration) (bool, error) {
	l.keys = append(l.keys, key)
	l.limits = append(l.limits, limit)
	l.windows = append(l.windows, window)
	return l.allowed, l.err
}

type fakeMailSender struct {
	configured bool
	fail       error
	messages   []AccountEmailMessage
}

func (s *fakeMailSender) Configured() bool { return s.configured }
func (s *fakeMailSender) Send(_ context.Context, message AccountEmailMessage) error {
	s.messages = append(s.messages, message)
	return s.fail
}

func TestAccountEmailVerificationLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	users := NewUserUsecase(repo)
	u, err := users.CreateAccount(ctx, "reader@example.com", "longenough1", "Reader", UserRoleReader)
	if err != nil {
		t.Fatal(err)
	}
	tokens := newFakeEmailTokenStore()
	tokens.users = repo
	mailer := &fakeMailSender{configured: true}
	uc := NewAccountEmailUsecase(users, tokens, &fakeEmailLimiter{allowed: true}, mailer, newFakeSessionRepo(), AccountEmailSettings{SiteOrigin: "https://example.com"})
	if err = uc.RequestVerification(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if len(mailer.messages) != 1 || !strings.Contains(mailer.messages[0].Link, "/verify-email#token=") {
		t.Fatalf("messages = %+v", mailer.messages)
	}
	if err = uc.ConfirmVerification(ctx, "verify-token"); err != nil {
		t.Fatal(err)
	}
	got, _ := users.ByID(ctx, u.ID)
	if !got.EmailVerified {
		t.Fatal("email not verified")
	}
	if err = uc.ConfirmVerification(ctx, "verify-token"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("replay = %v", err)
	}
}

func TestPasswordResetLifecycleAndEnumeration(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	users := NewUserUsecase(repo)
	u, _ := users.CreateAccount(ctx, "reader@example.com", "longenough1", "Reader", UserRoleReader)
	sessions := newFakeSessionRepo()
	tokens := newFakeEmailTokenStore()
	tokens.users = repo
	mailer := &fakeMailSender{configured: true}
	limiter := &fakeEmailLimiter{allowed: true}
	uc := NewAccountEmailUsecase(users, tokens, limiter, mailer, sessions, AccountEmailSettings{SiteOrigin: "https://example.com"})
	if err := uc.RequestPasswordReset(ctx, "missing@example.com", "127.0.0.1"); err != nil {
		t.Fatalf("unknown request = %v", err)
	}
	uc.processResetEmail(ctx, u.Email)
	if err := uc.ConfirmPasswordReset(ctx, "reset-token", "newpassword1"); err != nil {
		t.Fatal(err)
	}
	got, _ := users.ByID(ctx, u.ID)
	if !got.EmailVerified || got.CredentialVersion != 1 || !VerifyPassword("newpassword1", got.PasswordHash) {
		t.Fatalf("reset user = %+v", got)
	}
}

func TestAccountEmailFailuresAreSafe(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	users := NewUserUsecase(repo)
	u, _ := users.CreateAccount(ctx, "reader@example.com", "longenough1", "Reader", UserRoleReader)
	tokens := newFakeEmailTokenStore()
	tokens.users = repo
	mailer := &fakeMailSender{configured: true, fail: errors.New("smtp auth secret@example.com raw-token")}
	uc := NewAccountEmailUsecase(users, tokens, &fakeEmailLimiter{allowed: true}, mailer, newFakeSessionRepo(), AccountEmailSettings{SiteOrigin: "https://example.com"})
	if err := uc.RequestVerification(ctx, u.ID); !kratoserrors.IsServiceUnavailable(err) || strings.Contains(err.Error(), "secret@example.com") {
		t.Fatalf("verify send error = %v", err)
	}
	uc.processResetEmail(ctx, u.Email)
	if len(tokens.issued) != 0 {
		t.Fatalf("failed sends left tokens: %+v", tokens.issued)
	}
}

func TestInvalidResetTokenDoesNotRunArgon2(t *testing.T) {
	repo := newFakeUserRepo()
	users := NewUserUsecase(repo)
	tokens := newFakeEmailTokenStore()
	tokens.users = repo
	uc := NewAccountEmailUsecase(users, tokens, &fakeEmailLimiter{allowed: true}, &fakeMailSender{configured: true}, newFakeSessionRepo(), AccountEmailSettings{})
	calls := 0
	uc.hashPassword = func(string) (string, error) { calls++; return "hash", nil }
	if err := uc.ConfirmPasswordReset(context.Background(), "attacker-generated-token", "longenough1"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("invalid reset = %v", err)
	}
	if calls != 0 {
		t.Fatalf("Argon2 calls = %d, want 0", calls)
	}
}

func TestConcurrentResetConfirmRunsAtMostOneHasher(t *testing.T) {
	repo := newFakeUserRepo()
	users := NewUserUsecase(repo)
	u, _ := users.CreateAccount(context.Background(), "concurrent@example.com", "longenough1", "Concurrent", UserRoleReader)
	tokens := newFakeEmailTokenStore()
	tokens.users = repo
	_, _ = tokens.Issue(context.Background(), EmailPurposeReset, u.ID, 0, time.Hour)
	uc := NewAccountEmailUsecase(users, tokens, &fakeEmailLimiter{allowed: true}, &fakeMailSender{configured: true}, newFakeSessionRepo(), AccountEmailSettings{})
	// Retain the first notification even when the observer has not started waiting.
	entered, release := make(chan struct{}, 1), make(chan struct{})
	active, maxActive := 0, 0
	var mu sync.Mutex
	uc.hashPassword = func(string) (string, error) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		mu.Lock()
		active--
		mu.Unlock()
		return "hash", nil
	}
	results := make(chan error, 8)
	start := make(chan struct{})
	for range 8 {
		go func() {
			<-start
			results <- uc.ConfirmPasswordReset(context.Background(), "reset-token", "newpassword1")
		}()
	}
	close(start)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("hasher did not start")
	}
	close(release)
	success := 0
	for range 8 {
		if err := <-results; err == nil {
			success++
		}
	}
	if success != 1 || maxActive != 1 {
		t.Fatalf("success=%d maxActive=%d", success, maxActive)
	}
}

func TestCanceledResetConfirmDoesNotConsumeOrHash(t *testing.T) {
	repo := newFakeUserRepo()
	users := NewUserUsecase(repo)
	u, _ := users.CreateAccount(context.Background(), "cancel@example.com", "longenough1", "Cancel", UserRoleReader)
	tokens := newFakeEmailTokenStore()
	tokens.users = repo
	_, _ = tokens.Issue(context.Background(), EmailPurposeReset, u.ID, 0, time.Hour)
	uc := NewAccountEmailUsecase(users, tokens, &fakeEmailLimiter{allowed: true}, &fakeMailSender{configured: true}, newFakeSessionRepo(), AccountEmailSettings{})
	calls := 0
	uc.hashPassword = func(string) (string, error) { calls++; return "hash", nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := uc.ConfirmPasswordReset(ctx, "reset-token", "newpassword1"); !kratoserrors.IsServiceUnavailable(err) {
		t.Fatalf("canceled confirm = %v", err)
	}
	if calls != 0 {
		t.Fatalf("canceled hash calls = %d", calls)
	}
	if _, ok := tokens.issued["reset-token"]; !ok {
		t.Fatal("canceled confirm consumed token")
	}
}

func TestPasswordResetQueueAdmissionIsUniformAndBounded(t *testing.T) {
	repo := newFakeUserRepo()
	users := NewUserUsecase(repo)
	_, _ = users.CreateAccount(context.Background(), "known@example.com", "longenough1", "Known", UserRoleReader)
	tokens := newFakeEmailTokenStore()
	tokens.users = repo
	uc := NewAccountEmailUsecase(users, tokens, &fakeEmailLimiter{allowed: true}, &fakeMailSender{configured: true}, newFakeSessionRepo(), AccountEmailSettings{SiteOrigin: "https://example.com"})
	if err := uc.RequestPasswordReset(context.Background(), "known@example.com", "ip-a"); err != nil {
		t.Fatal(err)
	}
	if err := uc.RequestPasswordReset(context.Background(), "unknown@example.com", "ip-b"); err != nil {
		t.Fatal(err)
	}
	if len(uc.queue) != 2 {
		t.Fatalf("queue len = %d", len(uc.queue))
	}
	for len(uc.queue) < cap(uc.queue) {
		uc.queue <- resetEmailJob{email: "unknown@example.com"}
	}
	if err := uc.RequestPasswordReset(context.Background(), "overflow@example.com", "ip-c"); !kratoserrors.IsServiceUnavailable(err) {
		t.Fatalf("queue full = %v", err)
	}

	disabled := NewAccountEmailUsecase(users, tokens, &fakeEmailLimiter{allowed: true}, &fakeMailSender{}, newFakeSessionRepo(), AccountEmailSettings{})
	if err := disabled.RequestPasswordReset(context.Background(), "known@example.com", "ip"); !kratoserrors.IsServiceUnavailable(err) {
		t.Fatalf("disabled known = %v", err)
	}
	if err := disabled.RequestPasswordReset(context.Background(), "unknown@example.com", "ip"); !kratoserrors.IsServiceUnavailable(err) {
		t.Fatalf("disabled unknown = %v", err)
	}
	failingLimit := NewAccountEmailUsecase(users, tokens, &fakeEmailLimiter{allowed: false, err: errors.New("redis offline")}, &fakeMailSender{configured: true}, newFakeSessionRepo(), AccountEmailSettings{})
	if err := failingLimit.RequestPasswordReset(context.Background(), "known@example.com", "ip"); !kratoserrors.IsServiceUnavailable(err) {
		t.Fatalf("limiter outage = %v", err)
	}
}

func TestAccountEmailUsesApprovedRateLimits(t *testing.T) {
	repo := newFakeUserRepo()
	users := NewUserUsecase(repo)
	u, _ := users.CreateAccount(context.Background(), "limits@example.com", "longenough1", "Limits", UserRoleReader)
	tokens := newFakeEmailTokenStore()
	tokens.users = repo
	verifyLimiter := &fakeEmailLimiter{allowed: true}
	verifyUC := NewAccountEmailUsecase(users, tokens, verifyLimiter, &fakeMailSender{configured: true}, newFakeSessionRepo(), AccountEmailSettings{SiteOrigin: "https://example.com"})
	if err := verifyUC.RequestVerification(context.Background(), u.ID); err != nil {
		t.Fatal(err)
	}
	if len(verifyLimiter.limits) != 2 || verifyLimiter.limits[0] != 1 || verifyLimiter.windows[0] != time.Minute || verifyLimiter.limits[1] != 5 || verifyLimiter.windows[1] != time.Hour {
		t.Fatalf("verify limits = %v/%v", verifyLimiter.limits, verifyLimiter.windows)
	}
	resetLimiter := &fakeEmailLimiter{allowed: true}
	resetUC := NewAccountEmailUsecase(users, tokens, resetLimiter, &fakeMailSender{configured: true}, newFakeSessionRepo(), AccountEmailSettings{})
	if err := resetUC.RequestPasswordReset(context.Background(), u.Email, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if len(resetLimiter.limits) != 3 || resetLimiter.limits[0] != 1 || resetLimiter.limits[1] != 3 || resetLimiter.limits[2] != 5 {
		t.Fatalf("reset limits = %v", resetLimiter.limits)
	}
}

type blockingMailSender struct{ entered chan struct{} }

func (s *blockingMailSender) Configured() bool { return true }
func (s *blockingMailSender) Send(ctx context.Context, _ AccountEmailMessage) error {
	close(s.entered)
	<-ctx.Done()
	return ctx.Err()
}

func TestAccountEmailWorkerStopCancelsStalledDelivery(t *testing.T) {
	repo := newFakeUserRepo()
	users := NewUserUsecase(repo)
	_, _ = users.CreateAccount(context.Background(), "worker@example.com", "longenough1", "Worker", UserRoleReader)
	tokens := newFakeEmailTokenStore()
	tokens.users = repo
	mailer := &blockingMailSender{entered: make(chan struct{})}
	uc := NewAccountEmailUsecase(users, tokens, &fakeEmailLimiter{allowed: true}, mailer, newFakeSessionRepo(), AccountEmailSettings{SiteOrigin: "https://example.com"})
	if err := uc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := uc.RequestPasswordReset(context.Background(), "worker@example.com", "ip"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-mailer.entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not start delivery")
	}
	done := make(chan struct{})
	go func() { _ = uc.Stop(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker stop blocked on delivery")
	}
}
