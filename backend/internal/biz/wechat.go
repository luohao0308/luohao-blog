package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
)

var (
	// ErrWechatNotConfigured is returned when the server has no mini-program
	// credentials: the endpoints are wired but disabled.
	ErrWechatNotConfigured = errors.New(412, v1.ErrorReason_AUTH_WECHAT_CODE_INVALID.String(), "wechat login is not configured")
	// ErrWechatCodeInvalid is returned when WeChat rejects the login code —
	// invalid, expired, or replayed. The client must run wx.login again.
	ErrWechatCodeInvalid = errors.Unauthorized(v1.ErrorReason_AUTH_WECHAT_CODE_INVALID.String(), "wechat code invalid or expired")
	// ErrWechatTicketInvalid is returned for a binding ticket that is missing,
	// expired, or already consumed. The client must run wx.login again.
	ErrWechatTicketInvalid = errors.Unauthorized(v1.ErrorReason_AUTH_WECHAT_TICKET_INVALID.String(), "binding ticket invalid or expired")
	// ErrWechatConflict is returned when the openid being bound is already
	// attached to another account.
	ErrWechatConflict = errors.Conflict(v1.ErrorReason_USER_WECHAT_CONFLICT.String(), "openid already bound to another account")
)

// DefaultWechatBindingTTL bounds how long a WechatLogin binding ticket stays
// redeemable. Long enough for a user to type email and password; short enough
// that a leaked ticket is worthless quickly.
const DefaultWechatBindingTTL = 10 * time.Minute

// WechatClient exchanges a mini-program login code for the caller's stable
// openid. The implementation lives in the data layer next to the other
// outbound HTTP clients.
type WechatClient interface {
	// Code2Session validates the code and returns the openid. Config-missing
	// setups return ErrWechatNotConfigured; WeChat-side rejections return
	// ErrWechatCodeInvalid.
	Code2Session(ctx context.Context, code string) (string, error)
}

// WechatBindingStore bridges an unbound openid to the BindWechat call that
// follows, addressed by a single-use random ticket.
type WechatBindingStore interface {
	// Save issues a fresh ticket for the openid; the ticket expires after ttl.
	Save(ctx context.Context, openid string, ttl time.Duration) (string, error)
	// Consume atomically redeems the ticket and returns the openid. Unknown
	// or replayed tickets return ErrWechatTicketInvalid.
	Consume(ctx context.Context, ticket string) (string, error)
}

// WechatLoginResult is the outcome of WechatUsecase.Login: either a signed-in
// account with its token pair, or a binding ticket for an unbound openid.
type WechatLoginResult struct {
	User *User
	Pair *TokenPair
	// BindingTicket is set only when User and Pair are nil.
	BindingTicket string
}

// WechatUsecase implements the mini-program login flow: code → openid →
// existing account (sign in) or binding ticket (attach to an existing
// account via email+password). It reuses the auth usecase's token issuance
// so every surface shares one session model.
type WechatUsecase struct {
	users      *UserUsecase
	auth       *AuthUsecase
	wechat     WechatClient
	bindings   WechatBindingStore
	limiter    RateLimiter
	bindingTTL time.Duration
}

// NewWechatUsecase new a Wechat login usecase.
func NewWechatUsecase(users *UserUsecase, auth *AuthUsecase, wechat WechatClient, bindings WechatBindingStore, limiter RateLimiter) *WechatUsecase {
	return &WechatUsecase{users: users, auth: auth, wechat: wechat, bindings: bindings, limiter: limiter, bindingTTL: DefaultWechatBindingTTL}
}

// Login exchanges a wx.login code for either a signed-in session or a
// binding ticket. Throttling shares the login budget: both endpoints are
// anonymous and WeChat-side rate limits make hammering pointless anyway.
func (uc *WechatUsecase) Login(ctx context.Context, code, clientIP string) (*WechatLoginResult, error) {
	if uc.limiter != nil {
		ok, err := uc.limiter.Allow(ctx, "login:"+clientIP)
		if err != nil {
			log.Warn("wechat: rate limiter unavailable, failing open", "err", err)
		} else if !ok {
			return nil, ErrAuthTooManyAttempts
		}
	}
	openid, err := uc.wechat.Code2Session(ctx, code)
	if err != nil {
		return nil, err
	}
	u, err := uc.users.ByWechatOpenID(ctx, openid)
	if err == nil {
		pair, err := uc.auth.issuePair(ctx, u)
		if err != nil {
			return nil, err
		}
		return &WechatLoginResult{User: u, Pair: pair}, nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}
	ticket, err := uc.bindings.Save(ctx, openid, uc.bindingTTL)
	if err != nil {
		return nil, err
	}
	return &WechatLoginResult{BindingTicket: ticket}, nil
}

// Bind attaches the ticket's openid to an existing account authenticated by
// email and password, then signs the caller in. The ticket is consumed first:
// a wrong password burns it, so each bind attempt needs a fresh wx.login —
// deliberate, it keeps a captured ticket from being re-armed after a failure.
func (uc *WechatUsecase) Bind(ctx context.Context, ticket, email, password, clientIP string) (*User, *TokenPair, error) {
	if uc.limiter != nil {
		ok, err := uc.limiter.Allow(ctx, "login:"+clientIP)
		if err != nil {
			log.Warn("wechat: rate limiter unavailable, failing open", "err", err)
		} else if !ok {
			return nil, nil, ErrAuthTooManyAttempts
		}
	}
	if ticket == "" {
		return nil, nil, ErrWechatTicketInvalid
	}
	openid, err := uc.bindings.Consume(ctx, ticket)
	if err != nil {
		return nil, nil, err
	}
	u, err := uc.users.Authenticate(ctx, email, password)
	if err != nil {
		return nil, nil, err
	}
	if err := uc.users.BindWechat(ctx, u.ID, openid); err != nil {
		return nil, nil, err
	}
	pair, err := uc.auth.issuePair(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}
