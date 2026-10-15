package data

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/accountemailtoken"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/user"

	"github.com/google/uuid"
)

type accountEmailTokenRepo struct{ data *Data }

func NewAccountEmailTokenRepo(data *Data) biz.AccountEmailTokenRepo {
	return &accountEmailTokenRepo{data: data}
}

func emailTokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (r *accountEmailTokenRepo) Issue(ctx context.Context, purpose biz.EmailPurpose, userID uuid.UUID, version uint64, ttl time.Duration) (string, error) {
	raw, err := newOpaqueToken()
	if err != nil {
		return "", err
	}
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return "", err
	}
	current, err := tx.User.Query().Where(user.IDEQ(userID)).Only(ctx)
	if err != nil || current.CredentialVersion != version {
		_ = tx.Rollback()
		if err != nil {
			if ent.IsNotFound(err) {
				return "", biz.ErrEmailTokenInvalid
			}
			return "", err
		}
		return "", biz.ErrEmailTokenInvalid
	}
	if _, err = tx.AccountEmailToken.Delete().Where(accountemailtoken.UserIDEQ(userID), accountemailtoken.PurposeEQ(string(purpose))).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return "", err
	}
	_, err = tx.AccountEmailToken.Create().SetUserID(userID).SetPurpose(string(purpose)).SetTokenHash(emailTokenHash(raw)).SetCredentialVersion(version).SetExpiresAt(time.Now().Add(ttl)).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return raw, nil
}

func (r *accountEmailTokenRepo) IsCurrent(ctx context.Context, purpose biz.EmailPurpose, userID uuid.UUID, version uint64, raw string) (bool, error) {
	n, err := r.data.db.AccountEmailToken.Query().Where(accountemailtoken.UserIDEQ(userID), accountemailtoken.PurposeEQ(string(purpose)), accountemailtoken.TokenHashEQ(emailTokenHash(raw)), accountemailtoken.CredentialVersionEQ(version), accountemailtoken.ExpiresAtGT(time.Now())).Count(ctx)
	if err != nil {
		return false, err
	}
	if n != 1 {
		return false, nil
	}
	users, err := r.data.db.User.Query().Where(user.IDEQ(userID), user.CredentialVersionEQ(version)).Count(ctx)
	return users == 1, err
}

func (r *accountEmailTokenRepo) Validate(ctx context.Context, purpose biz.EmailPurpose, raw string) (*biz.EmailTokenClaims, bool, error) {
	po, err := r.data.db.AccountEmailToken.Query().Where(accountemailtoken.PurposeEQ(string(purpose)), accountemailtoken.TokenHashEQ(emailTokenHash(raw)), accountemailtoken.ExpiresAtGT(time.Now())).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	n, err := r.data.db.User.Query().Where(user.IDEQ(po.UserID), user.CredentialVersionEQ(po.CredentialVersion)).Count(ctx)
	if err != nil || n != 1 {
		return nil, false, err
	}
	return &biz.EmailTokenClaims{Purpose: purpose, UserID: po.UserID, CredentialVersion: po.CredentialVersion}, true, nil
}

func (r *accountEmailTokenRepo) Revoke(ctx context.Context, purpose biz.EmailPurpose, userID uuid.UUID, raw string) error {
	_, err := r.data.db.AccountEmailToken.Delete().Where(accountemailtoken.UserIDEQ(userID), accountemailtoken.PurposeEQ(string(purpose)), accountemailtoken.TokenHashEQ(emailTokenHash(raw))).Exec(ctx)
	return err
}

func (r *accountEmailTokenRepo) ConfirmVerification(ctx context.Context, raw string) (uuid.UUID, bool, error) {
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	po, err := tx.AccountEmailToken.Query().Where(accountemailtoken.PurposeEQ(string(biz.EmailPurposeVerify)), accountemailtoken.TokenHashEQ(emailTokenHash(raw))).Only(ctx)
	if ent.IsNotFound(err) {
		_ = tx.Rollback()
		return uuid.Nil, false, nil
	}
	if err != nil {
		_ = tx.Rollback()
		return uuid.Nil, false, err
	}
	affected, err := tx.AccountEmailToken.Delete().Where(accountemailtoken.IDEQ(po.ID), accountemailtoken.TokenHashEQ(po.TokenHash)).Exec(ctx)
	if err != nil || affected != 1 {
		_ = tx.Rollback()
		if err != nil {
			return uuid.Nil, false, err
		}
		return uuid.Nil, false, nil
	}
	if !po.ExpiresAt.After(time.Now()) {
		if err = tx.Commit(); err != nil {
			return uuid.Nil, false, err
		}
		return uuid.Nil, false, nil
	}
	current, err := tx.User.Query().Where(user.IDEQ(po.UserID), user.CredentialVersionEQ(po.CredentialVersion)).Only(ctx)
	if ent.IsNotFound(err) {
		if err = tx.Commit(); err != nil {
			return uuid.Nil, false, err
		}
		return uuid.Nil, false, nil
	}
	if err != nil {
		_ = tx.Rollback()
		return uuid.Nil, false, err
	}
	if !current.EmailVerified {
		affected, err = tx.User.Update().Where(user.IDEQ(po.UserID), user.CredentialVersionEQ(po.CredentialVersion), user.EmailVerifiedEQ(false)).SetEmailVerified(true).Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			return uuid.Nil, false, err
		}
		if affected != 1 {
			if err = tx.Commit(); err != nil {
				return uuid.Nil, false, err
			}
			return uuid.Nil, false, nil
		}
	}
	if err = tx.Commit(); err != nil {
		return uuid.Nil, false, err
	}
	return po.UserID, true, nil
}

func (r *accountEmailTokenRepo) ConfirmPasswordReset(ctx context.Context, raw, passwordHash string) (uuid.UUID, uint64, bool, error) {
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return uuid.Nil, 0, false, err
	}
	po, err := tx.AccountEmailToken.Query().Where(accountemailtoken.PurposeEQ(string(biz.EmailPurposeReset)), accountemailtoken.TokenHashEQ(emailTokenHash(raw))).Only(ctx)
	if ent.IsNotFound(err) {
		_ = tx.Rollback()
		return uuid.Nil, 0, false, nil
	}
	if err != nil {
		_ = tx.Rollback()
		return uuid.Nil, 0, false, err
	}
	affected, err := tx.AccountEmailToken.Delete().Where(accountemailtoken.IDEQ(po.ID), accountemailtoken.TokenHashEQ(po.TokenHash)).Exec(ctx)
	if err != nil || affected != 1 {
		_ = tx.Rollback()
		if err != nil {
			return uuid.Nil, 0, false, err
		}
		return uuid.Nil, 0, false, nil
	}
	if !po.ExpiresAt.After(time.Now()) {
		if err = tx.Commit(); err != nil {
			return uuid.Nil, 0, false, err
		}
		return uuid.Nil, 0, false, nil
	}
	affected, err = tx.User.Update().Where(user.IDEQ(po.UserID), user.CredentialVersionEQ(po.CredentialVersion)).SetPasswordHash(passwordHash).SetEmailVerified(true).AddCredentialVersion(1).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return uuid.Nil, 0, false, err
	}
	if affected != 1 {
		if err = tx.Commit(); err != nil {
			return uuid.Nil, 0, false, err
		}
		return uuid.Nil, 0, false, nil
	}
	if _, err = tx.AccountEmailToken.Delete().Where(accountemailtoken.UserIDEQ(po.UserID)).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return uuid.Nil, 0, false, err
	}
	if err = tx.Commit(); err != nil {
		return uuid.Nil, 0, false, err
	}
	return po.UserID, po.CredentialVersion + 1, true, nil
}

func (r *accountEmailTokenRepo) RevokeUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.data.db.AccountEmailToken.Delete().Where(accountemailtoken.UserIDEQ(userID)).Exec(ctx)
	return err
}
