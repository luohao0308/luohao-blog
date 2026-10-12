package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/uuid"
	"strings"
	"sync"
	"time"
)

// WechatArticle is a versioned snapshot; it contains no account credentials.
type WechatArticle struct {
	Title        string
	Summary      string
	ContentHTML  string
	SourceURL    string
	Author       string
	ThumbMediaID string
}

// WechatSyncTask is durable derived state, independent of blog availability.
type WechatSyncTask struct {
	ID                 string
	Slug               string
	ContentHash        string
	AccountFingerprint string
	Article            WechatArticle
	Mode               string
	State              string
	DraftID            string
	RemoteID           string
	ArticleURL         string
	LastError          string
	Attempts           int
	CreatedAt          time.Time
	UpdatedAt          time.Time
	NextAttemptAt      time.Time
}

type WechatPublishResult struct {
	Pending    bool
	Succeeded  bool
	ArticleURL string
	ErrorCode  string
}

// WechatRemoteError never embeds tokens, secrets or response bodies.
type WechatRemoteError struct {
	Code      string
	Ambiguous bool
}

func (e *WechatRemoteError) Error() string { return "wechat request failed: " + e.Code }

type WechatPublisher interface {
	CreateDraft(context.Context, WechatArticle) (string, error)
	Submit(context.Context, string, string, string) (string, error)
	Status(context.Context, string, string) (*WechatPublishResult, error)
}

type WechatSyncRepo interface {
	Enqueue(context.Context, *WechatSyncTask) error
	Get(context.Context, string) (*WechatSyncTask, error)
	List(context.Context, int, int) ([]*WechatSyncTask, error)
	Runnable(context.Context, int) ([]*WechatSyncTask, error)
	Claim(context.Context, string, string, string) (bool, error)
	Save(context.Context, *WechatSyncTask, string) error
	Recover(context.Context, time.Time) error
}

// ArticlePublicationRepo atomically commits an article and its outgoing task.
type ArticlePublicationRepo interface {
	UpdateArticleAndEnqueue(context.Context, *Article, *WechatSyncTask) (*Article, error)
}

type WechatSyncSettings struct {
	Enabled            bool
	Configured         bool
	Mode               string
	SiteURL            string
	ThumbMediaID       string
	Author             string
	AccountName        string
	AccountID          string
	AccountFingerprint string
}

var (
	ErrWechatSyncDisabled = errors.BadRequest("WECHAT_SYNC_DISABLED", "公众号同步尚未配置或启用")
	ErrWechatSyncNotFound = errors.NotFound("WECHAT_SYNC_NOT_FOUND", "同步任务不存在")
	ErrWechatSyncConflict = errors.Conflict("WECHAT_SYNC_CONFLICT", "任务状态已变化或不能安全重试")
	ErrWechatSyncInvalid  = errors.BadRequest("WECHAT_SYNC_INVALID", "无效的同步请求")
)

func WechatContentHash(a *Article) string {
	data, _ := json.Marshal([]string{a.Title, a.Summary, a.ContentHTML})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type WechatSyncUsecase struct {
	repo      WechatSyncRepo
	articles  ArticleRepo
	publisher WechatPublisher
	settings  WechatSyncSettings
	mu        sync.Mutex
	cancel    context.CancelFunc
	done      chan struct{}
}

func NewWechatSyncUsecase(repo WechatSyncRepo, articles ArticleRepo, publisher WechatPublisher, settings WechatSyncSettings) *WechatSyncUsecase {
	return &WechatSyncUsecase{repo: repo, articles: articles, publisher: publisher, settings: settings}
}
func (uc *WechatSyncUsecase) Settings() WechatSyncSettings { return uc.settings }
func (uc *WechatSyncUsecase) Task(a *Article) *WechatSyncTask {
	if !uc.settings.Enabled || !uc.settings.Configured || a.Status != ArticleStatusPublished {
		return nil
	}
	hash := WechatContentHash(a)
	data, _ := json.Marshal([]string{a.ID.String(), hash, uc.settings.Mode, uc.settings.AccountFingerprint, uc.settings.SiteURL, uc.settings.ThumbMediaID, uc.settings.Author})
	id := uuid.NewSHA1(uuid.NameSpaceURL, data).String()
	return &WechatSyncTask{ID: id, Slug: a.Slug, ContentHash: hash, AccountFingerprint: uc.settings.AccountFingerprint, Mode: uc.settings.Mode, State: "queued", Article: WechatArticle{Title: a.Title, Summary: a.Summary, ContentHTML: a.ContentHTML, SourceURL: strings.TrimRight(uc.settings.SiteURL, "/") + "/posts/" + a.Slug, Author: uc.settings.Author, ThumbMediaID: uc.settings.ThumbMediaID}}
}
func (uc *WechatSyncUsecase) List(ctx context.Context, limit, offset int) ([]*WechatSyncTask, error) {
	return uc.repo.List(ctx, limit, offset)
}
func WechatTaskCanRetry(t *WechatSyncTask) bool {
	return t.State == "failed" && t.RemoteID == "" && t.Attempts < 10
}
func (uc *WechatSyncUsecase) Retry(ctx context.Context, id string) (*WechatSyncTask, error) {
	if !uc.settings.Enabled || !uc.settings.Configured {
		return nil, ErrWechatSyncDisabled
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrWechatSyncInvalid
	}
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !WechatTaskCanRetry(t) {
		return nil, ErrWechatSyncConflict
	}
	a, err := uc.articles.FindBySlug(ctx, t.Slug)
	if err != nil {
		return nil, err
	}
	if current := uc.Task(a); current == nil || current.ID != t.ID {
		return nil, ErrWechatSyncConflict
	}
	t.State = "queued"
	if t.DraftID != "" {
		t.State = "ready"
	}
	t.LastError = ""
	t.NextAttemptAt = time.Time{}
	if err := uc.repo.Save(ctx, t, "failed"); err != nil {
		return nil, err
	}
	return uc.repo.Get(ctx, id)
}
func (uc *WechatSyncUsecase) SyncArticle(ctx context.Context, slug string) (*WechatSyncTask, error) {
	if !ValidSlug(slug) {
		return nil, ErrWechatSyncInvalid
	}
	a, err := uc.articles.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if a.Status != ArticleStatusPublished {
		return nil, ErrWechatSyncInvalid
	}
	t := uc.Task(a)
	if t == nil {
		return nil, ErrWechatSyncDisabled
	}
	if err := uc.repo.Enqueue(ctx, t); err != nil {
		return nil, err
	}
	return uc.repo.Get(ctx, t.ID)
}

// Start and Stop join the Kratos lifecycle; no detached delivery goroutines.
func (uc *WechatSyncUsecase) Start(context.Context) error {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.cancel != nil || !uc.settings.Enabled || !uc.settings.Configured {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	uc.cancel = cancel
	uc.done = make(chan struct{})
	go func() {
		defer close(uc.done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			if err := uc.Tick(ctx); err != nil && ctx.Err() == nil {
				log.Warn("wechat task processing/storage failed")
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return nil
}
func (uc *WechatSyncUsecase) Stop(ctx context.Context) error {
	uc.mu.Lock()
	cancel, done := uc.cancel, uc.done
	uc.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (uc *WechatSyncUsecase) Tick(ctx context.Context) error {
	if !uc.settings.Enabled || !uc.settings.Configured || uc.publisher == nil {
		return nil
	}
	if err := uc.repo.Recover(ctx, time.Now().Add(-5*time.Minute)); err != nil {
		return err
	}
	tasks, err := uc.repo.Runnable(ctx, 10)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if err := uc.process(ctx, t); err != nil {
			return err
		}
	}
	return nil
}
func (uc *WechatSyncUsecase) process(ctx context.Context, t *WechatSyncTask) error {
	from := t.State
	if t.AccountFingerprint != uc.settings.AccountFingerprint {
		t.State = "cancelled"
		if t.DraftID != "" || t.RemoteID != "" {
			t.State = "unknown"
		}
		t.LastError = "target_account_changed"
		return uc.repo.Save(ctx, t, from)
	}
	processing := map[string]string{"queued": "drafting", "ready": "submitting", "polling": "checking"}[from]
	if processing == "" {
		return nil
	}
	claimed, err := uc.repo.Claim(ctx, t.ID, from, processing)
	if err != nil || !claimed {
		return err
	}
	t.State = processing
	t.Attempts++
	t.NextAttemptAt = time.Time{}
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if from != "polling" {
		a, readErr := uc.articles.FindBySlug(requestCtx, t.Slug)
		if readErr != nil {
			if errors.IsNotFound(readErr) {
				t.State = "cancelled"
				t.LastError = "article_unavailable"
			} else {
				t.State = "failed"
				t.LastError = "article_read_failed"
			}
			return uc.repo.Save(ctx, t, processing)
		}
		current := uc.Task(a)
		if current == nil || current.ID != t.ID {
			t.State = "cancelled"
			t.LastError = "article_changed_or_unpublished"
			return uc.repo.Save(ctx, t, processing)
		}
	}
	switch from {
	case "queued":
		t.DraftID, err = uc.publisher.CreateDraft(requestCtx, t.Article)
		if err == nil {
			t.State = "ready"
		}
	case "ready":
		if t.Mode == "draft" {
			t.State = "drafted"
		} else {
			t.RemoteID, err = uc.publisher.Submit(requestCtx, t.DraftID, t.Mode, strings.ReplaceAll(t.ID, "-", ""))
			if err == nil {
				t.State = "polling"
			}
		}
	case "polling":
		var result *WechatPublishResult
		result, err = uc.publisher.Status(requestCtx, t.RemoteID, t.Mode)
		if err != nil {
			t.State = "polling"
			t.LastError = "status_query_failed"
			var remote *WechatRemoteError
			if stderrors.As(err, &remote) {
				t.LastError = remote.Code
				switch remote.Code {
				case "-1", "45009", "45011", "45066", "40001", "40014", "42001", "transport_error", "http_status_429", "http_status_500", "http_status_502", "http_status_503", "http_status_504", "invalid_response":
				default:
					t.State = "unknown"
				}
			}
			if t.Attempts >= 100 {
				t.State = "unknown"
			}
			if t.State == "polling" {
				t.NextAttemptAt = time.Now().Add(wechatPollBackoff(t.Attempts))
			}
			return uc.repo.Save(ctx, t, processing)
		}
		if result == nil {
			t.State = "polling"
			t.LastError = "invalid_status_response"
			t.NextAttemptAt = time.Now().Add(wechatPollBackoff(t.Attempts))
			return uc.repo.Save(ctx, t, processing)
		}
		t.LastError = ""
		t.State = "failed"
		if result.Pending {
			t.State = "polling"
			t.NextAttemptAt = time.Now().Add(wechatPollBackoff(t.Attempts))
		} else if result.Succeeded {
			t.State = "succeeded"
			t.ArticleURL = result.ArticleURL
		} else {
			t.LastError = result.ErrorCode
		}
	}
	if err != nil {
		t.State = "failed"
		t.LastError = "remote_request_failed"
		var remote *WechatRemoteError
		if stderrors.As(err, &remote) {
			t.LastError = remote.Code
			if remote.Ambiguous {
				t.State = "unknown"
			}
		}
	}
	return uc.repo.Save(ctx, t, processing)
}

// Read-only polling backs off persistently; quota and permission errors must
// not turn into a fixed-interval request storm after a process restart.
func wechatPollBackoff(attempts int) time.Duration {
	shift := attempts - 3
	if shift < 0 {
		shift = 0
	}
	if shift > 5 {
		shift = 5
	}
	delay := time.Minute * time.Duration(1<<shift)
	if delay > 30*time.Minute {
		return 30 * time.Minute
	}
	return delay
}
