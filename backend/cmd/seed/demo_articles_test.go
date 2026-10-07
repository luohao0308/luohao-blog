package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

type tombstoneSeedRepo struct {
	biz.ArticleRepo
	creates int
	updates int
}

func (*tombstoneSeedRepo) FindBySlug(context.Context, string) (*biz.Article, error) {
	return nil, biz.ErrArticleNotFound
}

func (r *tombstoneSeedRepo) CreateArticle(context.Context, *biz.Article) (*biz.Article, error) {
	r.creates++
	return nil, biz.ErrArticleSlugConflict
}

func (r *tombstoneSeedRepo) UpdateArticle(context.Context, *biz.Article) (*biz.Article, error) {
	r.updates++
	return nil, biz.ErrArticleNotFound
}

func TestSeedPreservesDeletedSlugs(t *testing.T) {
	articles, err := loadDemoArticles()
	if err != nil {
		t.Fatal(err)
	}
	for _, reset := range []bool{false, true} {
		r := &tombstoneSeedRepo{}
		if err := seedDemoArticles(context.Background(), biz.NewArticleUsecase(r, nil), reset); err != nil {
			t.Fatalf("reset=%v: %v", reset, err)
		}
		if r.creates != len(articles) || r.updates != 0 {
			t.Fatalf("reset=%v creates=%d updates=%d", reset, r.creates, r.updates)
		}
	}
}

// The bundled demo files must parse and satisfy the same invariants the biz
// layer enforces, otherwise a content edit would only fail at seed time
// against a live database.
func TestLoadDemoArticles(t *testing.T) {
	articles, err := loadDemoArticles()
	if err != nil {
		t.Fatalf("loadDemoArticles() error = %v", err)
	}
	if len(articles) < 2 {
		t.Fatalf("loadDemoArticles() loaded %d articles, want the bundled set", len(articles))
	}
	slugs := map[string]bool{}
	for _, a := range articles {
		if a.Slug == "" || a.Title == "" || a.ContentMD == "" {
			t.Fatalf("article %+v misses slug, title or content", a)
		}
		if slugs[a.Slug] {
			t.Fatalf("duplicate slug %q across demo files", a.Slug)
		}
		slugs[a.Slug] = true
		if a.Status == biz.ArticleStatusPublished && a.PublishedAt == nil {
			t.Fatalf("published article %s misses published_at", a.Slug)
		}
		if a.Status == biz.ArticleStatusDraft && a.PublishedAt != nil {
			t.Fatalf("draft article %s carries published_at", a.Slug)
		}
	}
}

func TestParseDemoArticle(t *testing.T) {
	stamp := "2026-09-30T18:06:46+08:00"
	valid := "---\nslug: hello-world\ntitle: 你好\nsummary:\ntags: go, kratos ,\nstatus: published\npublished_at: " + stamp + "\n---\n# 正文\n"
	a, err := parseDemoArticle("hello.md", []byte(valid))
	if err != nil {
		t.Fatalf("parseDemoArticle(valid) error = %v", err)
	}
	if a.Slug != "hello-world" || a.Title != "你好" || a.Summary != "" {
		t.Fatalf("parsed = %+v", a)
	}
	if len(a.Tags) != 2 || a.Tags[0] != "go" || a.Tags[1] != "kratos" {
		t.Fatalf("tags = %v, want [go kratos]", a.Tags)
	}
	if a.Status != biz.ArticleStatusPublished || a.PublishedAt == nil {
		t.Fatalf("status = %d, want published with stamp", a.Status)
	}
	if want, _ := time.Parse(time.RFC3339, stamp); !a.PublishedAt.Equal(want) {
		t.Fatalf("published_at = %v, want %v", a.PublishedAt, want)
	}
	if a.ContentMD != "# 正文" {
		t.Fatalf("content_md = %q, want the body without the added trailing newline", a.ContentMD)
	}

	for name, tc := range map[string]string{
		"no frontmatter":          "# 正文\n",
		"unclosed frontmatter":    "---\nslug: x\nstatus: draft\n# 正文\n",
		"bad frontmatter line":    "---\nslug\n---\nbody\n",
		"unknown status":          "---\nslug: x\ntitle: t\nstatus: archived\n---\nbody\n",
		"published missing stamp": "---\nslug: x\ntitle: t\nstatus: published\n---\nbody\n",
		"draft with stamp":        "---\nslug: x\ntitle: t\nstatus: draft\npublished_at: " + stamp + "\n---\nbody\n",
		"bad stamp":               "---\nslug: x\ntitle: t\nstatus: published\npublished_at: 2026-13-40\n---\nbody\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseDemoArticle("bad.md", []byte(tc)); err == nil {
				t.Fatalf("parseDemoArticle(%q) succeeded, want error", name)
			} else if !strings.HasPrefix(err.Error(), "seed: bad.md:") {
				t.Fatalf("error = %v, want the file name as context", err)
			}
		})
	}
}
