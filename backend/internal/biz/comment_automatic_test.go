package biz

import (
	"context"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/errors"
)

func TestAutomaticCommentsPublishUsefulLinksAndRejectSpamWithoutPendingRows(t *testing.T) {
	for _, tt := range []struct {
		name, content string
		rejected      bool
	}{
		{"discussion", "这个实现有竞态，可以补一个测试。", false},
		{"technical reference", "参考 https://go.dev/doc/ ，邮箱 user@example.org，示例号码 13812345678", false},
		{"code", "这里应该 // TODO 后续优化", false},
		{"spam", "加微信领取优惠", true},
		{"obfuscated spam", "加\u200bＶＸ领取优惠", true},
		{"flood", strings.Repeat("哈", 25), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			uc, articles, _, claims := newCommentTestUsecase(t, &allowAllLimiter{})
			uc.moderation.Mode = "auto"
			publishCommentableArticle(t, articles)
			c, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "commentable", Content: tt.content}, claims, "ip")
			if tt.rejected {
				if !errors.IsBadRequest(err) || c != nil {
					t.Fatalf("spam accepted: %v %v", c, err)
				}
				if len(uc.repo.(*fakeCommentRepo).comments) != 0 {
					t.Fatal("rejected comment created manual work")
				}
				return
			}
			if err != nil || c.Status != CommentStatusApproved {
				t.Fatalf("ordinary comment not automatic: %v %v", c, err)
			}
			public, err := uc.ListPublic(context.Background(), "commentable", 20, 0)
			if err != nil || len(public) != 1 {
				t.Fatal("accepted comment not public", err)
			}
		})
	}
}

func TestAutomaticCommentsLimiterFailureDoesNotCreateManualQueue(t *testing.T) {
	uc, articles, _, claims := newCommentTestUsecase(t, unavailableCommentLimiter{})
	uc.moderation.Mode = "auto"
	publishCommentableArticle(t, articles)
	c, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "commentable", Content: "普通评论"}, claims, "ip")
	if c != nil || errors.Code(err) != 503 || len(uc.repo.(*fakeCommentRepo).comments) != 0 {
		t.Fatalf("outage created hidden/manual comment: %v %v", c, err)
	}
}

func TestMiniAutomaticCommentPassWithTechnicalLinkPublishes(t *testing.T) {
	uc, repo, _, claims := newMiniUsecaseFixture(t)
	uc.base.moderation.Mode = "auto"
	r, err := uc.Create(context.Background(), claims, "commentable", "参考 https://go.dev/doc/", "code-fixture", "auto-link", "")
	if err != nil || r == nil || !r.Visible() || !repo.approve {
		t.Fatalf("pass still requires manual approval: %v %v", r, err)
	}
}

func TestMiniAutomaticCommentRejectsLocalSpamBeforeWechatCall(t *testing.T) {
	uc, repo, checker, claims := newMiniUsecaseFixture(t)
	uc.base.moderation.Mode = "auto"
	r, err := uc.Create(context.Background(), claims, "commentable", "加微信领取优惠", "code-fixture", "auto-spam", "")
	if !errors.IsBadRequest(err) || r != nil || repo.creates != 0 || len(checker.inputs) != 0 {
		t.Fatalf("spam consumed upstream or created queue: %v %v", r, err)
	}
}

func TestMiniAutomaticCommentRecoveryPublishesWithoutAdmin(t *testing.T) {
	uc, repo, _, claims := newMiniUsecaseFixture(t)
	uc.base.moderation.Mode = "auto"
	r, err := uc.Create(context.Background(), claims, "commentable", "普通评论", "code-fixture", "auto-recover", "")
	if err != nil {
		t.Fatal(err)
	}
	repo.record.State = MiniCommentUnavailable
	repo.record.Comment.Status = CommentStatusPending
	repo.record.Generation = 2
	repo.approve = false
	r, err = uc.Recheck(context.Background(), claims, r.Comment.ID, "code-fixture", "")
	if err != nil || r == nil || !r.Visible() || !repo.approve || !repo.recoveryApproval {
		t.Fatalf("recovered check stuck pending: %v %v", r, err)
	}
}

func TestAutomaticCommentsMissingLimiterDoesNotPublish(t *testing.T) {
	uc, articles, _, claims := newCommentTestUsecase(t, nil)
	uc.moderation.Mode = "auto"
	publishCommentableArticle(t, articles)
	if _, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "commentable", Content: "ordinary"}, claims, "ip"); errors.Code(err) != 503 || len(uc.repo.(*fakeCommentRepo).comments) != 0 {
		t.Fatal("missing budget bypassed protection", err)
	}
}
func TestMiniAutomaticCommentsNonPassAndChangedStoredNicknameStayPrivate(t *testing.T) {
	for _, decision := range []ContentSafetyDecision{ContentSafetyReview, ContentSafetyRisky, "unexpected"} {
		uc, repo, checker, claims := newMiniUsecaseFixture(t)
		uc.base.moderation.Mode = "auto"
		checker.decision = decision
		r, err := uc.Create(context.Background(), claims, "commentable", "ordinary", "code-fixture", "auto-nonpass", "")
		if err != nil || r.Visible() || repo.approve {
			t.Fatal("non-pass became public", err)
		}
	}
	uc, repo, checker, claims := newMiniUsecaseFixture(t)
	uc.base.moderation.Mode = "auto"
	repo.storedName = "加微信领取优惠"
	r, err := uc.Create(context.Background(), claims, "commentable", "ordinary", "code-fixture", "auto-changed-name", "")
	if err != nil || r.Visible() || r.State != MiniCommentRisky || len(checker.inputs) != 0 {
		t.Fatal("changed persisted nickname bypassed local checks", err)
	}
}
