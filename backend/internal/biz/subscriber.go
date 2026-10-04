package biz

import (
	"context"
	"regexp"
	"strings"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/uuid"
)

var (
	// ErrSubscriberNotFound is returned when a subscriber does not exist.
	ErrSubscriberNotFound = errors.NotFound(v1.ErrorReason_SUBSCRIBER_NOT_FOUND.String(), "subscriber not found")
	// ErrSubscriberInvalidArgument is returned when a subscribe request is invalid.
	ErrSubscriberInvalidArgument = errors.BadRequest(v1.ErrorReason_SUBSCRIBER_INVALID_ARGUMENT.String(), "invalid subscriber argument")
	// ErrSubscriberTooManyAttempts is returned when a client exceeds the
	// subscription budget.
	ErrSubscriberTooManyAttempts = errors.TooManyRequests(v1.ErrorReason_SUBSCRIBER_TOO_MANY_ATTEMPTS.String(), "too many subscribe attempts")
)

// DefaultSubscribeAttempts/DefaultSubscribeWindow bound anonymous
// subscription submissions per client IP; subscribing is rare, so the budget
// is deliberately tighter than comments and carries no conf knob until real
// sending lands.
const (
	DefaultSubscribeAttempts int64 = 5
	DefaultSubscribeWindow         = time.Hour
)

// subscriberEmailPattern is the practical email shape check: a real-world
// address form without pretending to implement RFC 5322 in full.
var subscriberEmailPattern = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)

// Subscriber is a registered email for future article notifications.
type Subscriber struct {
	ID        uuid.UUID
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SubscriberRepo is a subscriber repo.
type SubscriberRepo interface {
	FindByEmail(ctx context.Context, email string) (*Subscriber, error)
	CreateSubscriber(ctx context.Context, s *Subscriber) (*Subscriber, error)
	ListSubscribers(ctx context.Context, offset int, limit int) ([]*Subscriber, error)
	DeleteSubscriber(ctx context.Context, email string) error
}

// SubscribeRateLimiter is the fixed-window limiter for anonymous subscribe
// submissions; a distinct type so wire keeps separate budgets per concern.
type SubscribeRateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// SubscriberUsecase is a Subscriber usecase.
type SubscriberUsecase struct {
	repo    SubscriberRepo
	limiter SubscribeRateLimiter
}

// NewSubscriberUsecase new a Subscriber usecase.
func NewSubscriberUsecase(repo SubscriberRepo, limiter SubscribeRateLimiter) *SubscriberUsecase {
	return &SubscriberUsecase{repo: repo, limiter: limiter}
}

// Subscribe registers an email, idempotently: an already-registered address
// succeeds without change so the public endpoint cannot be used to enumerate
// the list.
func (uc *SubscriberUsecase) Subscribe(ctx context.Context, email string) error {
	if uc.limiter != nil {
		ok, err := uc.limiter.Allow(ctx, "subscribe")
		if err != nil {
			log.Warn("subscriber: rate limiter unavailable, failing open", "err", err)
		} else if !ok {
			return ErrSubscriberTooManyAttempts
		}
	}
	email = normalizeSubscriberEmail(email)
	if !ValidSubscriberEmail(email) {
		return ErrSubscriberInvalidArgument
	}
	if _, err := uc.repo.FindByEmail(ctx, email); err == nil {
		return nil
	}
	_, err := uc.repo.CreateSubscriber(ctx, &Subscriber{Email: email})
	return err
}

// ListSubscribers returns a page of subscribers, newest first.
func (uc *SubscriberUsecase) ListSubscribers(ctx context.Context, limit, offset int) ([]*Subscriber, error) {
	if limit <= 0 || offset < 0 {
		return nil, ErrSubscriberInvalidArgument
	}
	return uc.repo.ListSubscribers(ctx, offset, limit)
}

// Delete removes a subscription by email.
func (uc *SubscriberUsecase) Delete(ctx context.Context, email string) error {
	email = normalizeSubscriberEmail(email)
	if !ValidSubscriberEmail(email) {
		return ErrSubscriberInvalidArgument
	}
	return uc.repo.DeleteSubscriber(ctx, email)
}

// normalizeSubscriberEmail lowercases and trims the address: the stored form
// is canonical, so lookups and uniqueness are case-insensitive in practice.
func normalizeSubscriberEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidSubscriberEmail reports whether the address is storable: practical
// shape, at most 191 runes so the unique index bound holds.
func ValidSubscriberEmail(email string) bool {
	if len(email) == 0 || len(email) > 191 {
		return false
	}
	return subscriberEmailPattern.MatchString(email)
}
