package biz

import (
	"context"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type fakeCommentRepo struct {
	comments map[uuid.UUID]*Comment
	seq      int
}

func newFakeCommentRepo() *fakeCommentRepo {
	return &fakeCommentRepo{comments: map[uuid.UUID]*Comment{}}
}

func (f *fakeCommentRepo) Create(_ context.Context, c *Comment) (*Comment, error) {
	f.seq++
	id, _ := uuid.NewV7()
	c.ID = id
	stored := *c
	f.comments[id] = &stored
	return &stored, nil
}

func (f *fakeCommentRepo) FindByID(_ context.Context, id uuid.UUID) (*Comment, error) {
	c, ok := f.comments[id]
	if !ok {
		return nil, ErrCommentNotFound
	}
	return c, nil
}

func (f *fakeCommentRepo) ListApprovedBySlug(_ context.Context, slug string, limit, offset int) ([]*Comment, error) {
	var out []*Comment
	for _, c := range f.comments {
		if c.ArticleSlug == slug && c.Status == CommentStatusApproved {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeCommentRepo) List(_ context.Context, _ ...CommentListOption) ([]*Comment, error) {
	var out []*Comment
	for _, c := range f.comments {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeCommentRepo) Approve(_ context.Context, id uuid.UUID) (*Comment, error) {
	c, ok := f.comments[id]
	if !ok {
		return nil, ErrCommentNotFound
	}
	c.Status = CommentStatusApproved
	return c, nil
}

func (f *fakeCommentRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.comments[id]; !ok {
		return ErrCommentNotFound
	}
	delete(f.comments, id)
	return nil
}

type allowAllLimiter struct{ calls int }

func (a *allowAllLimiter) Allow(context.Context, string) (bool, error) { a.calls++; return true, nil }

type denyAllLimiter struct{ calls int }

func (a *denyAllLimiter) Allow(context.Context, string) (bool, error) { a.calls++; return false, nil }

func TestCommentUsecaseSubmitValidation(t *testing.T) {
	ctx := context.Background()
	articles := NewArticleUsecase(newFakeArticleRepo())
	if _, err := articles.CreateArticle(ctx, &Article{Slug: "commentable", Title: "t", ContentMD: "c"}); err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}
	repo := newFakeCommentRepo()
	limiter := &allowAllLimiter{}
	uc := NewCommentUsecase(repo, articles, limiter)

	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "nope", DisplayName: "n", Content: "c"}, "1.2.3.4"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("Submit(unknown article) error = %v, want not found", err)
	}
	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", DisplayName: "", Content: "c"}, "1.2.3.4"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("Submit(empty name) error = %v, want bad request", err)
	}
	long := make([]rune, CommentContentMaxRunes+1)
	for i := range long {
		long[i] = '字'
	}
	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", DisplayName: "n", Content: string(long)}, "1.2.3.4"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("Submit(overlong content) error = %v, want bad request", err)
	}

	// A comment on a DRAFT article is rejected: drafts are not publicly
	// readable, so they are not commentable either.
	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", DisplayName: "n", Content: "c"}, "1.2.3.4"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("Submit(draft article) error = %v, want not found", err)
	}

	// Publish the article through the usecase and submit for real.
	if _, err := articles.UpdateArticle(ctx, &Article{Slug: "commentable", Title: "t", ContentMD: "c", Status: ArticleStatusPublished}); err != nil {
		t.Fatalf("UpdateArticle(publish) error = %v", err)
	}
	submitted, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", DisplayName: "访客", Content: "好文章"}, "1.2.3.4")
	if err != nil {
		t.Fatalf("Submit(published) error = %v", err)
	}
	if submitted.Status != CommentStatusPending {
		t.Fatalf("Submit status = %v, want pending", submitted.Status)
	}
	// Public reads only see approved comments, so the pending one is absent.
	approved, err := uc.ListPublic(ctx, "commentable", 20, 0)
	if err != nil || len(approved) != 0 {
		t.Fatalf("ListPublic before approval = (%v, %v), want empty", approved, err)
	}
	if limiter.calls < 1 {
		t.Fatalf("limiter never consulted")
	}
}

func TestCommentUsecaseSubmitRateLimited(t *testing.T) {
	ctx := context.Background()
	limiter := &denyAllLimiter{}
	uc := NewCommentUsecase(newFakeCommentRepo(), NewArticleUsecase(newFakeArticleRepo()), limiter)

	// The budget is checked before anything else, matching the login flow.
	_, err := uc.Submit(ctx, &Comment{ArticleSlug: "any", DisplayName: "n", Content: "c"}, "1.2.3.4")
	if !kratoserrors.IsTooManyRequests(err) {
		t.Fatalf("Submit(denied) error = %v, want too many requests", err)
	}
	if limiter.calls != 1 {
		t.Fatalf("limiter calls = %d, want 1", limiter.calls)
	}
}

func TestCommentUsecaseModerationFlow(t *testing.T) {
	ctx := context.Background()
	articles := NewArticleUsecase(newFakeArticleRepo())
	if _, err := articles.CreateArticle(ctx, &Article{Slug: "published", Title: "t", ContentMD: "c"}); err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}
	if _, err := articles.UpdateArticle(ctx, &Article{Slug: "published", Title: "t", ContentMD: "c", Status: ArticleStatusPublished}); err != nil {
		t.Fatalf("UpdateArticle(publish) error = %v", err)
	}
	repo := newFakeCommentRepo()
	uc := NewCommentUsecase(repo, articles, &allowAllLimiter{})

	submitted, err := uc.Submit(ctx, &Comment{ArticleSlug: "published", DisplayName: "n", Content: "c"}, "ip")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	approved, err := uc.Approve(ctx, submitted.ID)
	if err != nil || approved.Status != CommentStatusApproved {
		t.Fatalf("Approve() = (%v, %v), want approved", approved, err)
	}
	// Idempotent approval.
	if again, err := uc.Approve(ctx, submitted.ID); err != nil || again.Status != CommentStatusApproved {
		t.Fatalf("Approve(again) = (%v, %v), want approved", again, err)
	}
	visible, err := uc.ListPublic(ctx, "published", 20, 0)
	if err != nil || len(visible) != 1 {
		t.Fatalf("ListPublic after approval = (%d comments, %v), want 1", len(visible), err)
	}
	if err := uc.Delete(ctx, submitted.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	visible, err = uc.ListPublic(ctx, "published", 20, 0)
	if err != nil || len(visible) != 0 {
		t.Fatalf("ListPublic after delete = (%d comments, %v), want 0", len(visible), err)
	}
	if err := uc.Delete(ctx, submitted.ID); !kratoserrors.IsNotFound(err) {
		t.Fatalf("Delete(again) error = %v, want not found", err)
	}
}
