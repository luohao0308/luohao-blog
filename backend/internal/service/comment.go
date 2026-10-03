package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/google/uuid"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CommentService is a comment service.
type CommentService struct {
	v1.UnimplementedCommentServiceServer

	uc *biz.CommentUsecase
}

// NewCommentService new a comment service.
func NewCommentService(uc *biz.CommentUsecase) *CommentService {
	return &CommentService{uc: uc}
}

// CreateComment submits a visitor comment for moderation. The client IP
// (same derivation as the login throttler) feeds the per-IP budget.
func (s *CommentService) CreateComment(ctx context.Context, req *v1.CreateCommentRequest) (*v1.Comment, error) {
	comment, err := s.uc.Submit(ctx, &biz.Comment{
		ArticleSlug: req.GetArticleSlug(),
		DisplayName: req.GetDisplayName(),
		Content:     req.GetContent(),
	}, clientIP(ctx))
	if err != nil {
		return nil, err
	}
	return convertCommentReply(comment), nil
}

// ListArticleComments returns approved comments of an article, newest first.
func (s *CommentService) ListArticleComments(ctx context.Context, req *v1.ListArticleCommentsRequest) (*v1.CommentSet, error) {
	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		// Same treatment as the admin list below: AIP parse errors are plain
		// errors that Kratos would map to 500; the contract promises 400.
		return nil, biz.ErrCommentInvalidArgument.WithCause(err)
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	clampPageSize(&req.PageSize)
	comments, err := s.uc.ListPublic(ctx, req.GetSlug(), int(req.PageSize), int(pageToken.Offset))
	if err != nil {
		return nil, err
	}
	set := &v1.CommentSet{
		Comments: make([]*v1.Comment, 0, len(comments)),
	}
	if len(comments) >= int(req.PageSize) {
		set.NextPageToken = pageToken.Next(req).String()
	}
	for _, c := range comments {
		set.Comments = append(set.Comments, convertCommentReply(c))
	}
	return set, nil
}

// ListComments returns comments in any moderation state for the admin panel.
// The status filter compiles into SQL the same way the article list filter
// does; unknown fields fail ParseFilter before touching the repo.
func (s *CommentService) ListComments(ctx context.Context, req *v1.ListCommentsRequest) (*v1.CommentSet, error) {
	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("status", filtering.TypeInt),
	)
	if err != nil {
		return nil, err
	}
	// The AIP parsers return plain errors, which Kratos would map to 500;
	// wrap them so a malformed list argument surfaces as the documented
	// INVALID_ARGUMENT (same treatment as the article list).
	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return nil, biz.ErrCommentInvalidArgument.WithCause(err)
	}
	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, biz.ErrCommentInvalidArgument.WithCause(err)
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	clampPageSize(&req.PageSize)
	comments, err := s.uc.ListAdmin(ctx,
		biz.CommentListFilter(biz.CommentFilter{Filter: filter}),
		biz.CommentLimit(int(req.PageSize)),
		biz.CommentOffset(int(pageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	set := &v1.CommentSet{
		Comments: make([]*v1.Comment, 0, len(comments)),
	}
	if len(comments) >= int(req.PageSize) {
		set.NextPageToken = pageToken.Next(req).String()
	}
	for _, c := range comments {
		set.Comments = append(set.Comments, convertCommentReply(c))
	}
	return set, nil
}

// ApproveComment moves a comment to approved.
func (s *CommentService) ApproveComment(ctx context.Context, req *v1.ApproveCommentRequest) (*v1.Comment, error) {
	id, err := parseCommentID(req.GetId())
	if err != nil {
		return nil, err
	}
	comment, err := s.uc.Approve(ctx, id)
	if err != nil {
		return nil, err
	}
	return convertCommentReply(comment), nil
}

// DeleteComment removes a comment permanently.
func (s *CommentService) DeleteComment(ctx context.Context, req *v1.DeleteCommentRequest) (*emptypb.Empty, error) {
	id, err := parseCommentID(req.GetId())
	if err != nil {
		return nil, err
	}
	if err := s.uc.Delete(ctx, id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// parseCommentID validates the client-supplied comment identifier.
func parseCommentID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, biz.ErrCommentInvalidArgument
	}
	return id, nil
}

// convertCommentReply maps a domain comment onto the api representation.
func convertCommentReply(in *biz.Comment) *v1.Comment {
	if in == nil {
		return nil
	}
	return &v1.Comment{
		Id:          in.ID.String(),
		ArticleSlug: in.ArticleSlug,
		DisplayName: in.DisplayName,
		Content:     in.Content,
		Status:      convertCommentStatus(in.Status),
		CreatedAt:   timestamppb.New(in.CreatedAt),
		UpdatedAt:   timestamppb.New(in.UpdatedAt),
	}
}

// convertCommentStatus maps a domain status onto the api enum. The two share
// their numeric values, but the mapping is written out so neither side can
// drift into the other silently.
func convertCommentStatus(in biz.CommentStatus) v1.CommentStatus {
	switch in {
	case biz.CommentStatusPending:
		return v1.CommentStatus_COMMENT_STATUS_PENDING
	case biz.CommentStatusApproved:
		return v1.CommentStatus_COMMENT_STATUS_APPROVED
	default:
		return v1.CommentStatus_COMMENT_STATUS_UNSPECIFIED
	}
}
