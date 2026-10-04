package service

import (
	"context"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

// stubSubscriberRepo backs the service-level tests; Subscribe accepts
// everything so validation cases flow through the usecase.
type stubSubscriberRepo struct {
	emails map[string]bool
}

func (s *stubSubscriberRepo) FindByEmail(_ context.Context, email string) (*biz.Subscriber, error) {
	if s.emails[email] {
		return &biz.Subscriber{Email: email}, nil
	}
	return nil, biz.ErrSubscriberNotFound
}
func (s *stubSubscriberRepo) CreateSubscriber(_ context.Context, sub *biz.Subscriber) (*biz.Subscriber, error) {
	s.emails[sub.Email] = true
	return sub, nil
}
func (s *stubSubscriberRepo) ListSubscribers(context.Context, int, int) ([]*biz.Subscriber, error) {
	return []*biz.Subscriber{
		{Email: "reader@example.com"},
		{Email: "other@example.com"},
	}, nil
}
func (s *stubSubscriberRepo) DeleteSubscriber(context.Context, string) error {
	return nil
}

func newStubSubscriberService() *SubscriberService {
	return NewSubscriberService(biz.NewSubscriberUsecase(&stubSubscriberRepo{emails: map[string]bool{}}, nil))
}

func TestSubscribeValidation(t *testing.T) {
	svc := newStubSubscriberService()
	if _, err := svc.Subscribe(context.Background(), &v1.SubscribeRequest{Email: "not-an-email"}); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("invalid email = %v, want bad request", err)
	}
	if _, err := svc.Subscribe(context.Background(), &v1.SubscribeRequest{Email: "reader@example.com"}); err != nil {
		t.Fatalf("valid subscribe = %v, want nil", err)
	}
}

func TestListSubscribersCarriesEntries(t *testing.T) {
	svc := newStubSubscriberService()
	set, err := svc.ListSubscribers(context.Background(), &v1.ListSubscribersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Subscribers) != 2 || set.Subscribers[0].Email != "reader@example.com" {
		t.Fatalf("subscribers = %+v, want two entries", set.Subscribers)
	}
	if set.NextPageToken != "" {
		t.Fatalf("next page token = %q, want empty for a short page", set.NextPageToken)
	}
}
