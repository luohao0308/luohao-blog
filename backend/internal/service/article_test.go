package service

import (
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

// The update flow merges the patch into the current record and converts it
// for the usecase; a dropped status would surface as UNSPECIFIED and get
// rejected by the biz layer (the bug that made every update a 400).
func TestConvertArticleCarriesStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status v1.ArticleStatus
		want   biz.ArticleStatus
	}{
		{"draft", v1.ArticleStatus_ARTICLE_STATUS_DRAFT, biz.ArticleStatusDraft},
		{"published", v1.ArticleStatus_ARTICLE_STATUS_PUBLISHED, biz.ArticleStatusPublished},
		{"unspecified stays zero", v1.ArticleStatus_ARTICLE_STATUS_UNSPECIFIED, biz.ArticleStatusUnspecified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := convertArticle(&v1.Article{Slug: "s", Title: "t", ContentMd: "md", Status: tc.status})
			if got.Status != tc.want {
				t.Fatalf("status = %d, want %d", got.Status, tc.want)
			}
		})
	}
}
