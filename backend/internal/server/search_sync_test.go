package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	khttp "github.com/go-kratos/kratos/v3/transport/http"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
)

type syncHTTPRepo struct{ biz.ArticleRepo }

func (syncHTTPRepo) FindBySlug(context.Context, string) (*biz.Article, error) {
	return &biz.Article{Slug: "article", Status: biz.ArticleStatusPublished}, nil
}
func TestSearchSyncHTTPAndGRPCPermissions(t *testing.T) {
	authz, err := data.NewAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	issuer := newMapIssuer()
	admin := mustIssue(t, issuer, biz.UserRoleAdmin)
	reader := mustIssue(t, issuer, biz.UserRoleReader)
	srv := khttp.NewServer(khttp.Middleware(AuthJWT(issuer), Authorize(authz)))
	v1.RegisterArticleServiceHTTPServer(srv, service.NewArticleService(biz.NewArticleUsecase(syncHTTPRepo{}, nil)))
	ts := httptest.NewServer(srv)
	defer ts.Close()
	for _, operation := range []struct{ path, method string }{{"/v1/articles/article/search-sync", "GET"}, {"/v1/articles/article/search-sync/retry", "POST"}} {
		for _, tc := range []struct {
			token string
			code  int
		}{{"", 401}, {reader, 403}, {admin, 200}} {
			req, _ := http.NewRequest(operation.method, ts.URL+operation.path, strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			res, e := http.DefaultClient.Do(req)
			if e != nil {
				t.Fatal(e)
			}
			_ = res.Body.Close()
			if res.StatusCode != tc.code {
				t.Fatalf("%s got %d want %d", operation.path, res.StatusCode, tc.code)
			}
		}
	}
	for _, operation := range []string{"/blog.v1.ArticleService/GetArticleSearchSync", "/blog.v1.ArticleService/RetryArticleSearchSync"} {
		for _, tc := range []struct {
			token   string
			allowed bool
		}{{"", false}, {reader, false}, {admin, true}} {
			_, e := newAuthzChain(issuer, authz)(newTestTransport(operation, mapHeader{"Authorization": {"Bearer " + tc.token}}), nil)
			if (e == nil) != tc.allowed {
				t.Fatalf("gRPC %s: %v", operation, e)
			}
		}
	}
}
