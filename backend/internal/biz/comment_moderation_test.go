package biz

import (
	"context"
	"fmt"
	"strings"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

func TestCommentSubmissionModeration(t *testing.T) {
	for _, tt := range []struct {
		name, content string
		want          CommentStatus
	}{
		{"discussion", "文章有帮助，不过这个实现可能存在竞态。", CommentStatusApproved},
		{"technical terms", "Vue3、Go1.26 和微信登录的例子不错 👍", CommentStatusApproved},
		{"code comment", "这里应该 // TODO 后续优化", CommentStatusApproved},
		{"relative URL", "//example.com/docs", CommentStatusPending},
		{"URL", "参考 https://example.com/docs", CommentStatusPending},
		{"bare domain", "访问 example.cn 查看详情", CommentStatusPending},
		{"email", "请联系 user@example.org", CommentStatusPending},
		{"phone", "联系 13812345678", CommentStatusPending},
		{"solicitation", "加微信领取优惠", CommentStatusPending},
		{"obfuscated URL", "ｈｔｔｐｓ：／／ｅｘａｍｐｌｅ．ｃｏｍ", CommentStatusPending},
		{"invisible URL", "h\u200bttps://example.com", CommentStatusPending},
		{"repeated characters", strings.Repeat("哈", 25), CommentStatusPending},
	} {
		t.Run(tt.name, func(t *testing.T) {
			uc, articles, _, claims := newCommentTestUsecase(t, &allowAllLimiter{})
			publishCommentableArticle(t, articles)
			c, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "commentable", Content: tt.content, Status: CommentStatusApproved}, claims, "127.0.0.1")
			if err != nil {
				t.Fatal(err)
			}
			if c.Status != tt.want {
				t.Fatalf("status = %v, want %v", c.Status, tt.want)
			}
			public, err := uc.ListPublic(context.Background(), "commentable", 20, 0)
			if err != nil {
				t.Fatal(err)
			}
			wantLen := 0
			if tt.want == CommentStatusApproved {
				wantLen = 1
			}
			if len(public) != wantLen {
				t.Fatalf("public count = %d, want %d", len(public), wantLen)
			}
		})
	}
}

func TestCommentTrimsWhitespace(t *testing.T) {
	uc, articles, _, claims := newCommentTestUsecase(t, &allowAllLimiter{})
	publishCommentableArticle(t, articles)
	if _, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "commentable", Content: " \n\t "}, claims, "ip"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("blank = %v, want bad request", err)
	}
	c, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "commentable", Content: "  有帮助！ \n"}, claims, "ip")
	if err != nil || c.Content != "有帮助！" {
		t.Fatalf("trim = %v, %v", c, err)
	}
}

type unavailableCommentLimiter struct{}

func (unavailableCommentLimiter) Allow(context.Context, string) (bool, error) {
	return false, fmt.Errorf("offline")
}
func TestCommentLimiterUnavailableRequiresReview(t *testing.T) {
	uc, articles, _, claims := newCommentTestUsecase(t, unavailableCommentLimiter{})
	publishCommentableArticle(t, articles)
	c, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "commentable", Content: "普通评论"}, claims, "ip")
	if err != nil || c.Status != CommentStatusPending {
		t.Fatalf("offline limiter = %v, %v", c, err)
	}
}

func TestCommentsOfUnpublishedArticleAreHidden(t *testing.T) {
	uc, articles, _, claims := newCommentTestUsecase(t, &allowAllLimiter{})
	publishCommentableArticle(t, articles)
	ctx := context.Background()
	c, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: "hello"}, claims, "ip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = uc.Approve(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = articles.UpdateArticle(ctx, &Article{Slug: "commentable", Title: "t", ContentMD: "c", Status: ArticleStatusDraft}); err != nil {
		t.Fatal(err)
	}
	public, err := uc.ListPublic(ctx, "commentable", 20, 0)
	if err != nil || len(public) != 0 {
		t.Fatalf("draft public = %v, %v", public, err)
	}
	if err = articles.DeleteArticle(ctx, "commentable"); err != nil {
		t.Fatal(err)
	}
	public, err = uc.ListPublic(ctx, "commentable", 20, 0)
	if err != nil || len(public) != 0 {
		t.Fatalf("deleted public = %v, %v", public, err)
	}
}

func TestCommentModerationModesAndKeywords(t *testing.T) {
	for _, tt := range []struct {
		mode     string
		keywords []string
		content  string
		want     CommentStatus
	}{
		{"manual", nil, "正常讨论", CommentStatusPending},
		{"typo", nil, "正常讨论", CommentStatusPending},
		{"SMART", []string{"自定义 风险"}, "自 定义风险", CommentStatusPending},
		{"smart", []string{" "}, "正常讨论", CommentStatusApproved},
	} {
		uc, articles, _, claims := newCommentTestUsecase(t, &allowAllLimiter{})
		uc.moderation = CommentModerationPolicy{Mode: tt.mode, ReviewKeywords: tt.keywords}
		publishCommentableArticle(t, articles)
		c, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "commentable", Content: tt.content}, claims, "ip")
		if err != nil || c.Status != tt.want {
			t.Fatalf("mode %q = %v, %v", tt.mode, c, err)
		}
	}
}

type keyCommentLimiter struct {
	keys   []string
	denied string
}

func (l *keyCommentLimiter) Allow(_ context.Context, key string) (bool, error) {
	l.keys = append(l.keys, key)
	return key != l.denied, nil
}
func TestCommentAccountBudgetAcrossIPs(t *testing.T) {
	l := &keyCommentLimiter{}
	uc, articles, author, claims := newCommentTestUsecase(t, l)
	publishCommentableArticle(t, articles)
	ctx := context.Background()
	accountKey := "comment:user:" + author.ID.String()
	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: "first"}, claims, "ip-a"); err != nil {
		t.Fatal(err)
	}
	if len(l.keys) != 2 || l.keys[0] != accountKey || l.keys[1] != "comment:ip-a" {
		t.Fatalf("budgets = %v", l.keys)
	}
	l.denied = accountKey
	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: "second"}, claims, "ip-b"); !kratoserrors.IsTooManyRequests(err) {
		t.Fatalf("rotating IP = %v", err)
	}
	if len(uc.repo.(*fakeCommentRepo).comments) != 1 {
		t.Fatal("denied submission must not be stored")
	}
}

func TestCommentNilAndRuneLimit(t *testing.T) {
	uc, articles, _, claims := newCommentTestUsecase(t, &allowAllLimiter{})
	publishCommentableArticle(t, articles)
	if _, err := uc.Submit(context.Background(), nil, claims, "ip"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("nil comment = %v", err)
	}
	// Supplementary characters count as one rune; varying text avoids spam rules.
	text := strings.Repeat("好👍", 500)
	if _, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "commentable", Content: text}, claims, "ip"); err != nil {
		t.Fatalf("1000 runes = %v", err)
	}
	if _, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "commentable", Content: text + "字"}, claims, "ip"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("1001 runes = %v", err)
	}
}
