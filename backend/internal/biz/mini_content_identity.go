package biz

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

// MiniContentIdentityUsecase proves that a fresh mini-program code belongs to
// the authenticated website account. It never creates or changes a binding.
type MiniContentIdentityUsecase struct {
	users  *UserUsecase
	wechat WechatClient
}

func NewMiniContentIdentityUsecase(users *UserUsecase, wechat WechatClient) *MiniContentIdentityUsecase {
	return &MiniContentIdentityUsecase{users: users, wechat: wechat}
}

func (uc *MiniContentIdentityUsecase) AppID() string {
	if uc == nil || uc.wechat == nil {
		return ""
	}
	return uc.wechat.AppID()
}

func (uc *MiniContentIdentityUsecase) Resolve(ctx context.Context, claims *Claims, code string) (string, error) {
	if claims == nil || claims.UserID == uuid.Nil {
		return "", ErrAuthUnauthorized
	}
	if uc == nil || uc.users == nil || uc.wechat == nil || !uc.wechat.Configured() || uc.AppID() == "" {
		return "", ErrContentSafetyDisabled
	}
	if strings.TrimSpace(code) == "" || len(code) > 256 {
		return "", ErrContentSafetyIdentity
	}
	openid, err := uc.wechat.Code2Session(ctx, code)
	if errors.Is(err, ErrWechatCodeInvalid) {
		// A code is single-use. A 401 here would make the mini request layer
		// rotate JWTs and replay a POST with the already-consumed code.
		return "", ErrContentSafetyIdentity
	}
	if err != nil {
		return "", ErrContentSafetyUnavailable
	}
	if openid == "" {
		return "", ErrContentSafetyIdentity
	}
	owner, err := uc.users.ByWechatIdentity(ctx, uc.AppID(), openid)
	if errors.Is(err, ErrUserNotFound) {
		return "", ErrContentSafetyIdentity
	}
	if err != nil || owner == nil {
		return "", ErrContentSafetyUnavailable
	}
	if owner.ID != claims.UserID {
		return "", ErrAuthForbidden
	}
	return openid, nil
}
