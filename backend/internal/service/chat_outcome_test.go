package service

import (
	"context"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

type outcomeLLM struct{}

func (outcomeLLM) Generate(context.Context, string, string) (string, error) { return "unused", nil }
func TestChatMapsRetrievalOutcome(t *testing.T) {
	svc := NewChatService(biz.NewChatUsecase(biz.NewArticleUsecase(&stubArticleRepo{}, nil), outcomeLLM{}, nil))
	reply, err := svc.Chat(context.Background(), &v1.ChatRequest{Query: "question"})
	if err != nil || reply.Outcome != v1.ChatOutcome_CHAT_OUTCOME_RETRIEVAL_UNAVAILABLE || reply.Answer == "" {
		t.Fatalf("result: %+v, %v", reply, err)
	}
}
