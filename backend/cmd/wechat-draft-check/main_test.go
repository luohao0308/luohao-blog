package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

type fakeDraftPublisher struct {
	create func(context.Context, biz.WechatArticle) (string, error)
}

func (f *fakeDraftPublisher) CreateDraft(ctx context.Context, article biz.WechatArticle) (string, error) {
	return f.create(ctx, article)
}

func (*fakeDraftPublisher) Submit(context.Context, string, string, string) (string, error) {
	return "", errors.New("submit must not be called")
}

func (*fakeDraftPublisher) Status(context.Context, string, string) (*biz.WechatPublishResult, error) {
	return nil, errors.New("status must not be called")
}

func draftCheckFixture(t *testing.T, response func() publicArticle) (*conf.Bootstrap, draftCheckDependencies, string, *int) {
	t.Helper()
	fetchCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCalls++
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/articles/hello-world" {
			t.Fatalf("unexpected public API request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" || r.URL.RawQuery != "" {
			t.Fatalf("public API request carried credentials")
		}
		_ = json.NewEncoder(w).Encode(response())
	}))
	t.Cleanup(server.Close)
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	bc := &conf.Bootstrap{OfficialAccount: &conf.OfficialAccount{
		Enabled: false, AppId: "fixture-account", AppSecret: "private-fixture",
		SiteUrl: server.URL, DefaultThumbMediaId: "cover-id", Author: "作者", Mode: "publish",
	}}
	deps := draftCheckDependencies{
		validateOrigin: func(context.Context, string) (*url.URL, *http.Client, error) {
			copy := *origin
			return &copy, server.Client(), nil
		},
	}
	return bc, deps, t.TempDir(), &fetchCalls
}

func publishedArticle() publicArticle {
	return publicArticle{
		Slug: "hello-world", Title: "公开文章", Summary: "摘要", ContentHTML: "<p>正文</p>", Status: 2,
		PublishedAt: json.RawMessage(`"2026-10-13T00:00:00Z"`),
	}
}

func TestDraftCheckRejectsUnpublishedArticle(t *testing.T) {
	t.Parallel()
	bc, deps, journalDir, _ := draftCheckFixture(t, func() publicArticle {
		article := publishedArticle()
		article.Status = 1
		return article
	})
	publisherCalls := 0
	deps.newPublisher = func(*conf.Bootstrap) (biz.WechatPublisher, error) {
		publisherCalls++
		return &fakeDraftPublisher{}, nil
	}
	_, err := executeDraftCheck(context.Background(), bc, "hello-world", journalDir, deps)
	if err == nil || err.Error() != "article_not_published" {
		t.Fatalf("error = %v", err)
	}
	if publisherCalls != 0 {
		t.Fatalf("publisher constructed for unpublished article")
	}
	entries, err := os.ReadDir(journalDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("journal created for unpublished article")
	}
}

func TestDraftCheckReusesCompletedJournalAndDoesNotModifyConfig(t *testing.T) {
	t.Parallel()
	bc, deps, journalDir, fetchCalls := draftCheckFixture(t, publishedArticle)
	createCalls := 0
	deps.newPublisher = func(cloned *conf.Bootstrap) (biz.WechatPublisher, error) {
		if !cloned.GetOfficialAccount().GetEnabled() || cloned.GetOfficialAccount().GetMode() != "draft" {
			t.Fatalf("publisher clone not forced to draft mode: %#v", cloned.GetOfficialAccount())
		}
		return &fakeDraftPublisher{create: func(_ context.Context, article biz.WechatArticle) (string, error) {
			createCalls++
			if article.Title != "公开文章" || article.ContentHTML != "<p>正文</p>" || article.SourceURL != bc.GetOfficialAccount().GetSiteUrl()+"/posts/hello-world" {
				t.Fatalf("unexpected article: %#v", article)
			}
			return "draft-media-id", nil
		}}, nil
	}
	for range 2 {
		state, err := executeDraftCheck(context.Background(), bc, "hello-world", journalDir, deps)
		if err != nil || state.Status != "completed" || state.MediaID != "draft-media-id" {
			t.Fatalf("state = %#v, err = %v", state, err)
		}
	}
	if createCalls != 1 {
		t.Fatalf("draft created %d times", createCalls)
	}
	if *fetchCalls != 2 {
		t.Fatalf("public article fetched %d times", *fetchCalls)
	}
	if bc.GetOfficialAccount().GetEnabled() || bc.GetOfficialAccount().GetMode() != "publish" {
		t.Fatalf("original config modified: %#v", bc.GetOfficialAccount())
	}
	entries, err := os.ReadDir(journalDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("journal entries = %d, err = %v", len(entries), err)
	}
	raw, err := os.ReadFile(filepath.Join(journalDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private-fixture") || strings.Contains(string(raw), "<p>正文</p>") {
		t.Fatalf("journal persisted secret or article body")
	}
}

func TestDraftCheckUnknownJournalBlocksReplay(t *testing.T) {
	t.Parallel()
	bc, deps, journalDir, _ := draftCheckFixture(t, publishedArticle)
	createCalls := 0
	deps.newPublisher = func(*conf.Bootstrap) (biz.WechatPublisher, error) {
		return &fakeDraftPublisher{create: func(context.Context, biz.WechatArticle) (string, error) {
			createCalls++
			return "", &biz.WechatRemoteError{Code: "unknown", Ambiguous: true}
		}}, nil
	}
	for attempt := 0; attempt < 2; attempt++ {
		state, err := executeDraftCheck(context.Background(), bc, "hello-world", journalDir, deps)
		if err == nil || state.Status != "unknown" {
			t.Fatalf("attempt %d state = %#v, err = %v", attempt, state, err)
		}
	}
	if createCalls != 1 {
		t.Fatalf("unknown result replayed: calls = %d", createCalls)
	}
}

func TestDraftCheckRequiresExplicitValidSlug(t *testing.T) {
	t.Parallel()
	deps := draftCheckDependencies{
		validateOrigin: func(context.Context, string) (*url.URL, *http.Client, error) {
			t.Fatal("origin validation called without a valid slug")
			return nil, nil, nil
		},
	}
	for _, slug := range []string{"", "../secret", "Hello"} {
		if _, err := executeDraftCheck(context.Background(), &conf.Bootstrap{}, slug, t.TempDir(), deps); err == nil || err.Error() != "article_slug_required_or_invalid" {
			t.Fatalf("slug %q error = %v", slug, err)
		}
	}
}

func TestValidatePublicSiteOriginRejectsPrivateAndNonOrigin(t *testing.T) {
	t.Parallel()
	for _, site := range []string{"http://127.0.0.1", "http://169.254.169.254", "https://example.com/path", "https://user:pass@example.com"} {
		if _, _, err := validatePublicSiteOrigin(context.Background(), site); err == nil {
			t.Fatalf("site %q accepted", site)
		}
	}
}

func TestFetchPublicArticleDoesNotExposeResponseBodyOnFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "appsecret=private access_token=private")
	}))
	defer server.Close()
	origin, _ := url.Parse(server.URL)
	_, err := fetchPublicArticle(context.Background(), server.Client(), origin, "hello-world")
	if err == nil || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("unsafe error = %v", err)
	}
}
