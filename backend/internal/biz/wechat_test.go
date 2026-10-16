package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

// fakeWechatClient maps codes to openids or a fixed error, standing in for
// the jscode2session round trip.
type fakeWechatClient struct {
	appID string
	codes map[string]string
	err   error
}

func (f *fakeWechatClient) AppID() string {
	if f.appID != "" {
		return f.appID
	}
	return "wx-fixture"
}
func (f *fakeWechatClient) Configured() bool { return !kratoserrors.Is(f.err, ErrWechatNotConfigured) }

func (f *fakeWechatClient) Code2Session(_ context.Context, code string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	openid, ok := f.codes[code]
	if !ok {
		return "", ErrWechatCodeInvalid
	}
	return openid, nil
}

// fakeBindingStore keeps tickets in memory with one-shot consume semantics.
type fakeBindingStore struct {
	tickets map[string]string
	next    int
}

func newFakeBindingStore() *fakeBindingStore {
	return &fakeBindingStore{tickets: map[string]string{}}
}

func (f *fakeBindingStore) Save(_ context.Context, appID, openid string, _ time.Duration) (string, error) {
	f.next++
	ticket := "ticket-" + strings.Repeat("t", f.next)
	f.tickets[appID+":"+ticket] = openid
	return ticket, nil
}

func (f *fakeBindingStore) Consume(_ context.Context, appID, ticket string) (string, error) {
	openid, ok := f.tickets[appID+":"+ticket]
	if !ok {
		return "", ErrWechatTicketInvalid
	}
	delete(f.tickets, appID+":"+ticket)
	return openid, nil
}

func newWechatUsecase(repo *fakeUserRepo, client WechatClient, bindings WechatBindingStore, limiter RateLimiter) *WechatUsecase {
	users := NewUserUsecase(repo)
	auth := NewAuthUsecase(users, newFakeSessionRepo(), newFakeIssuer(), limiter, nil, 0)
	return NewWechatUsecase(users, auth, client, bindings, limiter)
}

func TestWechatLoginBoundOpenidSignsIn(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	u, err := repo.Create(ctx, &User{Email: "reader@example.com", PasswordHash: "x", Role: UserRoleReader})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.BindWechatIdentity(ctx, u.ID, 0, "wx-fixture", "openid-1"); err != nil {
		t.Fatal(err)
	}
	uc := newWechatUsecase(repo, &fakeWechatClient{codes: map[string]string{"good-code": "openid-1"}}, newFakeBindingStore(), &fakeLimiter{n: 100})

	res, err := uc.Login(ctx, "good-code", "1.2.3.4")
	if err != nil {
		t.Fatalf("Login(bound) error = %v", err)
	}
	if res.Pair == nil || res.User == nil {
		t.Fatalf("Login(bound) = %+v, want a token pair for the bound account", res)
	}
	if res.User.ID != u.ID {
		t.Fatalf("Login(bound) user = %v, want the bound account", res.User.ID)
	}
	if res.BindingTicket != "" {
		t.Fatal("Login(bound) returned a binding ticket")
	}
}

func TestWechatLoginUnboundOpenidReturnsTicket(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	if _, err := repo.Create(ctx, &User{Email: "reader@example.com", PasswordHash: "x", Role: UserRoleReader}); err != nil {
		t.Fatal(err)
	}
	uc := newWechatUsecase(repo, &fakeWechatClient{codes: map[string]string{"new-code": "openid-new"}}, newFakeBindingStore(), &fakeLimiter{n: 100})

	res, err := uc.Login(ctx, "new-code", "1.2.3.4")
	if err != nil {
		t.Fatalf("Login(unbound) error = %v", err)
	}
	if res.Pair != nil || res.User != nil {
		t.Fatalf("Login(unbound) = %+v, want no session", res)
	}
	if res.BindingTicket == "" {
		t.Fatal("Login(unbound) did not return a binding ticket")
	}
}

func TestWechatLoginRejectedCode(t *testing.T) {
	ctx := context.Background()
	uc := newWechatUsecase(newFakeUserRepo(), &fakeWechatClient{codes: map[string]string{}}, newFakeBindingStore(), &fakeLimiter{n: 100})

	if _, err := uc.Login(ctx, "stale-code", "1.2.3.4"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("Login(rejected code) error = %v, want unauthorized", err)
	}
}

func TestWechatLoginRateLimited(t *testing.T) {
	ctx := context.Background()
	uc := newWechatUsecase(newFakeUserRepo(), &fakeWechatClient{codes: map[string]string{"c": "o"}}, newFakeBindingStore(), &fakeLimiter{n: 0})

	if _, err := uc.Login(ctx, "c", "1.2.3.4"); !kratoserrors.IsTooManyRequests(err) {
		t.Fatalf("Login(rate limited) error = %v, want too many requests", err)
	}
}

func TestWechatBindSuccess(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	hash, err := HashPassword("longenough1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, &User{Email: "reader@example.com", PasswordHash: hash, Role: UserRoleReader}); err != nil {
		t.Fatal(err)
	}
	bindings := newFakeBindingStore()
	uc := newWechatUsecase(repo, &fakeWechatClient{codes: map[string]string{"c": "openid-new"}}, bindings, &fakeLimiter{n: 100})

	ticket, err := bindings.Save(ctx, "wx-fixture", "openid-new", DefaultWechatBindingTTL)
	if err != nil {
		t.Fatal(err)
	}
	u, pair, err := uc.Bind(ctx, ticket, "reader@example.com", "longenough1", "1.2.3.4")
	if err != nil {
		t.Fatalf("Bind(ok) error = %v", err)
	}
	if pair == nil {
		t.Fatal("Bind(ok) returned no token pair")
	}
	if bound, err := repo.FindByWechatIdentity(ctx, "wx-fixture", "openid-new"); err != nil || bound.ID != u.ID {
		t.Fatalf("Bind(ok) openid = %q, want it attached", u.WechatOpenID)
	}
	// The consumed ticket must be burned even after success.
	if _, _, err := uc.Bind(ctx, ticket, "reader@example.com", "longenough1", "1.2.3.4"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("Bind(ticket replay) error = %v, want unauthorized", err)
	}
	// And the openid now resolves to the account on the next login.
	res, err := uc.Login(ctx, "c", "1.2.3.4")
	if err != nil || res.Pair == nil {
		t.Fatalf("Login(after bind) = %+v err %v, want a signed-in pair", res, err)
	}
}

func TestWechatBindWrongPasswordBurnsTicket(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	hash, err := HashPassword("longenough1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, &User{Email: "reader@example.com", PasswordHash: hash, Role: UserRoleReader}); err != nil {
		t.Fatal(err)
	}
	bindings := newFakeBindingStore()
	uc := newWechatUsecase(repo, &fakeWechatClient{codes: map[string]string{"c": "openid-new"}}, bindings, &fakeLimiter{n: 100})

	ticket, err := bindings.Save(ctx, "wx-fixture", "openid-new", DefaultWechatBindingTTL)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := uc.Bind(ctx, ticket, "reader@example.com", "wrong-password", "1.2.3.4"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("Bind(wrong password) error = %v, want unauthorized", err)
	}
	// The failed attempt consumed the ticket: retrying with the right
	// password on the same ticket must fail too.
	if _, _, err := uc.Bind(ctx, ticket, "reader@example.com", "longenough1", "1.2.3.4"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("Bind(burned ticket) error = %v, want unauthorized", err)
	}
}

func TestWechatBindConflictWithAnotherAccount(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	hash, err := HashPassword("longenough1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, &User{Email: "a@example.com", PasswordHash: hash, Role: UserRoleReader}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, &User{Email: "b@example.com", PasswordHash: hash, Role: UserRoleReader}); err != nil {
		t.Fatal(err)
	}
	// Account A already carries the openid.
	if err := repo.BindWechatIdentity(ctx, repo.users["a@example.com"].ID, 0, "wx-fixture", "openid-taken"); err != nil {
		t.Fatal(err)
	}
	bindings := newFakeBindingStore()
	uc := newWechatUsecase(repo, &fakeWechatClient{codes: map[string]string{"c": "openid-taken"}}, bindings, &fakeLimiter{n: 100})

	ticket, err := bindings.Save(ctx, "wx-fixture", "openid-taken", DefaultWechatBindingTTL)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = uc.Bind(ctx, ticket, "b@example.com", "longenough1", "1.2.3.4")
	if !kratoserrors.IsConflict(err) {
		t.Fatalf("Bind(conflict) error = %v, want conflict", err)
	}
}

func TestWechatBindEmptyTicket(t *testing.T) {
	ctx := context.Background()
	uc := newWechatUsecase(newFakeUserRepo(), &fakeWechatClient{}, newFakeBindingStore(), &fakeLimiter{n: 100})

	if _, _, err := uc.Bind(ctx, "", "a@example.com", "longenough1", "1.2.3.4"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("Bind(empty ticket) error = %v, want unauthorized", err)
	}
}

func TestWechatBindDisabledMustNotConsumeOrBind(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	users := NewUserUsecase(repo)
	if _, err := users.CreateAccount(ctx, "disabled@example.com", "longenough1", "Fixture", UserRoleReader); err != nil {
		t.Fatal(err)
	}
	store := newFakeBindingStore()
	ticket, err := store.Save(ctx, "wx-fixture", "openid-disabled", DefaultWechatBindingTTL)
	if err != nil {
		t.Fatal(err)
	}
	uc := newWechatUsecase(repo, &fakeWechatClient{err: ErrWechatNotConfigured}, store, nil)
	if _, _, err := uc.Bind(ctx, ticket, "disabled@example.com", "longenough1", "local"); !kratoserrors.Is(err, ErrWechatNotConfigured) {
		t.Fatalf("disabled bind must reject before consuming: %v", err)
	}
	if _, err := store.Consume(ctx, "wx-fixture", ticket); err != nil {
		t.Fatal("disabled bind burned ticket")
	}
}

func TestWechatLoginIsolatesAppAndIgnoresLegacyColumn(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	u, err := repo.Create(ctx, &User{Email: "isolation@example.invalid", PasswordHash: "fixture", Role: UserRoleReader, WechatOpenID: "same-openid"})
	if err != nil {
		t.Fatal(err)
	}
	old := &fakeWechatClient{appID: "wx-old", codes: map[string]string{"code": "same-openid"}}
	formal := &fakeWechatClient{appID: "wx-formal", codes: map[string]string{"code": "same-openid"}}
	if err := repo.BindWechatIdentity(ctx, u.ID, 0, old.AppID(), "same-openid"); err != nil {
		t.Fatal(err)
	}
	for _, client := range []*fakeWechatClient{formal, old} {
		result, err := newWechatUsecase(repo, client, newFakeBindingStore(), nil).Login(ctx, "code", "local")
		if err != nil {
			t.Fatal(err)
		}
		if client == formal && (result.User != nil || result.Pair != nil || result.BindingTicket == "") {
			t.Fatal("formal app reused old or bare identity")
		}
		if client == old && (result.User == nil || result.User.ID != u.ID || result.Pair == nil) {
			t.Fatal("old AppID identity no longer resolves")
		}
	}
	if u.WechatOpenID != "same-openid" {
		t.Fatal("legacy column changed")
	}
}

type resetBeforeIdentityBind struct{ UserRepository }

func (r resetBeforeIdentityBind) BindWechatIdentity(ctx context.Context, id uuid.UUID, version uint64, appID, openid string) error {
	if _, ok, err := r.UpdatePasswordState(ctx, id, version, "fixture-new-hash", false); err != nil || !ok {
		return ErrUserInvalidCredentials
	}
	return r.UserRepository.BindWechatIdentity(ctx, id, version, appID, openid)
}
func TestWechatBindRejectsPasswordResetAfterAuthentication(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	users := NewUserUsecase(resetBeforeIdentityBind{repo})
	u, err := users.CreateAccount(ctx, "reset-race@example.invalid", "longenough1", "Fixture", UserRoleReader)
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeBindingStore()
	ticket, err := store.Save(ctx, "wx-fixture", "fresh-openid", DefaultWechatBindingTTL)
	if err != nil {
		t.Fatal(err)
	}
	sessions := newFakeSessionRepo()
	auth := NewAuthUsecase(users, sessions, newFakeIssuer(), nil, nil, 0)
	uc := NewWechatUsecase(users, auth, &fakeWechatClient{}, store, nil)
	if _, pair, err := uc.Bind(ctx, ticket, u.Email, "longenough1", "local"); !kratoserrors.IsUnauthorized(err) || pair != nil {
		t.Fatal("stale password result created identity/session")
	}
	if len(repo.identities) != 0 || len(sessions.sessions) != 0 {
		t.Fatal("failed bind left identity or session")
	}
}
