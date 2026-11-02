package data

import (
	"context"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/comment"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/user"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatcommentcheck"
)

func (r *commentRepo) GetAuthorReply(ctx context.Context, id uuid.UUID) (*biz.Comment, error) {
	root, e := r.data.db.Comment.Get(ctx, id)
	if e != nil {
		return nil, miniDataError(e)
	}
	if root.ParentID != nil {
		return nil, biz.ErrCommentInvalidArgument
	}
	c, e := r.data.db.Comment.Query().Where(comment.ParentIDEQ(id)).Only(ctx)
	if e != nil {
		return nil, miniDataError(e)
	}
	return toBizComment(c), nil
}
func (r *commentRepo) PutAuthorReply(ctx context.Context, id uuid.UUID, c *biz.Comment, version uint64) (*biz.Comment, error) {
	if c == nil || c.UserID == nil {
		return nil, biz.ErrAuthForbidden
	}
	tx, e := r.data.db.Tx(ctx)
	if e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	db := tx.Client()
	author, e := db.User.Query().Where(user.IDEQ(*c.UserID)).Where(func(s *entsql.Selector) {
		if s.Dialect() == dialect.MySQL {
			s.ForUpdate()
		}
	}).Only(ctx)
	if e != nil || author.Role != biz.UserRoleAdmin {
		return nil, biz.ErrAuthForbidden
	}
	root, e := lockMiniComment(ctx, db, id)
	if e != nil {
		return nil, miniDataError(e)
	}
	if root.ParentID != nil {
		return nil, biz.ErrCommentInvalidArgument
	}
	current, e := db.Comment.Query().Where(comment.ParentIDEQ(id)).Only(ctx)
	if e != nil && !ent.IsNotFound(e) {
		return nil, biz.ErrContentSafetyUnavailable
	}
	var out *ent.Comment
	if ent.IsNotFound(e) {
		if author.DisplayName != c.DisplayName {
			return nil, biz.ErrAuthorReplyConflict
		}
		if version != 0 {
			return nil, biz.ErrAuthorReplyConflict
		}
		out, e = db.Comment.Create().SetParentID(id).SetReplyVersion(1).SetArticleSlug(root.ArticleSlug).SetUserID(author.ID).SetDisplayName(author.DisplayName).SetContent(c.Content).SetStatus(c.Status).Save(ctx)
	} else {
		current, e = lockMiniComment(ctx, db, current.ID)
		if e != nil {
			return nil, miniDataError(e)
		}
		if current.UserID == nil || *current.UserID != author.ID {
			return nil, biz.ErrAuthForbidden
		}
		if current.ReplyVersion != version {
			return nil, biz.ErrAuthorReplyConflict
		}
		if current.Content == c.Content && current.Status == c.Status {
			out = current
		} else {
			n, err := db.Comment.Update().Where(comment.IDEQ(current.ID), comment.ReplyVersionEQ(version)).SetContent(c.Content).SetStatus(c.Status).AddReplyVersion(1).Save(ctx)
			if err != nil {
				return nil, biz.ErrContentSafetyUnavailable
			}
			if n != 1 {
				return nil, biz.ErrAuthorReplyConflict
			}
			_, e = db.WechatCommentCheck.Update().Where(wechatcommentcheck.CommentIDEQ(current.ID)).SetState(string(biz.MiniCommentUnknown)).AddGeneration(1).ClearIdentityID().SetInputHash("").SetNameTraceID("").SetContentTraceID("").ClearCheckedAt().ClearLeaseUntil().SetErrorKind("").Save(ctx)
			if e != nil {
				return nil, biz.ErrContentSafetyUnavailable
			}
			out, e = db.Comment.Get(ctx, current.ID)
		}
	}
	if e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	if e = tx.Commit(); e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	return toBizComment(out), nil
}
func (r *commentRepo) attachReplies(ctx context.Context, roots []*biz.Comment, public bool) error {
	ids := make([]uuid.UUID, 0, len(roots))
	by := map[uuid.UUID]*biz.Comment{}
	for _, c := range roots {
		ids = append(ids, c.ID)
		by[c.ID] = c
	}
	if len(ids) == 0 {
		return nil
	}
	q := r.data.db.Comment.Query().Where(comment.ParentIDIn(ids...))
	if public {
		q = q.Where(comment.StatusEQ(biz.CommentStatusApproved))
	}
	rows, e := q.All(ctx)
	if e != nil {
		return e
	}
	for _, c := range rows {
		if root := by[*c.ParentID]; root != nil && (!public || root.Status == biz.CommentStatusApproved) {
			root.AuthorReply = toBizComment(c)
		}
	}
	return nil
}
