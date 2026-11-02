package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"go.einride.tech/aip/pagination"
)

func miniCommentPage(req pagination.Request, size *int32) (pagination.PageToken, error) {
	if *size <= 0 {
		*size = defaultPageSize
	}
	clampPageSize(size)
	token, err := pagination.ParsePageToken(req)
	if err != nil || !pageOffsetWithinWindow(token.Offset) {
		return pagination.PageToken{}, biz.ErrCommentInvalidArgument
	}
	return token, nil
}

func (s *CommentService) ListMiniArticleComments(ctx context.Context, req *v1.ListMiniArticleCommentsRequest) (*v1.CommentSet, error) {
	if req == nil {
		return nil, biz.ErrCommentInvalidArgument
	}
	token, err := miniCommentPage(req, &req.PageSize)
	if err != nil {
		return nil, err
	}
	result := &v1.CommentSet{Comments: []*v1.Comment{}}
	if s.mini == nil {
		return result, nil
	}
	rows, next, err := s.mini.ListPublic(ctx, req.GetSlug(), int(req.PageSize), int(token.Offset))
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		c := convertCommentReply(r.Comment)
		c.AvatarUrl = ""
		if c.AuthorReply != nil {
			c.AuthorReply.AvatarUrl = ""
			c.AuthorReply.AuthorReply = nil
		}
		result.Comments = append(result.Comments, c)
	}
	if next >= 0 && pageOffsetWithinWindow(int64(next)) {
		cursor := token.Next(req)
		cursor.Offset = int64(next)
		result.NextPageToken = cursor.String()
	}
	return result, nil
}

func (s *CommentService) CreateMiniComment(ctx context.Context, req *v1.CreateMiniCommentRequest) (*v1.MiniCommentRecord, error) {
	claims, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	if s.mini == nil {
		return nil, biz.ErrContentSafetyDisabled
	}
	r, err := s.mini.Create(ctx, claims, req.GetArticleSlug(), req.GetContent(), req.GetWechatCode(), req.GetClientRequestId(), clientIP(ctx))
	if err != nil {
		return nil, err
	}
	return s.convertMiniComment(r), nil
}

func (s *CommentService) ListMyMiniArticleComments(ctx context.Context, req *v1.ListMyMiniArticleCommentsRequest) (*v1.MiniCommentRecords, error) {
	claims, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	if req == nil {
		return nil, biz.ErrCommentInvalidArgument
	}
	token, err := miniCommentPage(req, &req.PageSize)
	if err != nil {
		return nil, err
	}
	if s.mini == nil {
		return nil, biz.ErrContentSafetyDisabled
	}
	rows, next, err := s.mini.ListMine(ctx, claims, req.GetSlug(), int(req.PageSize), int(token.Offset))
	if err != nil {
		return nil, err
	}
	result := &v1.MiniCommentRecords{Records: []*v1.MiniCommentRecord{}}
	for _, r := range rows {
		result.Records = append(result.Records, s.convertMiniComment(r))
	}
	if next >= 0 && pageOffsetWithinWindow(int64(next)) {
		cursor := token.Next(req)
		cursor.Offset = int64(next)
		result.NextPageToken = cursor.String()
	}
	return result, nil
}

func (s *CommentService) RecheckMiniComment(ctx context.Context, req *v1.RecheckMiniCommentRequest) (*v1.MiniCommentRecord, error) {
	claims, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	if s.mini == nil {
		return nil, biz.ErrContentSafetyDisabled
	}
	id, err := parseCommentID(req.GetId())
	if err != nil {
		return nil, err
	}
	r, err := s.mini.Recheck(ctx, claims, id, req.GetWechatCode(), clientIP(ctx))
	if err != nil {
		return nil, err
	}
	return s.convertMiniComment(r), nil
}

func (s *CommentService) convertMiniComment(r *biz.MiniComment) *v1.MiniCommentRecord {
	if r == nil || r.Comment == nil {
		return nil
	}
	c := convertCommentReply(r.Comment)
	c.AvatarUrl = ""
	if c.AuthorReply != nil {
		c.AuthorReply.AvatarUrl = ""
		c.AuthorReply.AuthorReply = nil
	}
	state := v1.MiniCommentSafetyState_MINI_COMMENT_SAFETY_UNKNOWN
	switch r.State {
	case biz.MiniCommentChecking:
		state = v1.MiniCommentSafetyState_MINI_COMMENT_SAFETY_CHECKING
	case biz.MiniCommentPass:
		state = v1.MiniCommentSafetyState_MINI_COMMENT_SAFETY_PASS
	case biz.MiniCommentReview:
		state = v1.MiniCommentSafetyState_MINI_COMMENT_SAFETY_REVIEW
	case biz.MiniCommentRisky:
		state = v1.MiniCommentSafetyState_MINI_COMMENT_SAFETY_RISKY
	case biz.MiniCommentUnavailable:
		state = v1.MiniCommentSafetyState_MINI_COMMENT_SAFETY_UNAVAILABLE
	case biz.MiniCommentDeleted:
		state = v1.MiniCommentSafetyState_MINI_COMMENT_SAFETY_DELETED
	}
	errorKind := r.ErrorKind
	switch errorKind {
	case "", "identity", "unavailable", "disabled", "input":
	default:
		errorKind = "unavailable"
	}
	return &v1.MiniCommentRecord{Comment: c, SafetyState: state, Visible: r.Visible(), CanRecheck: s.mini.CanRecheck(r), SafetyError: errorKind}
}
