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

// newCommentTestUsecase wires the comment usecase against fakes and seeds a
// published article plus a signing reader account, returning both.
func newCommentTestUsecase(t *testing.T, limiter CommentRateLimiter) (*CommentUsecase, *ArticleUsecase, *User, *Claims) {
	t.Helper()
	ctx := context.Background()
	articles := NewArticleUsecase(newFakeArticleRepo(), nil)
	if _, err := articles.CreateArticle(ctx, &Article{Slug: "commentable", Title: "t", ContentMD: "c"}); err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}
	users := NewUserUsecase(newFakeUserRepo())
	author, err := users.CreateAuthor(ctx, "author@example.com", "longenough1", "罗豪")
	if err != nil {
		t.Fatalf("CreateAuthor() error = %v", err)
	}
	claims := &Claims{UserID: author.ID, Role: author.Role}
	uc := NewCommentUsecase(newFakeCommentRepo(), articles, users, limiter)
	return uc, articles, author, claims
}

func publishCommentableArticle(t *testing.T, articles *ArticleUsecase) {
	t.Helper()
	if _, err := articles.UpdateArticle(context.Background(), &Article{Slug: "commentable", Title: "t", ContentMD: "c", Status: ArticleStatusPublished}); err != nil {
		t.Fatalf("UpdateArticle(publish) error = %v", err)
	}
}

func TestCommentUsecaseSubmitRequiresIdentity(t *testing.T) {
	ctx := context.Background()
	uc, _, author, claims := newCommentTestUsecase(t, &allowAllLimiter{})
	publishCommentableArticle(t, uc.articles)

	// Anonymous (no claims): the middleware already 401s these, the usecase
	// re-checks as defense in depth.
	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: "c"}, nil, "1.2.3.4"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("Submit(anonymous) error = %v, want unauthorized", err)
	}
	// A valid token for a since-deleted account must not resurrect as an
	// author.
	ghost := &Claims{UserID: uuid.Must(uuid.NewV7()), Role: UserRoleReader}
	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: "c"}, ghost, "1.2.3.4"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("Submit(deleted account) error = %v, want unauthorized", err)
	}

	// Identity is taken from the account, never from the payload.
	submitted, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: "好文章"}, claims, "1.2.3.4")
	if err != nil {
		t.Fatalf("Submit(authenticated) error = %v", err)
	}
	if submitted.UserID == nil || *submitted.UserID != author.ID {
		t.Fatalf("Submit() user id = %v, want the author account", submitted.UserID)
	}
	if submitted.DisplayName != author.DisplayName {
		t.Fatalf("Submit() display name = %q, want the profile name %q", submitted.DisplayName, author.DisplayName)
	}
	if submitted.Status != CommentStatusPending {
		t.Fatalf("Submit status = %v, want pending", submitted.Status)
	}
}

func TestCommentUsecaseSubmitValidation(t *testing.T) {
	ctx := context.Background()
	uc, _, _, claims := newCommentTestUsecase(t, &allowAllLimiter{})

	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "nope", Content: "c"}, claims, "1.2.3.4"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("Submit(unknown article) error = %v, want not found", err)
	}
	long := make([]rune, CommentContentMaxRunes+1)
	for i := range long {
		long[i] = '字'
	}
	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: string(long)}, claims, "1.2.3.4"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("Submit(overlong content) error = %v, want bad request", err)
	}

	// A comment on a DRAFT article is rejected: drafts are not publicly
	// readable, so they are not commentable either.
	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: "c"}, claims, "1.2.3.4"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("Submit(draft article) error = %v, want not found", err)
	}

	// An account whose stored display name is empty cannot author comments:
	// the identity column would be blank on the public page.
	empty := &User{ID: uuid.Must(uuid.NewV7()), Email: "blank@example.com", Role: UserRoleReader}
	ucusers := uc.users.repo.(*fakeUserRepo)
	ucusers.users[empty.Email] = empty
	blankClaims := &Claims{UserID: empty.ID, Role: empty.Role}
	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: "c"}, blankClaims, "1.2.3.4"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("Submit(empty profile name) error = %v, want bad request", err)
	}
}

func TestCommentUsecaseListPublicAttachesAvatars(t *testing.T) {
	ctx := context.Background()
	uc, _, author, claims := newCommentTestUsecase(t, &allowAllLimiter{})
	publishCommentableArticle(t, uc.articles)

	author.AvatarURL = "/v1/assets/avatars/0123456789abcdef0123456789abcdef.png"

	if _, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: "c"}, claims, "ip"); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	repo := uc.repo.(*fakeCommentRepo)
	var submitted *Comment
	for _, c := range repo.comments {
		submitted = c
	}
	if submitted == nil {
		t.Fatal("Submit() did not store a comment")
	}
	if _, err := uc.Approve(ctx, submitted.ID); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	visible, err := uc.ListPublic(ctx, "commentable", 20, 0)
	if err != nil || len(visible) != 1 {
		t.Fatalf("ListPublic() = (%d comments, %v), want 1", len(visible), err)
	}
	if visible[0].AvatarURL != author.AvatarURL {
		t.Fatalf("ListPublic() avatar = %q, want the author avatar", visible[0].AvatarURL)
	}
}

func TestCommentUsecaseSubmitRateLimited(t *testing.T) {
	limiter := &denyAllLimiter{}
	uc, _, _, claims := newCommentTestUsecase(t, limiter)

	// The budget is checked before anything else, matching the login flow.
	_, err := uc.Submit(context.Background(), &Comment{ArticleSlug: "any", Content: "c"}, claims, "1.2.3.4")
	if !kratoserrors.IsTooManyRequests(err) {
		t.Fatalf("Submit(denied) error = %v, want too many requests", err)
	}
	if limiter.calls != 1 {
		t.Fatalf("limiter calls = %d, want 1", limiter.calls)
	}
}

func TestCommentUsecaseModerationFlow(t *testing.T) {
	uc, _, _, claims := newCommentTestUsecase(t, &allowAllLimiter{})
	ctx := context.Background()
	publishCommentableArticle(t, uc.articles)

	submitted, err := uc.Submit(ctx, &Comment{ArticleSlug: "commentable", Content: "c"}, claims, "ip")
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
	visible, err := uc.ListPublic(ctx, "commentable", 20, 0)
	if err != nil || len(visible) != 1 {
		t.Fatalf("ListPublic after approval = (%d comments, %v), want 1", len(visible), err)
	}
	if err := uc.Delete(ctx, submitted.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	visible, err = uc.ListPublic(ctx, "commentable", 20, 0)
	if err != nil || len(visible) != 0 {
		t.Fatalf("ListPublic after delete = (%d comments, %v), want 0", len(visible), err)
	}
	if err := uc.Delete(ctx, submitted.ID); !kratoserrors.IsNotFound(err) {
		t.Fatalf("Delete(again) error = %v, want not found", err)
	}
}
