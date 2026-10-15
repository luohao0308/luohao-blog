package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sync"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/uuid"
)

type EmailPurpose string

const (
	EmailPurposeVerify                EmailPurpose = "verify"
	EmailPurposeReset                 EmailPurpose = "reset"
	DefaultVerificationTTL                         = 24 * time.Hour
	DefaultPasswordResetTTL                        = 15 * time.Minute
	DefaultEmailCooldown                           = time.Minute
	DefaultEmailHourlyWindow                       = time.Hour
	DefaultVerificationHourlyAttempts int64        = 5
	DefaultResetEmailHourlyAttempts   int64        = 3
	DefaultResetIPHourlyAttempts      int64        = 5
	defaultResetQueueSize                          = 64
)

var (
	ErrMailNotConfigured             = errors.ServiceUnavailable(v1.ErrorReason_AUTH_MAIL_NOT_CONFIGURED.String(), "account email is not configured")
	ErrEmailVerificationTokenInvalid = errors.BadRequest(v1.ErrorReason_AUTH_EMAIL_VERIFICATION_TOKEN_INVALID.String(), "invalid or expired verification token")
	ErrPasswordResetTokenInvalid     = errors.BadRequest(v1.ErrorReason_AUTH_PASSWORD_RESET_TOKEN_INVALID.String(), "invalid or expired password reset token")
	ErrEmailRequestCooldown          = errors.TooManyRequests(v1.ErrorReason_AUTH_EMAIL_REQUEST_COOLDOWN.String(), "email request cooldown active")
	ErrEmailTooManyAttempts          = errors.TooManyRequests(v1.ErrorReason_AUTH_EMAIL_TOO_MANY_ATTEMPTS.String(), "too many email requests")
	ErrEmailDeliveryFailed           = errors.ServiceUnavailable(v1.ErrorReason_AUTH_EMAIL_DELIVERY_FAILED.String(), "email delivery failed")
	ErrEmailTokenStoreUnavailable    = errors.ServiceUnavailable(v1.ErrorReason_AUTH_EMAIL_TOKEN_STORE_UNAVAILABLE.String(), "account email service unavailable")
	ErrEmailTokenInvalid             = errors.BadRequest(v1.ErrorReason_AUTH_EMAIL_VERIFICATION_TOKEN_INVALID.String(), "invalid email token")
)

type EmailTokenClaims struct {
	Purpose           EmailPurpose
	UserID            uuid.UUID
	CredentialVersion uint64
}

type AccountEmailTokenRepo interface {
	Issue(context.Context, EmailPurpose, uuid.UUID, uint64, time.Duration) (string, error)
	IsCurrent(context.Context, EmailPurpose, uuid.UUID, uint64, string) (bool, error)
	Validate(context.Context, EmailPurpose, string) (*EmailTokenClaims, bool, error)
	Revoke(context.Context, EmailPurpose, uuid.UUID, string) error
	ConfirmVerification(context.Context, string) (uuid.UUID, bool, error)
	ConfirmPasswordReset(context.Context, string, string) (uuid.UUID, uint64, bool, error)
	RevokeUser(context.Context, uuid.UUID) error
}

type EmailRateLimiter interface {
	Allow(context.Context, string, int64, time.Duration) (bool, error)
}
type AccountEmailMessage struct {
	To      string
	Subject string
	Body    string
	Link    string
	Purpose EmailPurpose
}
type AccountMailSender interface {
	Configured() bool
	Send(context.Context, AccountEmailMessage) error
}
type AccountEmailSettings struct {
	SiteOrigin                string
	VerificationTTL, ResetTTL time.Duration
}
type resetEmailJob struct{ email string }

type AccountEmailUsecase struct {
	users            *UserUsecase
	tokens           AccountEmailTokenRepo
	limiter          EmailRateLimiter
	mailer           AccountMailSender
	sessions         SessionRepo
	settings         AccountEmailSettings
	queue            chan resetEmailJob
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	hashPassword     func(string) (string, error)
	resetConfirmGate chan struct{}
}

func NewAccountEmailUsecase(users *UserUsecase, tokens AccountEmailTokenRepo, limiter EmailRateLimiter, mailer AccountMailSender, sessions SessionRepo, settings AccountEmailSettings) *AccountEmailUsecase {
	if settings.VerificationTTL <= 0 {
		settings.VerificationTTL = DefaultVerificationTTL
	}
	if settings.ResetTTL <= 0 {
		settings.ResetTTL = DefaultPasswordResetTTL
	}
	return &AccountEmailUsecase{users: users, tokens: tokens, limiter: limiter, mailer: mailer, sessions: sessions, settings: settings, queue: make(chan resetEmailJob, defaultResetQueueSize), hashPassword: HashPassword, resetConfirmGate: make(chan struct{}, 1)}
}

func (uc *AccountEmailUsecase) Start(ctx context.Context) error {
	workerCtx, cancel := context.WithCancel(ctx)
	uc.cancel = cancel
	uc.wg.Add(1)
	go func() {
		defer uc.wg.Done()
		for {
			select {
			case <-workerCtx.Done():
				return
			case job := <-uc.queue:
				uc.processResetEmail(workerCtx, job.email)
			}
		}
	}()
	return nil
}
func (uc *AccountEmailUsecase) Stop(context.Context) error {
	if uc.cancel != nil {
		uc.cancel()
	}
	uc.wg.Wait()
	return nil
}

func (uc *AccountEmailUsecase) RequestVerification(ctx context.Context, userID uuid.UUID) error {
	if uc.mailer == nil || !uc.mailer.Configured() {
		return ErrMailNotConfigured
	}
	u, err := uc.users.ByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.EmailVerified {
		return nil
	}
	if err = uc.checkLimit(ctx, "verify:cooldown:"+userID.String(), 1, DefaultEmailCooldown, ErrEmailRequestCooldown); err != nil {
		return err
	}
	if err = uc.checkLimit(ctx, "verify:hour:"+userID.String(), DefaultVerificationHourlyAttempts, DefaultEmailHourlyWindow, ErrEmailTooManyAttempts); err != nil {
		return err
	}
	token, err := uc.tokens.Issue(ctx, EmailPurposeVerify, u.ID, u.CredentialVersion, uc.settings.VerificationTTL)
	if err != nil {
		return ErrEmailTokenStoreUnavailable
	}
	current, err := uc.tokens.IsCurrent(ctx, EmailPurposeVerify, u.ID, u.CredentialVersion, token)
	if err != nil || !current {
		_ = uc.tokens.Revoke(ctx, EmailPurposeVerify, u.ID, token)
		return ErrEmailTokenStoreUnavailable
	}
	link := uc.settings.SiteOrigin + "/verify-email#token=" + url.PathEscape(token)
	if err = uc.mailer.Send(ctx, AccountEmailMessage{To: u.Email, Subject: "验证邮箱", Body: "请打开链接验证邮箱：" + link, Link: link, Purpose: EmailPurposeVerify}); err != nil {
		_ = uc.tokens.Revoke(ctx, EmailPurposeVerify, u.ID, token)
		return ErrEmailDeliveryFailed
	}
	return nil
}

func (uc *AccountEmailUsecase) ConfirmVerification(ctx context.Context, token string) error {
	if token == "" {
		return ErrEmailVerificationTokenInvalid
	}
	_, ok, err := uc.tokens.ConfirmVerification(ctx, token)
	if err != nil {
		return ErrEmailTokenStoreUnavailable
	}
	if !ok {
		return ErrEmailVerificationTokenInvalid
	}
	return nil
}

func (uc *AccountEmailUsecase) RequestPasswordReset(ctx context.Context, email, clientIP string) error {
	if uc.mailer == nil || !uc.mailer.Configured() {
		return ErrMailNotConfigured
	}
	email = NormalizeEmail(email)
	if !ValidUserEmail(email) {
		return ErrUserInvalidArgument
	}
	hash := sha256.Sum256([]byte(email))
	key := hex.EncodeToString(hash[:])
	if err := uc.checkLimit(ctx, "reset:cooldown:"+key, 1, DefaultEmailCooldown, ErrEmailRequestCooldown); err != nil {
		return err
	}
	if err := uc.checkLimit(ctx, "reset:email:"+key, DefaultResetEmailHourlyAttempts, DefaultEmailHourlyWindow, ErrEmailTooManyAttempts); err != nil {
		return err
	}
	if err := uc.checkLimit(ctx, "reset:ip:"+clientIP, DefaultResetIPHourlyAttempts, DefaultEmailHourlyWindow, ErrEmailTooManyAttempts); err != nil {
		return err
	}
	select {
	case uc.queue <- resetEmailJob{email: email}:
		return nil
	default:
		return ErrEmailTokenStoreUnavailable
	}
}

func (uc *AccountEmailUsecase) processResetEmail(ctx context.Context, email string) {
	u, err := uc.users.ByEmail(ctx, email)
	if err != nil {
		return
	}
	token, err := uc.tokens.Issue(ctx, EmailPurposeReset, u.ID, u.CredentialVersion, uc.settings.ResetTTL)
	if err != nil {
		log.Warn("account email: reset token issue failed")
		return
	}
	current, err := uc.tokens.IsCurrent(ctx, EmailPurposeReset, u.ID, u.CredentialVersion, token)
	if err != nil || !current {
		_ = uc.tokens.Revoke(ctx, EmailPurposeReset, u.ID, token)
		return
	}
	link := uc.settings.SiteOrigin + "/reset-password#token=" + url.PathEscape(token)
	if err = uc.mailer.Send(ctx, AccountEmailMessage{To: u.Email, Subject: "重置密码", Body: "请打开链接重置密码：" + link, Link: link, Purpose: EmailPurposeReset}); err != nil {
		_ = uc.tokens.Revoke(ctx, EmailPurposeReset, u.ID, token)
		log.Warn("account email: reset delivery failed")
	}
}

func (uc *AccountEmailUsecase) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	if token == "" {
		return ErrPasswordResetTokenInvalid
	}
	if !ValidNewPassword(newPassword) {
		return ErrUserInvalidArgument
	}
	select {
	case <-ctx.Done():
		return ErrEmailTokenStoreUnavailable
	default:
	}
	select {
	case uc.resetConfirmGate <- struct{}{}:
		defer func() { <-uc.resetConfirmGate }()
	default:
		return ErrEmailTooManyAttempts
	}
	if _, ok, err := uc.tokens.Validate(ctx, EmailPurposeReset, token); err != nil {
		return ErrEmailTokenStoreUnavailable
	} else if !ok {
		return ErrPasswordResetTokenInvalid
	}
	hash, err := uc.hashPassword(newPassword)
	if err != nil {
		return ErrUserInvalidArgument
	}
	userID, _, ok, err := uc.tokens.ConfirmPasswordReset(ctx, token, hash)
	if err != nil {
		return ErrEmailTokenStoreUnavailable
	}
	if !ok {
		return ErrPasswordResetTokenInvalid
	}
	if uc.sessions != nil {
		if err := uc.sessions.RevokeUser(ctx, userID); err != nil {
			log.Warn("account email: refresh session cleanup failed")
		}
	}
	return nil
}

func (uc *AccountEmailUsecase) checkLimit(ctx context.Context, key string, limit int64, window time.Duration, denied error) error {
	if uc.limiter == nil {
		return ErrEmailTokenStoreUnavailable
	}
	ok, err := uc.limiter.Allow(ctx, key, limit, window)
	if err != nil {
		return ErrEmailTokenStoreUnavailable
	}
	if !ok {
		return denied
	}
	return nil
}
