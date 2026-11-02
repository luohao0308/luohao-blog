package biz

import (
	"context"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	"strings"
)

var ErrAuthorReplyConflict = errors.Conflict("AUTHOR_REPLY_CONFLICT", "回复已变化，请重新加载后再保存")

type AuthorReplyRepo interface {
	GetAuthorReply(context.Context, uuid.UUID) (*Comment, error)
	PutAuthorReply(context.Context, uuid.UUID, *Comment, uint64) (*Comment, error)
}

func (uc *CommentUsecase) GetAuthorReply(ctx context.Context, id uuid.UUID) (*Comment, error) {
	claims, ok := AuthFromContext(ctx)
	if !ok {
		return nil, ErrAuthUnauthorized
	}
	u, e := uc.users.ByID(ctx, claims.UserID)
	if e != nil || u == nil || u.Role != UserRoleAdmin {
		return nil, ErrAuthForbidden
	}
	r, ok := uc.repo.(AuthorReplyRepo)
	if !ok {
		return nil, ErrContentSafetyUnavailable
	}
	return r.GetAuthorReply(ctx, id)
}
func (uc *CommentUsecase) PutAuthorReply(ctx context.Context, id uuid.UUID, body string, version uint64, ip string) (*Comment, error) {
	claims, ok := AuthFromContext(ctx)
	if !ok {
		return nil, ErrAuthUnauthorized
	}
	u, e := uc.users.ByID(ctx, claims.UserID)
	if e != nil || u == nil || u.Role != UserRoleAdmin {
		return nil, ErrAuthForbidden
	}
	parent, e := uc.repo.FindByID(ctx, id)
	if e != nil {
		return nil, e
	}
	if parent.ParentID != nil {
		return nil, ErrCommentInvalidArgument
	}
	if _, e = uc.articles.GetPublicArticle(ctx, parent.ArticleSlug); e != nil {
		return nil, e
	}
	c := &Comment{ArticleSlug: parent.ArticleSlug, DisplayName: u.DisplayName, UserID: &u.ID, Content: strings.TrimSpace(body), ParentID: &id}
	if r, ok := uc.repo.(AuthorReplyRepo); ok && version > 0 {
		old, err := r.GetAuthorReply(ctx, id)
		if err != nil {
			return nil, err
		}
		c.DisplayName = old.DisplayName
	}
	if e = validateComment(c); e != nil {
		return nil, e
	}
	c.Status = uc.moderation.status(c.DisplayName, c.Content)
	if uc.moderation.automatic() && uc.limiter == nil {
		return nil, ErrContentSafetyUnavailable
	}
	if uc.limiter != nil {
		keys := []string{"comment:user:" + u.ID.String()}
		if ip != "" {
			keys = append(keys, "comment:"+ip)
		}
		for _, key := range keys {
			allowed, err := uc.limiter.Allow(ctx, key)
			if err != nil {
				if uc.moderation.automatic() {
					return nil, ErrContentSafetyUnavailable
				}
				c.Status = CommentStatusPending
			} else if !allowed {
				return nil, ErrCommentTooManyAttempts
			}
		}
	}
	if uc.moderation.automatic() && c.Status != CommentStatusApproved {
		return nil, ErrCommentRejected
	}
	r, ok := uc.repo.(AuthorReplyRepo)
	if !ok {
		return nil, ErrContentSafetyUnavailable
	}
	return r.PutAuthorReply(ctx, id, c, version)
}
