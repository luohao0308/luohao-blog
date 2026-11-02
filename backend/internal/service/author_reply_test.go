package service

import (
	"context"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"testing"
)

func TestPutAuthorReplyRequiresExplicitVersion(t *testing.T) {
	s := NewCommentService(nil)
	for _, req := range []*v1.PutAuthorReplyRequest{nil, {Id: "00000000-0000-7000-8000-000000000001", Content: "reply"}} {
		if _, e := s.PutAuthorReply(context.Background(), req); e != biz.ErrCommentInvalidArgument {
			t.Fatal(e)
		}
	}
}
