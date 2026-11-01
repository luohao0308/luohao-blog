package server

import (
	"context"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type revisionHTTPRepo struct {
	syncHTTPRepo
	target *uint64
}

func (revisionHTTPRepo) ListArticleRevisions(context.Context, string, int, int) ([]*biz.ArticleRevision, error) {
	return []*biz.ArticleRevision{{ArticleSlug: "article", Version: 1, Action: "create", CreatedAt: time.Now()}}, nil
}
func (revisionHTTPRepo) GetArticleRevision(context.Context, string, uint64) (*biz.ArticleRevision, error) {
	return &biz.ArticleRevision{ArticleSlug: "article", Version: 1, Action: "create", CreatedAt: time.Now(), Snapshot: &biz.ArticleRevisionSnapshot{Title: "private", ContentMD: "body"}}, nil
}
func TestRevisionHTTPAndGRPCPermissions(t *testing.T) {
	a, e := data.NewAuthorizer()
	if e != nil {
		t.Fatal(e)
	}
	issuer := newMapIssuer()
	admin := mustIssue(t, issuer, biz.UserRoleAdmin)
	reader := mustIssue(t, issuer, biz.UserRoleReader)
	srv := khttp.NewServer(khttp.Middleware(AuthJWT(issuer), Authorize(a)))
	v1.RegisterArticleServiceHTTPServer(srv, service.NewArticleService(biz.NewArticleUsecase(revisionHTTPRepo{}, nil)))
	ts := httptest.NewServer(srv)
	defer ts.Close()
	for _, path := range []string{"/v1/articles/article/revisions", "/v1/articles/article/revisions/1"} {
		for _, tc := range []struct {
			token string
			code  int
		}{{"", 401}, {reader, 403}, {admin, 200}} {
			req, _ := http.NewRequest("GET", ts.URL+path, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			res, e := http.DefaultClient.Do(req)
			if e != nil {
				t.Fatal(e)
			}
			_ = res.Body.Close()
			if res.StatusCode != tc.code {
				t.Fatalf("%s got %d want %d", path, res.StatusCode, tc.code)
			}
		}
	}
	for _, op := range []string{"/blog.v1.ArticleService/ListArticleRevisions", "/blog.v1.ArticleService/GetArticleRevision"} {
		for _, tc := range []struct {
			token string
			ok    bool
		}{{"", false}, {reader, false}, {admin, true}} {
			_, e := newAuthzChain(issuer, a)(newTestTransport(op, mapHeader{"Authorization": {"Bearer " + tc.token}}), nil)
			if (e == nil) != tc.ok {
				t.Fatal(op, e)
			}
		}
	}
}

func (r revisionHTTPRepo) RestoreArticleRevision(_ context.Context, slug string, version, expected uint64, _ biz.ArticleRevisionPreparer) (*biz.Article, error) {
	if r.target != nil {
		*r.target = version
	}
	return &biz.Article{Slug: slug, Title: "trusted snapshot", ContentMD: "body", Version: expected + 1, Status: biz.ArticleStatusDraft}, nil
}
func TestRestoreHTTPContractAndPermissions(t *testing.T) {
	authz, e := data.NewAuthorizer()
	if e != nil {
		t.Fatal(e)
	}
	issuer := newMapIssuer()
	admin := mustIssue(t, issuer, biz.UserRoleAdmin)
	reader := mustIssue(t, issuer, biz.UserRoleReader)
	var target uint64
	srv := khttp.NewServer(khttp.Middleware(AuthJWT(issuer), Authorize(authz)))
	v1.RegisterArticleServiceHTTPServer(srv, service.NewArticleService(biz.NewArticleUsecase(revisionHTTPRepo{target: &target}, nil)))
	ts := httptest.NewServer(srv)
	defer ts.Close()
	for _, tc := range []struct {
		token, body string
		code        int
	}{{"", `{"expected_version":"1"}`, 401}, {reader, `{"expected_version":"1"}`, 403}, {admin, `{}`, 400}, {admin, `{"expected_version":"1"}`, 200}, {admin, `{"expected_version":"1","version":"999","snapshot":{"title":"attacker"},"status":2}`, 200}} {
		req, _ := http.NewRequest("POST", ts.URL+"/v1/articles/article/revisions/1/restore", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		body, e := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if e != nil {
			t.Fatal(e)
		}
		if res.StatusCode != tc.code {
			t.Fatalf("%s => %d %s", tc.body, res.StatusCode, body)
		}
		if res.StatusCode == 200 && (target != 1 || strings.Contains(string(body), "attacker") || !strings.Contains(string(body), "trusted snapshot")) {
			t.Fatalf("request body overrode snapshot/path: %s", body)
		}
	}
	for _, tc := range []struct {
		token   string
		allowed bool
	}{{"", false}, {reader, false}, {admin, true}} {
		_, e := newAuthzChain(issuer, authz)(newTestTransport("/blog.v1.ArticleService/RestoreArticleRevision", mapHeader{"Authorization": {"Bearer " + tc.token}}), nil)
		if (e == nil) != tc.allowed {
			t.Fatal(e)
		}
	}
}
