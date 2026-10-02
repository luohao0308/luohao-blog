package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

// ChatService serves retrieval-augmented answers over published articles.
type ChatService struct {
	v1.UnimplementedChatServiceServer

	uc *biz.ChatUsecase
}

// NewChatService new a chat service.
func NewChatService(uc *biz.ChatUsecase) *ChatService {
	return &ChatService{uc: uc}
}

// Chat answers one question with citations. The client IP feeds the
// per-IP question budget (same derivation as the comment throttler).
func (s *ChatService) Chat(ctx context.Context, req *v1.ChatRequest) (*v1.ChatReply, error) {
	answer, err := s.uc.Ask(ctx, req.GetQuery(), int(req.GetTopK()), clientIP(ctx))
	if err != nil {
		return nil, err
	}
	reply := &v1.ChatReply{
		Answer:    answer.Answer,
		Citations: make([]*v1.CitedArticle, 0, len(answer.Citations)),
	}
	for _, a := range answer.Citations {
		reply.Citations = append(reply.Citations, &v1.CitedArticle{
			Slug:    a.Slug,
			Title:   a.Title,
			Summary: a.Summary,
		})
	}
	return reply, nil
}
