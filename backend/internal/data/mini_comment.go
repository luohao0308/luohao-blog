package data

import (
	"context"
	"errors"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/comment"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatcommentcheck"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatidentity"
)

type miniCommentRepo struct{ data *Data }

func NewMiniCommentRepo(d *Data) biz.MiniCommentRepo { return &miniCommentRepo{data: d} }
func miniDataError(err error) error {
	if ent.IsNotFound(err) {
		return biz.ErrCommentNotFound
	}
	return biz.ErrContentSafetyUnavailable
}
func lockMiniComment(ctx context.Context, db *ent.Client, id uuid.UUID) (*ent.Comment, error) {
	return db.Comment.Query().Where(comment.IDEQ(id)).Where(func(s *entsql.Selector) {
		if s.Dialect() == dialect.MySQL {
			s.ForUpdate()
		}
	}).Only(ctx)
}
func miniIdentity(ctx context.Context, db *ent.Client, a biz.MiniCommentActor) (*ent.WechatIdentity, error) {
	i, e := db.WechatIdentity.Query().Where(wechatidentity.UserIDEQ(a.UserID), wechatidentity.AppIDEQ(a.AppID), wechatidentity.OpenidEQ(a.OpenID)).Select(wechatidentity.FieldID, wechatidentity.FieldUserID, wechatidentity.FieldAppID).Only(ctx)
	if ent.IsNotFound(e) {
		return nil, biz.ErrContentSafetyIdentity
	}
	if e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	return i, nil
}
func miniProjectionWithIdentity(c *ent.Comment, check *ent.WechatCommentCheck, identity *ent.WechatIdentity) *biz.MiniComment {
	out := &biz.MiniComment{Comment: toBizComment(c), State: biz.MiniCommentUnknown}
	if check == nil {
		return out
	}
	out.AppID = check.AppID
	out.InputHash = check.InputHash
	out.State = biz.MiniCommentState(check.State)
	out.Generation = check.Generation
	if check.LeaseUntil != nil {
		out.LeaseUntil = *check.LeaseUntil
	}
	out.NameTraceID = check.NameTraceID
	out.ContentTraceID = check.ContentTraceID
	out.ErrorKind = check.ErrorKind
	if check.IdentityID == nil || c.UserID == nil {
		return invalidateMiniVerdict(out)
	}
	out.IdentityID = *check.IdentityID
	if identity == nil {
		return invalidateMiniVerdict(out)
	}
	out.FingerprintValid = identity.UserID == *c.UserID && identity.UserID == check.AuthorID && identity.AppID == check.AppID && biz.MiniCommentInputHash(out.Comment, check.AppID, *check.IdentityID) == check.InputHash
	if out.State == biz.MiniCommentPass && (!validSafetyTrace(out.NameTraceID) || !validSafetyTrace(out.ContentTraceID)) {
		out.FingerprintValid = false
	}
	return invalidateMiniVerdict(out)
}

func invalidateMiniVerdict(record *biz.MiniComment) *biz.MiniComment {
	if !record.FingerprintValid && (record.State == biz.MiniCommentPass || record.State == biz.MiniCommentReview || record.State == biz.MiniCommentRisky) {
		record.State = biz.MiniCommentUnknown
	}
	return record
}
func miniProjection(ctx context.Context, db *ent.Client, c *ent.Comment, check *ent.WechatCommentCheck) (*biz.MiniComment, error) {
	if check == nil || check.IdentityID == nil || c.UserID == nil {
		return miniProjectionWithIdentity(c, check, nil), nil
	}
	identity, err := db.WechatIdentity.Query().Where(wechatidentity.IDEQ(*check.IdentityID)).Select(wechatidentity.FieldID, wechatidentity.FieldUserID, wechatidentity.FieldAppID).Only(ctx)
	if ent.IsNotFound(err) {
		return miniProjectionWithIdentity(c, check, nil), nil
	}
	if err != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	return miniProjectionWithIdentity(c, check, identity), nil
}
func miniBatchProjections(ctx context.Context, db *ent.Client, rows []*ent.Comment, app string) ([]*biz.MiniComment, error) {
	out := make([]*biz.MiniComment, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	checks, err := db.WechatCommentCheck.Query().Where(wechatcommentcheck.CommentIDIn(ids...), wechatcommentcheck.AppIDEQ(app)).All(ctx)
	if err != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	byComment := make(map[uuid.UUID]*ent.WechatCommentCheck, len(checks))
	identityIDs := make(map[uuid.UUID]struct{})
	for _, check := range checks {
		byComment[check.CommentID] = check
		if check.IdentityID != nil {
			identityIDs[*check.IdentityID] = struct{}{}
		}
	}
	byIdentity := make(map[uuid.UUID]*ent.WechatIdentity, len(identityIDs))
	if len(identityIDs) > 0 {
		ids := make([]uuid.UUID, 0, len(identityIDs))
		for id := range identityIDs {
			ids = append(ids, id)
		}
		identities, err := db.WechatIdentity.Query().Where(wechatidentity.IDIn(ids...)).Select(wechatidentity.FieldID, wechatidentity.FieldUserID, wechatidentity.FieldAppID).All(ctx)
		if err != nil {
			return nil, biz.ErrContentSafetyUnavailable
		}
		for _, identity := range identities {
			byIdentity[identity.ID] = identity
		}
	}
	for _, row := range rows {
		check := byComment[row.ID]
		var identity *ent.WechatIdentity
		if check != nil && check.IdentityID != nil {
			identity = byIdentity[*check.IdentityID]
		}
		item := miniProjectionWithIdentity(row, check, identity)
		if item.AppID == "" {
			item.AppID = app
		}
		out = append(out, item)
	}
	return out, nil
}
func findMiniCheck(ctx context.Context, db *ent.Client, id uuid.UUID, app string) (*ent.WechatCommentCheck, error) {
	row, e := db.WechatCommentCheck.Query().Where(wechatcommentcheck.CommentIDEQ(id), wechatcommentcheck.AppIDEQ(app)).Only(ctx)
	if ent.IsNotFound(e) {
		return nil, nil
	}
	return row, e
}
func lockMiniCheck(ctx context.Context, db *ent.Client, id uuid.UUID, app string) (*ent.WechatCommentCheck, error) {
	row, err := db.WechatCommentCheck.Query().Where(wechatcommentcheck.CommentIDEQ(id), wechatcommentcheck.AppIDEQ(app)).Where(func(selector *entsql.Selector) {
		if selector.Dialect() == dialect.MySQL {
			selector.ForUpdate()
		}
	}).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return row, err
}

func (r *miniCommentRepo) FindRequest(ctx context.Context, author uuid.UUID, app, key, hash string) (*biz.MiniComment, error) {
	row, e := r.data.db.WechatCommentCheck.Query().Where(wechatcommentcheck.AuthorIDEQ(author), wechatcommentcheck.AppIDEQ(app), wechatcommentcheck.ClientRequestIDEQ(key)).Only(ctx)
	if e != nil {
		return nil, miniDataError(e)
	}
	if row.CreateRequestHash != hash {
		return nil, biz.ErrMiniCommentConflict
	}
	if row.State == string(biz.MiniCommentDeleted) {
		return nil, biz.ErrMiniCommentDeleted
	}
	c, e := r.data.db.Comment.Get(ctx, row.CommentID)
	if ent.IsNotFound(e) {
		return nil, biz.ErrMiniCommentDeleted
	}
	if e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	if c.UserID == nil || *c.UserID != author {
		return nil, biz.ErrCommentNotFound
	}
	return miniProjection(ctx, r.data.db, c, row)
}
func (r *miniCommentRepo) CreatePending(ctx context.Context, input *biz.Comment, actor biz.MiniCommentActor, key, requestHash string, lease time.Duration) (*biz.MiniComment, bool, error) {
	if input == nil || requestHash != biz.MiniCommentRequestHash(input.ArticleSlug, input.Content) {
		return nil, false, biz.ErrMiniCommentConflict
	}
	if key != "" {
		old, e := r.FindRequest(ctx, actor.UserID, actor.AppID, key, requestHash)
		if e == nil {
			return old, false, nil
		}
		if !errors.Is(e, biz.ErrCommentNotFound) {
			return nil, false, e
		}
	}
	tx, e := r.data.db.Tx(ctx)
	if e != nil {
		return nil, false, biz.ErrContentSafetyUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	db := tx.Client()
	identity, e := miniIdentity(ctx, db, actor)
	if e != nil {
		return nil, false, e
	}
	author, e := db.User.Get(ctx, actor.UserID)
	if e != nil {
		return nil, false, miniDataError(e)
	}
	c, e := db.Comment.Create().SetArticleSlug(input.ArticleSlug).SetDisplayName(author.DisplayName).SetContent(input.Content).SetUserID(actor.UserID).SetStatus(biz.CommentStatusPending).Save(ctx)
	if e != nil {
		return nil, false, biz.ErrContentSafetyUnavailable
	}
	snapshot := toBizComment(c)
	hash := biz.MiniCommentInputHash(snapshot, actor.AppID, identity.ID)
	create := db.WechatCommentCheck.Create().SetCommentID(c.ID).SetAuthorID(actor.UserID).SetAppID(actor.AppID).SetCreateRequestHash(requestHash).SetIdentityID(identity.ID).SetInputHash(hash).SetState(string(biz.MiniCommentChecking)).SetGeneration(1).SetLeaseUntil(time.Now().UTC().Add(lease))
	if key != "" {
		create.SetClientRequestID(key)
	}
	check, e := create.Save(ctx)
	if e != nil {
		_ = tx.Rollback()
		if key != "" && ent.IsConstraintError(e) {
			winner, lookup := r.FindRequest(ctx, actor.UserID, actor.AppID, key, requestHash)
			if lookup == nil || errors.Is(lookup, biz.ErrMiniCommentConflict) || errors.Is(lookup, biz.ErrMiniCommentDeleted) {
				return winner, false, lookup
			}
			return nil, false, biz.ErrContentSafetyUnavailable
		}
		return nil, false, biz.ErrContentSafetyUnavailable
	}
	out, e := miniProjection(ctx, db, c, check)
	if e != nil {
		return nil, false, e
	}
	if e = tx.Commit(); e != nil {
		return nil, false, biz.ErrContentSafetyUnavailable
	}
	return out, true, nil
}
func (r *miniCommentRepo) FindOwned(ctx context.Context, id, owner uuid.UUID, app string) (*biz.MiniComment, error) {
	c, e := r.data.db.Comment.Get(ctx, id)
	if e != nil {
		return nil, miniDataError(e)
	}
	if c.UserID == nil || *c.UserID != owner {
		return nil, biz.ErrCommentNotFound
	}
	check, e := findMiniCheck(ctx, r.data.db, id, app)
	if e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	out, e := miniProjection(ctx, r.data.db, c, check)
	if out != nil && out.AppID == "" {
		out.AppID = app
	}
	return out, e
}
func (r *miniCommentRepo) BeginRecheck(ctx context.Context, id uuid.UUID, actor biz.MiniCommentActor, lease time.Duration) (*biz.MiniComment, bool, error) {
	tx, e := r.data.db.Tx(ctx)
	if e != nil {
		return nil, false, biz.ErrContentSafetyUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	db := tx.Client()
	c, e := lockMiniComment(ctx, db, id)
	if e != nil {
		return nil, false, miniDataError(e)
	}
	if c.UserID == nil || *c.UserID != actor.UserID {
		return nil, false, biz.ErrCommentNotFound
	}
	identity, e := miniIdentity(ctx, db, actor)
	if e != nil {
		return nil, false, e
	}
	check, e := lockMiniCheck(ctx, db, id, actor.AppID)
	if e != nil {
		return nil, false, biz.ErrContentSafetyUnavailable
	}
	current, e := miniProjection(ctx, db, c, check)
	if e != nil {
		return nil, false, e
	}
	if check != nil && current.State == biz.MiniCommentChecking && current.LeaseUntil.After(time.Now()) {
		return nil, false, biz.ErrMiniCommentCheckBusy
	}
	if check != nil && !current.CanRecheck(time.Now()) {
		if e = tx.Commit(); e != nil {
			return nil, false, biz.ErrContentSafetyUnavailable
		}
		return current, false, nil
	}
	hash := biz.MiniCommentInputHash(toBizComment(c), actor.AppID, identity.ID)
	if check == nil {
		check, e = db.WechatCommentCheck.Create().SetCommentID(c.ID).SetAuthorID(actor.UserID).SetAppID(actor.AppID).SetIdentityID(identity.ID).SetInputHash(hash).SetState(string(biz.MiniCommentChecking)).SetGeneration(1).SetLeaseUntil(time.Now().UTC().Add(lease)).Save(ctx)
	} else {
		var affected int
		affected, e = db.WechatCommentCheck.Update().Where(wechatcommentcheck.IDEQ(check.ID), wechatcommentcheck.GenerationEQ(check.Generation)).SetIdentityID(identity.ID).SetInputHash(hash).SetState(string(biz.MiniCommentChecking)).SetGeneration(check.Generation + 1).SetLeaseUntil(time.Now().UTC().Add(lease)).SetNameTraceID("").SetContentTraceID("").ClearCheckedAt().SetErrorKind("").Save(ctx)
		if e == nil && affected != 1 {
			return nil, false, biz.ErrMiniCommentCheckBusy
		}
		if e == nil {
			check, e = db.WechatCommentCheck.Get(ctx, check.ID)
		}
	}
	if e != nil {
		return nil, false, biz.ErrContentSafetyUnavailable
	}
	out, e := miniProjection(ctx, db, c, check)
	if e != nil {
		return nil, false, e
	}
	if e = tx.Commit(); e != nil {
		return nil, false, biz.ErrContentSafetyUnavailable
	}
	return out, true, nil
}
func (r *miniCommentRepo) CompleteCheck(ctx context.Context, input *biz.MiniComment, v biz.MiniCommentVerdict, approve bool) (*biz.MiniComment, error) {
	if input == nil || input.Comment == nil || input.InputHash == "" || biz.MiniCommentInputHash(input.Comment, input.AppID, input.IdentityID) != input.InputHash {
		return nil, biz.ErrMiniCommentConflict
	}
	if v.State != biz.MiniCommentPass && v.State != biz.MiniCommentReview && v.State != biz.MiniCommentRisky && v.State != biz.MiniCommentUnavailable {
		return nil, biz.ErrMiniCommentConflict
	}
	if v.State == biz.MiniCommentPass && (!validSafetyTrace(v.NameTraceID) || !validSafetyTrace(v.ContentTraceID)) {
		return nil, biz.ErrMiniCommentConflict
	}
	tx, e := r.data.db.Tx(ctx)
	if e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	db := tx.Client()
	c, e := lockMiniComment(ctx, db, input.Comment.ID)
	if e != nil {
		return nil, miniDataError(e)
	}
	check, e := lockMiniCheck(ctx, db, c.ID, input.AppID)
	if e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	if check == nil {
		return nil, biz.ErrMiniCommentConflict
	}
	current, e := miniProjection(ctx, db, c, check)
	if e != nil {
		return nil, e
	}
	if !current.FingerprintValid || current.Generation != input.Generation || current.InputHash != input.InputHash || current.IdentityID != input.IdentityID || current.State != biz.MiniCommentChecking || c.UserID == nil || input.Comment.UserID == nil || *c.UserID != *input.Comment.UserID {
		return nil, biz.ErrMiniCommentConflict
	}
	affected, e := db.WechatCommentCheck.Update().Where(wechatcommentcheck.IDEQ(check.ID), wechatcommentcheck.GenerationEQ(input.Generation), wechatcommentcheck.InputHashEQ(input.InputHash), wechatcommentcheck.IdentityIDEQ(input.IdentityID), wechatcommentcheck.AuthorIDEQ(*input.Comment.UserID), wechatcommentcheck.StateEQ(string(biz.MiniCommentChecking))).SetState(string(v.State)).SetNameTraceID(v.NameTraceID).SetContentTraceID(v.ContentTraceID).SetCheckedAt(time.Now().UTC()).ClearLeaseUntil().SetErrorKind(v.ErrorKind).Save(ctx)
	if e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	if affected != 1 {
		return nil, biz.ErrMiniCommentCheckBusy
	}
	if approve && check.ClientRequestID != nil && check.Generation == 1 && v.State == biz.MiniCommentPass && validSafetyTrace(v.NameTraceID) && validSafetyTrace(v.ContentTraceID) && c.Status == biz.CommentStatusPending {
		c, e = db.Comment.UpdateOneID(c.ID).SetStatus(biz.CommentStatusApproved).Save(ctx)
		if e != nil {
			return nil, biz.ErrContentSafetyUnavailable
		}
	}
	check, e = db.WechatCommentCheck.Get(ctx, check.ID)
	if e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	out, e := miniProjection(ctx, db, c, check)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, biz.ErrContentSafetyUnavailable
	}
	return out, nil
}
func (r *miniCommentRepo) ListPublic(ctx context.Context, slug, app string, limit, offset int) ([]*biz.MiniComment, int, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	// Filter approval/app/pass before applying the cursor. Hash-invalid rows still
	// consume cursor positions so stale evidence cannot truncate later pages.
	rows, e := r.data.db.Comment.Query().Where(comment.ArticleSlugEQ(slug), comment.StatusEQ(biz.CommentStatusApproved)).Where(func(s *entsql.Selector) {
		checks := entsql.Table(wechatcommentcheck.Table)
		s.Where(entsql.In(s.C(comment.FieldID), entsql.Select(checks.C(wechatcommentcheck.FieldCommentID)).From(checks).Where(entsql.And(entsql.EQ(checks.C(wechatcommentcheck.FieldAppID), app), entsql.EQ(checks.C(wechatcommentcheck.FieldState), string(biz.MiniCommentPass))))))
	}).Order(ent.Desc(comment.FieldCreatedAt), ent.Desc(comment.FieldID)).Offset(offset).Limit(limit + 1).All(ctx)
	if e != nil {
		return nil, -1, biz.ErrContentSafetyUnavailable
	}
	next := -1
	if len(rows) > limit {
		rows = rows[:limit]
		next = offset + limit
	}
	items, err := miniBatchProjections(ctx, r.data.db, rows, app)
	if err != nil {
		return nil, -1, err
	}
	out := make([]*biz.MiniComment, 0, len(items))
	for _, item := range items {
		if item.Visible() {
			out = append(out, item)
		}
	}
	return out, next, nil
}
func (r *miniCommentRepo) ListMine(ctx context.Context, slug string, owner uuid.UUID, app string, limit, offset int) ([]*biz.MiniComment, int, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, e := r.data.db.Comment.Query().Where(comment.ArticleSlugEQ(slug), comment.UserIDEQ(owner)).Order(ent.Desc(comment.FieldCreatedAt), ent.Desc(comment.FieldID)).Offset(offset).Limit(limit + 1).All(ctx)
	if e != nil {
		return nil, -1, biz.ErrContentSafetyUnavailable
	}
	next := -1
	if len(rows) > limit {
		rows = rows[:limit]
		next = offset + limit
	}
	out, err := miniBatchProjections(ctx, r.data.db, rows, app)
	if err != nil {
		return nil, -1, err
	}
	return out, next, nil
}
