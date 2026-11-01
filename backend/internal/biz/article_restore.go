package biz

import (
	"context"
	"github.com/go-kratos/kratos/v3/errors"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz/article/render"
	"strings"
)

var ErrArticleRevisionCategoryUnavailable = errors.Conflict(v1.ErrorReason_ARTICLE_REVISION_CATEGORY_UNAVAILABLE.String(), "历史分类已失效，请在编辑页重新选择分类后保存。")
var ErrArticleRevisionMediaUnavailable = errors.Conflict(v1.ErrorReason_ARTICLE_REVISION_MEDIA_UNAVAILABLE.String(), "历史封面不可用，请重新上传封面后保存。")

type ArticleRevisionPreparer func(context.Context, *Article) error
type ArticleRevisionRestorer interface {
	RestoreArticleRevision(context.Context, string, uint64, uint64, ArticleRevisionPreparer) (*Article, error)
}

func (uc *ArticleUsecase) RestoreArticleRevision(ctx context.Context, slug string, version, expected uint64) (*Article, error) {
	if expected == 0 {
		return nil, ErrArticleVersionRequired
	}
	if !ValidSlug(slug) || version == 0 {
		return nil, ErrArticleInvalidArgument
	}
	r, ok := uc.repo.(ArticleRevisionRestorer)
	if !ok {
		return nil, ErrArticleInvalidArgument
	}
	a, e := r.RestoreArticleRevision(ctx, slug, version, expected, func(ctx context.Context, a *Article) error {
		if !ValidMediaURL(a.CoverURL) {
			return ErrArticleRevisionMediaUnavailable
		}
		if a.CoverURL != "" {
			if uc.media == nil {
				return ErrArticleRevisionMediaUnavailable
			}
			data, _, e := uc.media.GetImage(ctx, strings.TrimPrefix(a.CoverURL, assetsMediaPrefix))
			if e != nil || len(data) == 0 {
				return ErrArticleRevisionMediaUnavailable
			}
		}
		if e := validateArticle(a); e != nil {
			return e
		}
		html, e := render.HTML(a.ContentMD)
		if e != nil {
			return ErrArticleInvalidArgument
		}
		a.ContentHTML = html
		return nil
	})
	if e != nil {
		return nil, e
	}
	uc.syncIndex(ctx, a)
	return a, nil
}
