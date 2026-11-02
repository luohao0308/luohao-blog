package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	articleRender "github.com/luohao0308/luohao-blog/backend/internal/biz/article/render"
	"google.golang.org/protobuf/encoding/protojson"
)

func loadReleaseBundle(raw []byte, hash string, apply, revision, caseStudies bool) (*bundle, error) {
	if revision && caseStudies {
		return nil, errRelease
	}
	if caseStudies {
		return loadCaseStudies(raw, hash, apply)
	}
	if revision {
		return loadRevision(raw, hash, apply)
	}
	return loadBundle(raw, hash, apply)
}

// Case studies have a separate, fixed create-only scope. The historical
// creator and revision formats deliberately cannot enter this path.
func loadCaseStudies(raw []byte, hash string, apply bool) (*bundle, error) {
	sum := sha256.Sum256(raw)
	if (apply || hash != "") && hash != hex.EncodeToString(sum[:]) {
		return nil, errRelease
	}
	var b bundle
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&b) != nil || decoder.Decode(new(any)) != io.EOF || b.Version != 1 || b.Status != "prepared_not_applied" || b.Target != "https://makerhao.cn" || b.Base != "" || len(b.Requires) != 0 || len(b.Operations) != 2 {
		return nil, errRelease
	}
	seen := map[string]bool{}
	for _, o := range b.Operations {
		if (o.Slug != "article-cas-revisions-case-study" && o.Slug != "article-search-sync-case-study") || seen[o.Slug] || o.Kind != "create_article" || !o.CheckSlug || o.Expected != "" || !same(o.Mask, articlePaths) || len(o.Preserve) != 0 || o.CoverDisplay != "" || o.ContentHash != "" {
			return nil, errRelease
		}
		seen[o.Slug] = true
		var fields map[string]json.RawMessage
		if json.Unmarshal(o.Fields, &fields) != nil || len(fields) != 6 {
			return nil, errRelease
		}
		for _, key := range []string{"slug", "title", "summary", "content_md", "tags", "status"} {
			value, ok := fields[key]
			if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, errRelease
			}
		}
		var status int
		if json.Unmarshal(fields["status"], &status) != nil || status != 2 {
			return nil, errRelease
		}
		var article v1.Article
		if protojson.Unmarshal(o.Fields, &article) != nil || article.Slug != o.Slug || strings.TrimSpace(article.Title) == "" || strings.TrimSpace(article.Summary) == "" || strings.TrimSpace(article.ContentMd) == "" {
			return nil, errRelease
		}
	}
	return &b, nil
}

// Called only after the strict case-study loader and before configuration loads.
func renderCaseStudies(b *bundle, output io.Writer) error {
	html := make(map[string]string, len(b.Operations))
	for _, operation := range b.Operations {
		var article v1.Article
		if err := protojson.Unmarshal(operation.Fields, &article); err != nil {
			return errRelease
		}
		rendered, err := articleRender.HTML(article.ContentMd)
		if err != nil {
			return err
		}
		html[operation.Slug] = rendered
	}
	return json.NewEncoder(output).Encode(html)
}
