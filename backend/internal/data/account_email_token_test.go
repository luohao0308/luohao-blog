package data

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/accountemailtoken"
)

func TestAccountEmailTokenRepoIssueReplaceAndVerify(t *testing.T) {
	ctx := context.Background()
	_, _, data := newTestRepo(t)
	users := biz.NewUserUsecase(NewUserRepo(data))
	u, err := users.CreateAccount(ctx, "verify@example.com", "longenough1", "Verify", biz.UserRoleReader)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewAccountEmailTokenRepo(data)
	first, err := repo.Issue(ctx, biz.EmailPurposeVerify, u.ID, 0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	row, err := data.db.AccountEmailToken.Query().Only(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row.TokenHash == first || len(row.TokenHash) != 64 {
		t.Fatal("raw token persisted")
	}
	second, err := repo.Issue(ctx, biz.EmailPurposeVerify, u.ID, 0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if current, _ := repo.IsCurrent(ctx, biz.EmailPurposeVerify, u.ID, 0, first); current {
		t.Fatal("reissued old token remains current")
	}
	if current, _ := repo.IsCurrent(ctx, biz.EmailPurposeVerify, u.ID, 0, second); !current {
		t.Fatal("new token not current")
	}
	if err := repo.Revoke(ctx, biz.EmailPurposeVerify, u.ID, first); err != nil {
		t.Fatal(err)
	}
	if current, _ := repo.IsCurrent(ctx, biz.EmailPurposeVerify, u.ID, 0, second); !current {
		t.Fatal("failed old delivery revoked newer token")
	}
	if _, ok, err := repo.ConfirmVerification(ctx, second); err != nil || !ok {
		t.Fatalf("confirm = %v/%v", ok, err)
	}
	got, _ := users.ByID(ctx, u.ID)
	if !got.EmailVerified {
		t.Fatal("verification flag not set")
	}
	if _, ok, _ := repo.ConfirmVerification(ctx, second); ok {
		t.Fatal("verification token replayed")
	}
}

func TestAccountEmailResetRollbackRestoresToken(t *testing.T) {
	ctx := context.Background()
	_, _, data := newTestRepo(t)
	users := biz.NewUserUsecase(NewUserRepo(data))
	u, _ := users.CreateAccount(ctx, "reset@example.com", "longenough1", "Reset", biz.UserRoleReader)
	repo := NewAccountEmailTokenRepo(data)
	raw, err := repo.Issue(ctx, biz.EmailPurposeReset, u.ID, 0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	fail := true
	data.db.User.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			if m, ok := mutation.(*ent.UserMutation); ok {
				if _, exists := m.PasswordHash(); exists && fail {
					return nil, errors.New("forced update failure")
				}
			}
			return next.Mutate(ctx, mutation)
		})
	})
	if _, _, _, err := repo.ConfirmPasswordReset(ctx, raw, "hash"); err == nil {
		t.Fatal("forced DB failure hidden")
	}
	fail = false
	uid, version, ok, err := repo.ConfirmPasswordReset(ctx, raw, "hash")
	if err != nil || !ok || uid != u.ID || version != 1 {
		t.Fatalf("retry = %s/%d/%v/%v", uid, version, ok, err)
	}
	if count, _ := data.db.AccountEmailToken.Query().Where(accountemailtoken.UserIDEQ(u.ID)).Count(ctx); count != 0 {
		t.Fatalf("tokens remain = %d", count)
	}
}

func TestAccountEmailTokenPurposeExpiryAndVersion(t *testing.T) {
	ctx := context.Background()
	_, _, data := newTestRepo(t)
	users := biz.NewUserUsecase(NewUserRepo(data))
	u, _ := users.CreateAccount(ctx, "bounds@example.com", "longenough1", "Bounds", biz.UserRoleReader)
	repo := NewAccountEmailTokenRepo(data)
	raw, _ := repo.Issue(ctx, biz.EmailPurposeReset, u.ID, 0, time.Hour)
	if _, ok, _ := repo.ConfirmVerification(ctx, raw); ok {
		t.Fatal("reset token accepted for verification")
	}
	if _, err := data.db.AccountEmailToken.Update().Where(accountemailtoken.UserIDEQ(u.ID)).SetExpiresAt(time.Now().Add(-time.Minute)).Save(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := repo.ConfirmPasswordReset(ctx, raw, "hash"); err != nil || ok {
		t.Fatalf("expired reset = %v/%v", ok, err)
	}
	current, _ := users.ByID(ctx, u.ID)
	if _, updated, err := NewUserRepo(data).UpdatePasswordState(ctx, u.ID, current.CredentialVersion, current.PasswordHash, false); err != nil || !updated {
		t.Fatal(err)
	}
	if _, err := repo.Issue(ctx, biz.EmailPurposeVerify, u.ID, 0, time.Hour); err == nil {
		t.Fatal("stale version issued token")
	}
}
