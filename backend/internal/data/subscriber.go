package data

import (
	"context"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/subscriber"

	"github.com/redis/go-redis/v9"
)

// toBizSubscriber converts a persisted subscriber into the domain
// representation.
func toBizSubscriber(po *ent.Subscriber) *biz.Subscriber {
	if po == nil {
		return nil
	}
	return &biz.Subscriber{
		ID:        po.ID,
		Email:     po.Email,
		CreatedAt: po.CreatedAt,
		UpdatedAt: po.UpdatedAt,
	}
}

type subscriberRepo struct {
	data *Data
}

// NewSubscriberRepo creates a new SubscriberRepo instance.
func NewSubscriberRepo(data *Data) biz.SubscriberRepo {
	return &subscriberRepo{data: data}
}

func (r *subscriberRepo) FindByEmail(ctx context.Context, email string) (*biz.Subscriber, error) {
	po, err := r.data.db.Subscriber.Query().
		Where(subscriber.EmailEQ(email)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrSubscriberNotFound
		}
		return nil, err
	}
	return toBizSubscriber(po), nil
}

func (r *subscriberRepo) CreateSubscriber(ctx context.Context, s *biz.Subscriber) (*biz.Subscriber, error) {
	po, err := r.data.db.Subscriber.Create().
		SetEmail(s.Email).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return toBizSubscriber(po), nil
}

func (r *subscriberRepo) ListSubscribers(ctx context.Context, offset int, limit int) ([]*biz.Subscriber, error) {
	pos, err := r.data.db.Subscriber.Query().
		Order(subscriber.ByCreatedAt()).
		Offset(offset).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*biz.Subscriber, 0, len(pos))
	for _, po := range pos {
		out = append(out, toBizSubscriber(po))
	}
	return out, nil
}

// DeleteSubscriber removes a subscription by its canonical (lowercased)
// email.
func (r *subscriberRepo) DeleteSubscriber(ctx context.Context, email string) error {
	affected, err := r.data.db.Subscriber.Delete().
		Where(subscriber.EmailEQ(email)).
		Exec(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return biz.ErrSubscriberNotFound
	}
	return nil
}

// NewSubscribeRateLimiter builds the email-subscription throttler with a
// fixed budget: subscribing is rare and the knob can wait until real sending
// lands. Same fail-open semantics as the other limiters.
func NewSubscribeRateLimiter(rdb redis.UniversalClient) biz.SubscribeRateLimiter {
	return &rateLimiter{rdb: rdb, attempts: biz.DefaultSubscribeAttempts, window: biz.DefaultSubscribeWindow}
}
