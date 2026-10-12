package data

import (
	"context"
	"fmt"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatsynctask"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
)

type wechatSyncRepo struct {
	data *Data
}

func NewWechatSyncRepo(data *Data) biz.WechatSyncRepo {
	return &wechatSyncRepo{data: data}
}

func toBizWechatSyncTask(po *ent.WechatSyncTask) *biz.WechatSyncTask {
	if po == nil {
		return nil
	}
	out := &biz.WechatSyncTask{
		ID:                 po.ID.String(),
		Slug:               po.Slug,
		ContentHash:        po.ContentHash,
		Mode:               po.Mode,
		AccountFingerprint: po.AccountFingerprint,
		State:              po.State,
		DraftID:            po.DraftID,
		RemoteID:           po.RemoteID,
		ArticleURL:         po.ArticleURL,
		LastError:          po.LastError,
		Attempts:           po.Attempts,
		CreatedAt:          po.CreatedAt,
		UpdatedAt:          po.UpdatedAt,
		Article: biz.WechatArticle{
			Title:        po.ArticleTitle,
			Summary:      po.ArticleSummary,
			ContentHTML:  po.ArticleContentHTML,
			SourceURL:    po.ArticleSourceURL,
			Author:       po.ArticleAuthor,
			ThumbMediaID: po.ArticleThumbMediaID,
		},
	}
	if po.NextAttemptAt != nil {
		out.NextAttemptAt = *po.NextAttemptAt
	}
	return out
}

func nonZeroTimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func parseWechatSyncTaskID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: invalid task id", biz.ErrWechatSyncInvalid)
	}
	return parsed, nil
}

func enqueueWechatSyncTask(ctx context.Context, client *ent.Client, task *biz.WechatSyncTask) error {
	if task == nil {
		return fmt.Errorf("%w: task is nil", biz.ErrWechatSyncInvalid)
	}
	id, err := parseWechatSyncTaskID(task.ID)
	if err != nil {
		return err
	}
	_, err = client.WechatSyncTask.Create().
		SetID(id).
		SetSlug(task.Slug).
		SetContentHash(task.ContentHash).
		SetMode(task.Mode).
		SetAccountFingerprint(task.AccountFingerprint).
		SetState(task.State).
		SetArticleTitle(task.Article.Title).
		SetArticleSummary(task.Article.Summary).
		SetArticleContentHTML(task.Article.ContentHTML).
		SetArticleSourceURL(task.Article.SourceURL).
		SetArticleAuthor(task.Article.Author).
		SetArticleThumbMediaID(task.Article.ThumbMediaID).
		SetDraftID(task.DraftID).
		SetRemoteID(task.RemoteID).
		SetArticleURL(task.ArticleURL).
		SetLastError(task.LastError).
		SetAttempts(task.Attempts).
		SetNillableNextAttemptAt(nonZeroTimePtr(task.NextAttemptAt)).
		Save(ctx)
	if err == nil {
		return nil
	}
	if !ent.IsConstraintError(err) {
		return err
	}
	// The deterministic caller-generated id includes the article version,
	// account and publication settings. A replay leaves its snapshot intact.
	exists, queryErr := client.WechatSyncTask.Query().
		Where(wechatsynctask.IDEQ(id)).
		Exist(ctx)
	if queryErr != nil {
		return queryErr
	}
	if exists {
		return nil
	}
	return err
}

func (r *wechatSyncRepo) Enqueue(ctx context.Context, task *biz.WechatSyncTask) error {
	return enqueueWechatSyncTask(ctx, r.data.db, task)
}

func (r *wechatSyncRepo) Get(ctx context.Context, id string) (*biz.WechatSyncTask, error) {
	parsed, err := parseWechatSyncTaskID(id)
	if err != nil {
		return nil, err
	}
	po, err := r.data.db.WechatSyncTask.Get(ctx, parsed)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrWechatSyncNotFound
		}
		return nil, err
	}
	return toBizWechatSyncTask(po), nil
}

func (r *wechatSyncRepo) List(ctx context.Context, limit, offset int) ([]*biz.WechatSyncTask, error) {
	if limit <= 0 || offset < 0 {
		return nil, biz.ErrWechatSyncInvalid
	}
	pos, err := r.data.db.WechatSyncTask.Query().
		Order(wechatsynctask.ByCreatedAt(entsql.OrderDesc()), wechatsynctask.ByID(entsql.OrderDesc())).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*biz.WechatSyncTask, 0, len(pos))
	for _, po := range pos {
		out = append(out, toBizWechatSyncTask(po))
	}
	return out, nil
}

func (r *wechatSyncRepo) Runnable(ctx context.Context, limit int) ([]*biz.WechatSyncTask, error) {
	if limit <= 0 {
		return nil, biz.ErrWechatSyncInvalid
	}
	pos, err := r.data.db.WechatSyncTask.Query().
		Where(
			wechatsynctask.StateIn("queued", "ready", "polling"),
			wechatsynctask.Or(
				wechatsynctask.NextAttemptAtIsNil(),
				wechatsynctask.NextAttemptAtLTE(time.Now()),
			),
		).
		Order(wechatsynctask.ByUpdatedAt(), wechatsynctask.ByID()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*biz.WechatSyncTask, 0, len(pos))
	for _, po := range pos {
		out = append(out, toBizWechatSyncTask(po))
	}
	return out, nil
}

func (r *wechatSyncRepo) Claim(ctx context.Context, id, expected, next string) (bool, error) {
	parsed, err := parseWechatSyncTaskID(id)
	if err != nil {
		return false, err
	}
	affected, err := r.data.db.WechatSyncTask.Update().
		Where(
			wechatsynctask.IDEQ(parsed),
			wechatsynctask.StateEQ(expected),
			wechatsynctask.Or(
				wechatsynctask.NextAttemptAtIsNil(),
				wechatsynctask.NextAttemptAtLTE(time.Now()),
			),
		).
		SetState(next).
		AddAttempts(1).
		Save(ctx)
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (r *wechatSyncRepo) Save(ctx context.Context, task *biz.WechatSyncTask, expected string) error {
	if task == nil {
		return fmt.Errorf("%w: task is nil", biz.ErrWechatSyncInvalid)
	}
	id, err := parseWechatSyncTaskID(task.ID)
	if err != nil {
		return err
	}
	updater := r.data.db.WechatSyncTask.Update().
		Where(wechatsynctask.IDEQ(id), wechatsynctask.StateEQ(expected)).
		SetState(task.State).
		SetDraftID(task.DraftID).
		SetRemoteID(task.RemoteID).
		SetArticleURL(task.ArticleURL).
		SetLastError(task.LastError)
	if task.NextAttemptAt.IsZero() {
		updater = updater.ClearNextAttemptAt()
	} else {
		updater = updater.SetNextAttemptAt(task.NextAttemptAt)
	}
	affected, err := updater.Save(ctx)
	if err != nil {
		return err
	}
	if affected != 1 {
		return biz.ErrWechatSyncConflict
	}
	return nil
}

func (r *wechatSyncRepo) Recover(ctx context.Context, cutoff time.Time) error {
	if _, err := r.data.db.WechatSyncTask.Update().
		Where(
			wechatsynctask.StateIn("drafting", "submitting"),
			wechatsynctask.UpdatedAtLT(cutoff),
		).
		SetState("unknown").
		SetLastError("recovery_ambiguous").
		Save(ctx); err != nil {
		return err
	}
	// Status checks are read-only. An interrupted check can safely return to
	// polling, unlike interrupted draft/submission writes whose outcome is
	// ambiguous and must stop for operator review.
	_, err := r.data.db.WechatSyncTask.Update().
		Where(
			wechatsynctask.StateEQ("checking"),
			wechatsynctask.UpdatedAtLT(cutoff),
		).
		SetState("polling").
		SetLastError("recovery_query_retry").
		ClearNextAttemptAt().
		Save(ctx)
	return err
}
