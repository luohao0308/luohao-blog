package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

// fakeWechatClient maps codes to openids or a fixed error, standing in for
// the jscode2session round trip.
type fakeWechatClient struct {
	codes map[string]string
	err   error
}

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

func (f *fakeBindingStore) Save(_ context.Context, openid string, _ time.Duration) (string, error) {
	f.next++
	ticket := "ticket-" + strings.Repeat("t", f.next)
	f.tickets[ticket] = openid
	return ticket, nil
}

func (f *fakeBindingStore) Consume(_ context.Context, ticket string) (string, error) {
	openid, ok := f.tickets[ticket]
	if !ok {
		return "", ErrWechatTicketInvalid
	}
	delete(f.tickets, ticket)
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
	u.WechatOpenID = "openid-1"
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

	ticket, err := bindings.Save(ctx, "openid-new", DefaultWechatBindingTTL)
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
	if u.WechatOpenID != "openid-new" {
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

	ticket, err := bindings.Save(ctx, "openid-new", DefaultWechatBindingTTL)
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
	if err := repo.BindWechat(ctx, repo.users["a@example.com"].ID, "openid-taken"); err != nil {
		t.Fatal(err)
	}
	bindings := newFakeBindingStore()
	uc := newWechatUsecase(repo, &fakeWechatClient{codes: map[string]string{"c": "openid-taken"}}, bindings, &fakeLimiter{n: 100})

	ticket, err := bindings.Save(ctx, "openid-taken", DefaultWechatBindingTTL)
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
