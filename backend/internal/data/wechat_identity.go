package data

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/user"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatidentity"
)

func (r *userRepo) FindByWechatIdentity(ctx context.Context, appID, openid string) (*biz.User, error) {
	if appID == "" || openid == "" {
		return nil, biz.ErrUserNotFound
	}
	po, err := r.data.db.WechatIdentity.Query().Where(wechatidentity.AppIDEQ(appID), wechatidentity.OpenidEQ(openid)).QueryUser().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, biz.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return userToBiz(po), nil
}

// BindWechatIdentity checks the password's version while holding the same user
// row lock used by password updates. Identities are insert-only.
func (r *userRepo) BindWechatIdentity(ctx context.Context, id uuid.UUID, expectedVersion uint64, appID, openid string) error {
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	po, err := tx.User.Query().Where(user.IDEQ(id)).ForUpdate().Only(ctx)
	if ent.IsNotFound(err) {
		return biz.ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if po.CredentialVersion != expectedVersion {
		return biz.ErrUserInvalidCredentials
	}
	if _, err = insertWechatIdentity(ctx, tx, id, appID, openid); err != nil {
		return err
	}
	return tx.Commit()
}

// insertWechatIdentity requires its caller to hold the target user's row lock.
func insertWechatIdentity(ctx context.Context, tx *ent.Tx, id uuid.UUID, appID, openid string) (bool, error) {
	if appID == "" || len(appID) > 32 || openid == "" || len(openid) > 64 {
		return false, biz.ErrUserWechatConflict
	}
	existing, err := tx.WechatIdentity.Query().Where(wechatidentity.UserIDEQ(id), wechatidentity.AppIDEQ(appID)).Only(ctx)
	if err == nil {
		if existing.Openid == openid {
			return false, nil
		}
		return false, biz.ErrUserWechatConflict
	}
	if !ent.IsNotFound(err) {
		return false, err
	}
	_, err = tx.WechatIdentity.Create().SetUserID(id).SetAppID(appID).SetOpenid(openid).Save(ctx)
	if ent.IsConstraintError(err) {
		return false, biz.ErrUserWechatConflict
	}
	return err == nil, err
}

// WechatBackfillReport deliberately contains counts only, never OpenIDs/users.
type WechatBackfillReport struct {
	Legacy, Existing, Inserted int
	Applied                    bool
}

// ErrWechatBackfillCommitUnknown means the database may have committed even
// though its acknowledgment was lost. Keep writers paused and recheck first.
var ErrWechatBackfillCommitUnknown = errors.New("wechat identity backfill commit outcome unknown")

var legacyAppIDPattern = regexp.MustCompile(`^wx[0-9a-f]{16}$`)

// BackfillWechatIdentities defaults to a rolled-back dry run. The caller must
// pause all identity writers before apply. It never runs schema migrations.
func BackfillWechatIdentities(ctx context.Context, client *ent.Client, legacyAppID string, apply, writersPaused bool) (WechatBackfillReport, error) {
	var report WechatBackfillReport
	if !legacyAppIDPattern.MatchString(legacyAppID) {
		return report, errors.New("explicit valid legacy AppID required")
	}
	if apply && !writersPaused {
		return report, errors.New("identity writers must be paused before apply")
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return report, err
	}
	defer func() { _ = tx.Rollback() }()
	// Lock all existing legacy holders. Writers must also be paused to prevent
	// newly-created holders or old binaries changing a previously empty column.
	holders, err := tx.User.Query().Where(user.WechatOpenidNotNil(), user.WechatOpenidNEQ("")).Order(ent.Asc(user.FieldID)).ForUpdate().All(ctx)
	if err != nil {
		return report, err
	}
	report.Legacy = len(holders)
	for _, holder := range holders {
		inserted, e := insertWechatIdentity(ctx, tx, holder.ID, legacyAppID, holder.WechatOpenid)
		if e != nil {
			return report, fmt.Errorf("legacy identity conflict or storage failure: %w", e)
		}
		if inserted {
			report.Inserted++
		} else {
			report.Existing++
		}
	}
	// Verify coverage and the exact legacy values while locks are still held.
	for _, holder := range holders {
		current, e := tx.User.Get(ctx, holder.ID)
		if e != nil {
			return report, e
		}
		if current.WechatOpenid != holder.WechatOpenid {
			return report, errors.New("legacy identity changed during backfill")
		}
		count, e := tx.WechatIdentity.Query().Where(wechatidentity.UserIDEQ(holder.ID), wechatidentity.AppIDEQ(legacyAppID), wechatidentity.OpenidEQ(holder.WechatOpenid)).Count(ctx)
		if e != nil {
			return report, e
		}
		if count != 1 {
			return report, errors.New("legacy identity coverage mismatch")
		}
	}
	if apply {
		if err = tx.Commit(); err != nil {
			return report, ErrWechatBackfillCommitUnknown
		}
		report.Applied = true
	} else if err = tx.Rollback(); err != nil {
		return report, err
	}
	return report, nil
}
