package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SubscriberService registers emails for future notifications; actual
// sending is not wired yet.
type SubscriberService struct {
	v1.UnimplementedSubscriberServiceServer

	uc *biz.SubscriberUsecase
}

// NewSubscriberService new a subscriber service.
func NewSubscriberService(uc *biz.SubscriberUsecase) *SubscriberService {
	return &SubscriberService{uc: uc}
}

// Subscribe registers an email address, idempotently, throttled per client.
func (s *SubscriberService) Subscribe(ctx context.Context, req *v1.SubscribeRequest) (*emptypb.Empty, error) {
	if err := s.uc.Subscribe(ctx, req.GetEmail()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ListSubscribers lists registered emails, newest first.
func (s *SubscriberService) ListSubscribers(ctx context.Context, req *v1.ListSubscribersRequest) (*v1.SubscriberSet, error) {
	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, invalidListArgument(err)
	}
	if !pageOffsetWithinWindow(pageToken.Offset) {
		return nil, invalidListArgument(errPageOffsetOutOfRange)
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	clampPageSize(&req.PageSize)
	subscribers, err := s.uc.ListSubscribers(ctx, int(req.PageSize), int(pageToken.Offset))
	if err != nil {
		return nil, err
	}
	set := &v1.SubscriberSet{
		Subscribers: make([]*v1.Subscriber, 0, len(subscribers)),
	}
	if len(subscribers) >= int(req.PageSize) {
		set.NextPageToken = pageToken.Next(req).String()
	}
	for _, subscriber := range subscribers {
		set.Subscribers = append(set.Subscribers, convertSubscriberReply(subscriber))
	}
	return set, nil
}

// DeleteSubscriber removes a subscription.
func (s *SubscriberService) DeleteSubscriber(ctx context.Context, req *v1.DeleteSubscriberRequest) (*emptypb.Empty, error) {
	if err := s.uc.Delete(ctx, req.GetEmail()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func convertSubscriberReply(in *biz.Subscriber) *v1.Subscriber {
	if in == nil {
		return nil
	}
	return &v1.Subscriber{
		Id:        in.ID.String(),
		Email:     in.Email,
		CreatedAt: timestamppb.New(in.CreatedAt),
	}
}
