package service

import (
	"context"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/google/uuid"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
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
	svc := NewCommentService(biz.NewCommentUsecase(repo, nil, nil))

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
