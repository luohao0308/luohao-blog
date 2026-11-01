package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

func TestMiniContentIdentityBindsCodeToAuthenticatedAccount(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	owner, _ := repo.Create(ctx, &User{Email: "owner@example.invalid", Role: UserRoleReader})
	other, _ := repo.Create(ctx, &User{Email: "other@example.invalid", Role: UserRoleReader})
	if err := repo.BindWechatIdentity(ctx, owner.ID, 0, "wx-fixture", "owner-openid"); err != nil {
		t.Fatal(err)
	}
	if err := repo.BindWechatIdentity(ctx, other.ID, 0, "wx-fixture", "other-openid"); err != nil {
		t.Fatal(err)
	}
	client := &fakeWechatClient{codes: map[string]string{"owner-code": "owner-openid", "other-code": "other-openid", "unbound-code": "unbound-openid"}}
	uc := NewMiniContentIdentityUsecase(NewUserUsecase(repo), client)
	claims := &Claims{UserID: owner.ID, Role: UserRoleReader}
	openid, err := uc.Resolve(ctx, claims, "owner-code")
	if err != nil || openid != "owner-openid" || uc.AppID() != "wx-fixture" {
		t.Fatalf("Resolve own code: openid=%q err=%v", openid, err)
	}
	for _, tc := range []struct {
		name   string
		code   string
		claims *Claims
		status int32
	}{
		{"missing claims", "owner-code", nil, 401},
		{"missing user", "owner-code", &Claims{UserID: uuid.Nil}, 401},
		{"missing code", "", claims, 412},
		{"invalid code cannot trigger POST auth replay", "invalid-code", claims, 412},
		{"another account code", "other-code", claims, 403},
		{"unbound identity is not created", "unbound-code", claims, 412},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := uc.Resolve(ctx, tc.claims, tc.code)
			if value != "" || err == nil || errors.FromError(err).Code != tc.status {
				t.Fatalf("unexpected identity result %q / %v", value, err)
			}
		})
	}
	if _, err := repo.FindByWechatIdentity(ctx, "wx-fixture", "unbound-openid"); !errors.Is(err, ErrUserNotFound) {
		t.Fatal("content identity resolution must not create bindings")
	}
}

func TestMiniContentIdentityDoesNotFallbackAcrossAppIDs(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	owner, _ := repo.Create(ctx, &User{Email: "owner@example.invalid", Role: UserRoleReader})
	if err := repo.BindWechatIdentity(ctx, owner.ID, 0, "wx-legacy", "same-openid"); err != nil {
		t.Fatal(err)
	}
	client := &fakeWechatClient{appID: "wx-formal", codes: map[string]string{"code": "same-openid"}}
	uc := NewMiniContentIdentityUsecase(NewUserUsecase(repo), client)
	if value, err := uc.Resolve(ctx, &Claims{UserID: owner.ID}, "code"); value != "" || !errors.Is(err, ErrContentSafetyIdentity) {
		t.Fatalf("legacy identity accepted: %q / %v", value, err)
	}
	client.err = ErrWechatNotConfigured
	if _, err := uc.Resolve(ctx, &Claims{UserID: owner.ID}, "code"); !errors.Is(err, ErrContentSafetyDisabled) {
		t.Fatalf("disabled upstream: %v", err)
	}
	client.err = errors.InternalServer("FIXTURE", "sensitive upstream fixture details")
	if _, err := uc.Resolve(ctx, &Claims{UserID: owner.ID}, "code"); !errors.Is(err, ErrContentSafetyUnavailable) {
		t.Fatalf("upstream error must be replaced with fixed error: %v", err)
	}
}
