package biz

import (
	"context"
	stderrors "errors"
	"hash/fnv"
	"slices"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

// ErrArticleSearchIndexMissing requires explicit administrator initialization.
var ErrArticleSearchIndexMissing = stderrors.New("article search index missing")

// ArticleIndexSnapshot is observed derived state, never persisted author data.
type ArticleIndexSnapshot struct {
	Exists             bool
	Article            *Article
	SemanticExpected   bool
	SemanticReady      bool
	SemanticRepairable bool
}
type ArticleIndexInspector interface {
	GetArticleIndexSnapshot(context.Context, string) (*ArticleIndexSnapshot, error)
	RetryArticleIndex(context.Context, *Article) error
}
type SearchSyncState int32

const (
	SearchSyncUnknown SearchSyncState = iota
	SearchSyncSynced
	SearchSyncMissing
	SearchSyncStale
	SearchSyncNotRequired
	SearchSyncUnavailable
	SearchSyncPartial
)

type ArticleSearchSyncStatus struct {
	ArticleSlug   string
	State         SearchSyncState
	CanRetry      bool
	Message       string
	CheckedAt     time.Time
	SemanticReady bool
}

func (uc *ArticleUsecase) indexLock(slug string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(slug))
	lock := &uc.indexLocks[h.Sum32()%uint32(len(uc.indexLocks))]
	lock.Lock()
	return lock.Unlock
}

// writeLatestIndex runs under a stripe lock and reloads author state before writing.
// This coordinates this process's saves, deletes and retries; CLI rebuilds require
// separate maintenance coordination.
func (uc *ArticleUsecase) writeLatestIndex(ctx context.Context, slug string) error {
	latest, err := uc.repo.FindBySlug(ctx, slug)
	if errors.IsNotFound(err) {
		return uc.indexer.RemoveArticle(ctx, slug)
	}
	if err != nil {
		return err
	}
	return uc.indexer.IndexArticle(ctx, latest)
}
func (uc *ArticleUsecase) GetArticleSearchSync(ctx context.Context, slug string) (*ArticleSearchSyncStatus, error) {
	a, err := uc.GetArticle(ctx, slug)
	if err != nil {
		return nil, err
	}
	return uc.inspectArticleIndex(ctx, a), nil
}
func (uc *ArticleUsecase) inspectArticleIndex(ctx context.Context, a *Article) *ArticleSearchSyncStatus {
	result := &ArticleSearchSyncStatus{ArticleSlug: a.Slug, State: SearchSyncUnavailable, Message: "搜索服务未配置或暂不可用，请检查服务配置。", CheckedAt: time.Now()}
	inspector, ok := uc.indexer.(ArticleIndexInspector)
	if !ok {
		return result
	}
	snap, err := inspector.GetArticleIndexSnapshot(ctx, a.Slug)
	if stderrors.Is(err, ErrArticleSearchIndexMissing) {
		result.Message = "搜索索引不存在，请管理员初始化或重建后再试。"
		return result
	}
	if err != nil || snap == nil {
		result.CanRetry = true
		result.Message = "无法检查搜索索引，正文仍已保存；可稍后重试。"
		return result
	}
	result.SemanticReady = snap.SemanticReady
	if a.Status != ArticleStatusPublished {
		if !snap.Exists {
			result.State = SearchSyncNotRequired
			result.Message = "草稿无需进入搜索索引。"
		} else {
			result.State = SearchSyncStale
			result.CanRetry = true
			result.Message = "未发布文章仍有索引残留，可重新同步清理。"
		}
		return result
	}
	result.CanRetry = true
	if !snap.Exists {
		result.State = SearchSyncMissing
		result.Message = "正文已保存，搜索索引缺失，请重新同步。"
		return result
	}
	b := snap.Article
	if b == nil || a.Title != b.Title || a.Summary != b.Summary || a.ContentMD != b.ContentMD || !sameIndexTags(a.Tags, b.Tags) || (a.PublishedAt != nil && (b.PublishedAt == nil || !a.PublishedAt.Equal(*b.PublishedAt))) {
		result.State = SearchSyncStale
		result.Message = "正文已保存，搜索索引尚未更新。"
		return result
	}
	if snap.SemanticExpected && !snap.SemanticReady {
		result.State = SearchSyncPartial
		result.CanRetry = snap.SemanticRepairable
		result.Message = "关键词索引已同步，语义检索暂未就绪。"
		if !result.CanRetry {
			result.Message += "请检查向量索引映射并安排重建。"
		}
		return result
	}
	result.State = SearchSyncSynced
	result.CanRetry = false
	result.Message = "关键词索引已同步，搜索结果可能稍后更新。"
	if result.SemanticReady {
		result.Message = "关键词与语义索引已同步，搜索结果可能稍后更新。"
	}
	return result
}
func sameIndexTags(a, b []string) bool {
	a = slices.Clone(a)
	b = slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(slices.Compact(a), slices.Compact(b))
}
func (uc *ArticleUsecase) RetryArticleSearchSync(ctx context.Context, slug string) (*ArticleSearchSyncStatus, error) {
	if !ValidSlug(slug) {
		return nil, ErrArticleInvalidArgument
	}
	unlock := uc.indexLock(slug)
	defer unlock()
	a, err := uc.GetArticle(ctx, slug)
	if err != nil {
		return nil, err
	}
	result := uc.inspectArticleIndex(ctx, a)
	if !result.CanRetry {
		return result, nil
	}
	inspector, ok := uc.indexer.(ArticleIndexInspector)
	if !ok {
		return result, nil
	}
	retryErr := inspector.RetryArticleIndex(ctx, a)
	if stderrors.Is(retryErr, ErrArticleSearchIndexMissing) {
		result.State = SearchSyncUnavailable
		result.CanRetry = false
		result.CheckedAt = time.Now()
		result.Message = "搜索索引不存在，请管理员初始化或重建后再试。"
		return result, nil
	}
	latest, err := uc.GetArticle(ctx, slug)
	if err != nil {
		return nil, err
	}
	result = uc.inspectArticleIndex(ctx, latest)
	if retryErr != nil {
		result.Message = "本次同步未完成，正文已保存；请刷新状态后重试。"
		if result.State == SearchSyncSynced || result.State == SearchSyncNotRequired {
			result.State = SearchSyncUnavailable
			result.CanRetry = true
		}
	}
	return result, nil
}
