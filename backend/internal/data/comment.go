package data

import (
	"context"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/comment"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatcommentcheck"

	"github.com/go-kratos/aip-go/ents"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// toBizComment converts a persisted comment row into the domain
// representation. The status column is bound to biz.CommentStatus; user_id
// is NULL for legacy anonymous rows.
func toBizComment(po *ent.Comment) *biz.Comment {
	if po == nil {
		return nil
	}
	return &biz.Comment{
		ParentID:     po.ParentID,
		ReplyVersion: po.ReplyVersion,
		ID:           po.ID,
		ArticleSlug:  po.ArticleSlug,
		DisplayName:  po.DisplayName,
		Content:      po.Content,
		Status:       po.Status,
		UserID:       po.UserID,
		CreatedAt:    po.CreatedAt,
		UpdatedAt:    po.UpdatedAt,
	}
}

type commentRepo struct {
	data *Data
}

// NewCommentRepo creates a new CommentRepo instance.
func NewCommentRepo(data *Data) biz.CommentRepo {
	return &commentRepo{data: data}
}

func (r *commentRepo) Create(ctx context.Context, c *biz.Comment) (*biz.Comment, error) {
	create := r.data.db.Comment.Create()
	// The usecase passes the zero id and lets the schema default mint a
	// UUIDv7; an explicit id (tests, imports) is honored when present.
	if c.ID != uuid.Nil {
		create = create.SetID(c.ID)
	}
	// Legacy-style rows without an author stay NULL; account comments carry
	// the authoring account.
	if c.UserID != nil {
		create = create.SetUserID(*c.UserID)
	}
	po, err := create.
		SetArticleSlug(c.ArticleSlug).
		SetDisplayName(c.DisplayName).
		SetContent(c.Content).
		SetStatus(c.Status).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return toBizComment(po), nil
}
func (r *commentRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.Comment, error) {
	po, err := r.data.db.Comment.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrCommentNotFound
		}
		return nil, err
	}
	return toBizComment(po), nil
}

func (r *commentRepo) ListApprovedBySlug(ctx context.Context, slug string, limit, offset int) ([]*biz.Comment, error) {
	pos, err := r.data.db.Comment.Query().
		Where(
			comment.ParentIDIsNil(),
			comment.ArticleSlugEQ(slug),
			comment.StatusEQ(biz.CommentStatusApproved),
		).
		Order(ent.Desc(comment.FieldCreatedAt), ent.Desc(comment.FieldID)).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*biz.Comment, 0, len(pos))
	for _, po := range pos {
		out = append(out, toBizComment(po))
	}
	if err := r.attachReplies(ctx, out, true); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *commentRepo) List(ctx context.Context, opts ...biz.CommentListOption) ([]*biz.Comment, error) {
	o := &biz.CommentListOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if o.Limit <= 0 {
		o.Limit = 20
	}
	pos, err := r.data.db.Comment.Query().
		Where(comment.ParentIDIsNil(), ents.ApplyFilter(o.Filter.Filter)).
		Order(ent.Desc(comment.FieldCreatedAt), ent.Desc(comment.FieldID)).
		Limit(o.Limit).
		Offset(o.Offset).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*biz.Comment, 0, len(pos))
	for _, po := range pos {
		out = append(out, toBizComment(po))
	}
	if err := r.attachReplies(ctx, out, false); err != nil {
		return nil, err
	}
	return out, nil
}

// Approve moves a comment to approved. An already-approved comment returns
// as-is: the operation is idempotent rather than a second write.
func (r *commentRepo) Approve(ctx context.Context, id uuid.UUID) (*biz.Comment, error) {
	current, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Status == biz.CommentStatusApproved {
		return current, nil
	}
	po, err := r.data.db.Comment.UpdateOneID(id).
		SetStatus(biz.CommentStatusApproved).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return toBizComment(po), nil
}

// Delete removes a comment row permanently; comments are not soft-deleted
// because they carry no history worth retaining.
func (r *commentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return biz.ErrContentSafetyUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	db := tx.Client()
	current, err := lockMiniComment(ctx, db, id)
	if err != nil {
		return miniDataError(err)
	}
	ids := []uuid.UUID{id}
	if current.ParentID == nil {
		children, e := db.Comment.Query().Where(comment.ParentIDEQ(id)).All(ctx)
		if e != nil {
			return biz.ErrContentSafetyUnavailable
		}
		for _, child := range children {
			if _, e = lockMiniComment(ctx, db, child.ID); e != nil {
				return miniDataError(e)
			}
			ids = append(ids, child.ID)
		}
	}
	_, err = db.WechatCommentCheck.Update().Where(wechatcommentcheck.CommentIDIn(ids...), wechatcommentcheck.ClientRequestIDNotNil()).SetState(string(biz.MiniCommentDeleted)).ClearIdentityID().SetInputHash("").SetNameTraceID("").SetContentTraceID("").ClearCheckedAt().SetErrorKind("").ClearLeaseUntil().AddGeneration(1).Save(ctx)
	if err != nil {
		return biz.ErrContentSafetyUnavailable
	}
	if _, err = db.WechatCommentCheck.Delete().Where(wechatcommentcheck.CommentIDIn(ids...), wechatcommentcheck.ClientRequestIDIsNil()).Exec(ctx); err != nil {
		return biz.ErrContentSafetyUnavailable
	}
	if len(ids) > 1 {
		if _, err = db.Comment.Delete().Where(comment.ParentIDEQ(id)).Exec(ctx); err != nil {
			return miniDataError(err)
		}
	}
	if err = db.Comment.DeleteOneID(id).Exec(ctx); err != nil {
		return miniDataError(err)
	}
	if err = tx.Commit(); err != nil {
		return biz.ErrContentSafetyUnavailable
	}
	return nil
}

// NewCommentRateLimiter builds the account-comment throttler from the auth
// config, applying the same defaults pattern as the login throttler.
func NewCommentRateLimiter(rdb redis.UniversalClient, a *conf.Auth) biz.CommentRateLimiter {
	attempts := a.GetRateLimit().GetCommentAttempts()
	if attempts <= 0 {
		attempts = biz.DefaultCommentAttempts
	}
	window := a.GetRateLimit().GetCommentWindow().AsDuration()
	if window <= 0 {
		window = biz.DefaultCommentWindow
	}
	return &rateLimiter{rdb: rdb, attempts: attempts, window: window}
}

// NewCommentModerationPolicy maps runtime settings into the domain policy.
func NewCommentModerationPolicy(a *conf.Auth) biz.CommentModerationPolicy {
	return biz.CommentModerationPolicy{Mode: a.GetCommentModeration().GetMode(), ReviewKeywords: a.GetCommentModeration().GetReviewKeywords()}
}
