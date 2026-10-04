package biz

import (
	"context"
	"errors"
	"testing"
)

// fakeSubscriberRepo records created emails; FindByEmail reports existence
// from the same map so idempotent re-subscribe can be exercised.
type fakeSubscriberRepo struct {
	emails map[string]bool
}

func (f *fakeSubscriberRepo) FindByEmail(_ context.Context, email string) (*Subscriber, error) {
	if f.emails[email] {
		return &Subscriber{Email: email}, nil
	}
	return nil, ErrSubscriberNotFound
}
func (f *fakeSubscriberRepo) CreateSubscriber(_ context.Context, s *Subscriber) (*Subscriber, error) {
	f.emails[s.Email] = true
	return s, nil
}
func (f *fakeSubscriberRepo) ListSubscribers(context.Context, int, int) ([]*Subscriber, error) {
	return nil, nil
}
func (f *fakeSubscriberRepo) DeleteSubscriber(_ context.Context, email string) error {
	if !f.emails[email] {
		return ErrSubscriberNotFound
	}
	delete(f.emails, email)
	return nil
}

func TestValidSubscriberEmail(t *testing.T) {
	for email, want := range map[string]bool{
		"reader@example.com":   true,
		"first.last+tag@io.io": true,
		"Reader@Example.COM":   false, // must be normalized before validation
		"not-an-email":         false,
		"@example.com":         false,
		"a@b":                  false, // no tld dot
		"":                     false,
	} {
		if got := ValidSubscriberEmail(email); got != want {
			t.Fatalf("ValidSubscriberEmail(%q) = %v, want %v", email, got, want)
		}
	}
}

func TestSubscribeIdempotentAndNormalized(t *testing.T) {
	repo := &fakeSubscriberRepo{emails: map[string]bool{}}
	uc := NewSubscriberUsecase(repo, nil)

	if err := uc.Subscribe(context.Background(), " Reader@Example.COM "); err != nil {
		t.Fatalf("first subscribe: %v", err)
	}
	if err := uc.Subscribe(context.Background(), "reader@example.com"); err != nil {
		t.Fatalf("re-subscribe must succeed idempotently: %v", err)
	}
	if len(repo.emails) != 1 {
		t.Fatalf("stored emails = %v, want exactly one lowercased entry", repo.emails)
	}
	if !repo.emails["reader@example.com"] {
		t.Fatalf("email stored unnormalized: %v", repo.emails)
	}
	if err := uc.Subscribe(context.Background(), "not-an-email"); !errors.Is(err, ErrSubscriberInvalidArgument) {
		t.Fatalf("invalid email = %v, want invalid argument", err)
	}
}
