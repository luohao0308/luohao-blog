package data

import (
	"context"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/user"
)

// userToBiz converts a persisted user into its domain representation. The
// role column is bound to biz.UserRole, so it carries over as-is.
func userToBiz(po *ent.User) *biz.User {
	if po == nil {
		return nil
	}
	return &biz.User{
		ID:           po.ID,
		Email:        po.Email,
		PasswordHash: po.PasswordHash,
		DisplayName:  po.DisplayName,
		Role:         po.Role,
		CreatedAt:    po.CreatedAt,
		UpdatedAt:    po.UpdatedAt,
	}
}

type userRepo struct {
	data *Data
}

// NewUserRepo creates a new UserRepository instance.
func NewUserRepo(data *Data) biz.UserRepository {
	return &userRepo{data: data}
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*biz.User, error) {
	po, err := r.data.db.User.Query().
		Where(user.EmailEQ(email)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrUserNotFound
		}
		return nil, err
	}
	return userToBiz(po), nil
}

func (r *userRepo) Create(ctx context.Context, u *biz.User) (*biz.User, error) {
	po, err := r.data.db.User.Create().
		SetEmail(u.Email).
		SetPasswordHash(u.PasswordHash).
		SetDisplayName(u.DisplayName).
		SetRole(u.Role).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, biz.ErrUserEmailConflict
		}
		return nil, err
	}
	return userToBiz(po), nil
}
