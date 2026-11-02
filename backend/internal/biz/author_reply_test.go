package biz

import (
	"context"
	"github.com/google/uuid"
	"testing"
)

type replyPolicyRepo struct {
	*fakeCommentRepo
	old    *Comment
	writes int
}

func (r *replyPolicyRepo) GetAuthorReply(context.Context, uuid.UUID) (*Comment, error) {
	return r.old, nil
}
func (r *replyPolicyRepo) PutAuthorReply(_ context.Context, _ uuid.UUID, c *Comment, _ uint64) (*Comment, error) {
	r.writes++
	return c, nil
}
func TestAuthorReplyRejectKeepsOldReply(t *testing.T) {
	uc, articles, author, claims := newCommentTestUsecase(t, &allowAllLimiter{})
	publishCommentableArticle(t, articles)
	ctx := NewAuthContext(context.Background(), claims)
	root := uuid.Must(uuid.NewV7())
	old := &Comment{DisplayName: "旧快照", Content: "旧正文", ReplyVersion: 4}
	repo := &replyPolicyRepo{fakeCommentRepo: newFakeCommentRepo(), old: old}
	repo.comments[root] = &Comment{ID: root, ArticleSlug: "commentable", Status: CommentStatusApproved}
	uc.repo = repo
	uc.moderation = CommentModerationPolicy{Mode: "auto"}
	if author.Role != UserRoleAdmin {
		t.Fatal("fixture author not admin")
	}
	for _, body := range []string{"", "博彩送彩金 联系微信 abc123"} {
		if _, err := uc.PutAuthorReply(ctx, root, body, 4, "127.0.0.1"); err == nil {
			t.Fatal("invalid edit accepted")
		}
	}
	if repo.writes != 0 || old.Content != "旧正文" || old.ReplyVersion != 4 {
		t.Fatal("rejection changed old reply")
	}
	uc.limiter = nil
	if _, err := uc.PutAuthorReply(ctx, root, "普通回复", 4, ""); err != ErrContentSafetyUnavailable {
		t.Fatal("outage", err)
	}
	if repo.writes != 0 {
		t.Fatal("outage wrote")
	}
}

func TestAuthorReplyKeepsRootSafetyHash(t *testing.T) {
	author := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	identity := uuid.MustParse("00000000-0000-7000-8000-000000000003")
	c := &Comment{ID: uuid.MustParse("00000000-0000-7000-8000-000000000001"), UserID: &author, ArticleSlug: "post", DisplayName: "作者", Content: "正文"}
	if got := MiniCommentInputHash(c, "test-app", identity); got != "88f43a38de46f94be22a7f9b48eb45282def4e3ff66faec91b680275646faf8c" {
		t.Fatal("legacy root fingerprint changed", got)
	}
}
