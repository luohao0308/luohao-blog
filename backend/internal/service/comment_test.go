package service

import (
	"context"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

// stubCommentRepo records admin list options; the other methods are out of
// scope for the list tests and fail loudly if reached.
type stubCommentRepo struct {
	listOpts []biz.CommentListOption
}

func (s *stubCommentRepo) Create(context.Context, *biz.Comment) (*biz.Comment, error) {
	return nil, biz.ErrCommentInvalidArgument
}
func (s *stubCommentRepo) FindByID(context.Context, uuid.UUID) (*biz.Comment, error) {
	return nil, biz.ErrCommentInvalidArgument
}
func (s *stubCommentRepo) ListApprovedBySlug(context.Context, string, int, int) ([]*biz.Comment, error) {
	return nil, biz.ErrCommentInvalidArgument
}
func (s *stubCommentRepo) List(_ context.Context, opts ...biz.CommentListOption) ([]*biz.Comment, error) {
	s.listOpts = opts
	return nil, nil
}
func (s *stubCommentRepo) Approve(context.Context, uuid.UUID) (*biz.Comment, error) {
	return nil, biz.ErrCommentInvalidArgument
}
func (s *stubCommentRepo) Delete(context.Context, uuid.UUID) error {
	return biz.ErrCommentInvalidArgument
}

// The documented status filter (status = 1, an int literal) must reach the
// usecase, while malformed list arguments surface as INVALID_ARGUMENT: the
// AIP parse errors are plain errors that Kratos would otherwise map to 500.
func TestListCommentsFilterHandling(t *testing.T) {
	ctx := context.Background()
	repo := &stubCommentRepo{}
	svc := NewCommentService(biz.NewCommentUsecase(repo, nil, nil, nil))

	t.Run("documented status filter reaches the repo", func(t *testing.T) {
		repo.listOpts = nil
		if _, err := svc.ListComments(ctx, &v1.ListCommentsRequest{Filter: "status = 1"}); err != nil {
			t.Fatalf("ListComments(status filter) error = %v, want success", err)
		}
		var opts biz.CommentListOptions
		for _, opt := range repo.listOpts {
			opt(&opts)
		}
		if opts.Filter.Filter.CheckedExpr == nil {
			t.Fatal("parsed filter did not reach the repo")
		}
	})
	t.Run("malformed filter is invalid argument", func(t *testing.T) {
		_, err := svc.ListComments(ctx, &v1.ListCommentsRequest{Filter: `status:`})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("malformed filter error = %v, want bad request", err)
		}
	})
	t.Run("string literal against int status is invalid argument", func(t *testing.T) {
		_, err := svc.ListComments(ctx, &v1.ListCommentsRequest{Filter: `status:"1"`})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("type mismatch error = %v, want bad request", err)
		}
	})
	t.Run("undeclared filter field is invalid argument", func(t *testing.T) {
		_, err := svc.ListComments(ctx, &v1.ListCommentsRequest{Filter: `bogus = 1`})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("undeclared field error = %v, want bad request", err)
		}
	})
	t.Run("malformed page token is invalid argument", func(t *testing.T) {
		_, err := svc.ListComments(ctx, &v1.ListCommentsRequest{PageToken: "@@@@"})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("malformed page token error = %v, want bad request", err)
		}
	})
}

// The policy layer 401s anonymous callers before the handler runs; this
// pins the handler-level defense in depth: no claims in the context means
// unauthorized, and claims from biz.NewAuthContext pass the gate (the stub
// repo's failure proves the request got past identity checks).
func TestCreateCommentRequiresIdentity(t *testing.T) {
	ctx := context.Background()
	repo := &stubCommentRepo{}
	svc := NewCommentService(biz.NewCommentUsecase(repo, nil, nil, nil))

	t.Run("no claims is unauthorized", func(t *testing.T) {
		_, err := svc.CreateComment(ctx, &v1.CreateCommentRequest{ArticleSlug: "post", Content: "c"})
		if !kratoserrors.IsUnauthorized(err) {
			t.Fatalf("CreateComment(anonymous) error = %v, want unauthorized", err)
		}
	})
	t.Run("claims pass the gate", func(t *testing.T) {
		// A working user repo (identity resolution runs before the repo
		// write), so any failure past the gate is the stub's invalid
		// argument — anything but unauthorized.
		author := &biz.User{ID: uuid.Must(uuid.NewV7()), Email: "author@example.com", DisplayName: "罗豪", Role: biz.UserRoleAdmin}
		users := biz.NewUserUsecase(newStubUserRepo(author))
		articles := biz.NewArticleUsecase(gateArticleRepo{}, nil)
		authedSvc := NewCommentService(biz.NewCommentUsecase(repo, articles, users, nil))
		authed := biz.NewAuthContext(ctx, &biz.Claims{UserID: author.ID, Role: author.Role})
		_, err := authedSvc.CreateComment(authed, &v1.CreateCommentRequest{ArticleSlug: "post", Content: "c"})
		if kratoserrors.IsUnauthorized(err) {
			t.Fatalf("CreateComment(authenticated) = unauthorized, want past the identity gate")
		}
	})
}

// newStubUserRepo serves exactly the seeded author and fails loudly
// elsewhere; the CreateComment gate test only exercises the read path.
func newStubUserRepo(author *biz.User) biz.UserRepository {
	return &stubUserRepo{author: author}
}

// gateArticleRepo serves one published article for the gate test's
// commentability check and fails loudly everywhere else.
type gateArticleRepo struct{}

func (gateArticleRepo) FindBySlug(_ context.Context, slug string) (*biz.Article, error) {
	if slug == "post" {
		return &biz.Article{Slug: slug, Title: "t", Status: biz.ArticleStatusPublished}, nil
	}
	return nil, biz.ErrArticleNotFound
}

func (gateArticleRepo) ListArticles(context.Context, ...biz.ListOption) ([]*biz.Article, error) {
	return nil, biz.ErrArticleNotFound
}

func (gateArticleRepo) CreateArticle(context.Context, *biz.Article) (*biz.Article, error) {
	return nil, biz.ErrArticleNotFound
}

func (gateArticleRepo) UpdateArticle(context.Context, *biz.Article) (*biz.Article, error) {
	return nil, biz.ErrArticleNotFound
}

func (gateArticleRepo) DeleteArticle(context.Context, string) error {
	return biz.ErrArticleNotFound
}

func (gateArticleRepo) IncrementView(context.Context, string, string) (uint64, bool, error) {
	return 0, false, biz.ErrArticleNotFound
}

func (gateArticleRepo) IncrementLike(context.Context, string, string) (uint64, bool, error) {
	return 0, false, biz.ErrArticleNotFound
}

type stubUserRepo struct{ author *biz.User }

func (s *stubUserRepo) FindByEmail(_ context.Context, email string) (*biz.User, error) {
	if s.author.Email == email {
		return s.author, nil
	}
	return nil, biz.ErrUserNotFound
}

func (s *stubUserRepo) FindByID(_ context.Context, id uuid.UUID) (*biz.User, error) {
	if s.author.ID == id {
		return s.author, nil
	}
	return nil, biz.ErrUserNotFound
}

func (s *stubUserRepo) FindByIDs(_ context.Context, ids []uuid.UUID) ([]*biz.User, error) {
	for _, id := range ids {
		if s.author.ID == id {
			return []*biz.User{s.author}, nil
		}
	}
	return nil, nil
}

func (s *stubUserRepo) Create(context.Context, *biz.User) (*biz.User, error) {
	return nil, biz.ErrUserInvalidArgument
}

func (s *stubUserRepo) UpdatePassword(context.Context, uuid.UUID, string) error {
	return biz.ErrUserNotFound
}

func (s *stubUserRepo) UpdateProfile(context.Context, uuid.UUID, string) error {
	return biz.ErrUserNotFound
}

func (s *stubUserRepo) UpdateAvatar(context.Context, uuid.UUID, string) error {
	return biz.ErrUserNotFound
}
