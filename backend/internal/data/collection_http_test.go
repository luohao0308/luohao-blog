package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	blogserver "github.com/luohao0308/luohao-blog/backend/internal/server"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
)

func TestCollectionHTTPLifecycle(t *testing.T) {
	ctx := context.Background()
	articleRepo, db := newTestArticleRepo(t)
	d := &Data{db: db}
	rdb, mr := newTestRedis(t)
	users := biz.NewUserUsecase(NewUserRepo(d))
	author, _ := users.CreateAuthor(ctx, "author@example.com", "longenough1", "Author")
	_ = author
	reader, _ := users.CreateAccount(ctx, "reader@example.com", "longenough1", "Reader", biz.UserRoleReader)
	reader2, _ := users.CreateAccount(ctx, "reader2@example.com", "longenough1", "Reader2", biz.UserRoleReader)
	articles := biz.NewArticleUsecase(articleRepo, nil)
	for i := 0; i < 105; i++ {
		slug := fmt.Sprintf("collection-fixture-%03d", i)
		created, err := articles.CreateArticle(ctx, &biz.Article{Slug: slug, Title: slug, ContentMD: "body"})
		if err != nil {
			t.Fatal(err)
		}
		created.Status = biz.ArticleStatusPublished
		if _, err = articles.UpdateArticle(ctx, created); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := articles.CreateArticle(ctx, &biz.Article{Slug: "collection-hidden-draft", Title: "draft", ContentMD: "body"}); err != nil {
		t.Fatal(err)
	}
	deleted, err := articles.CreateArticle(ctx, &biz.Article{Slug: "collection-hidden-deleted", Title: "deleted", ContentMD: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if err = articles.DeleteArticle(ctx, deleted.Slug); err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionRepo(rdb)
	tokens := NewAccountEmailTokenRepo(d)
	mailer := &capturedAccountMailer{}
	accountEmail := biz.NewAccountEmailUsecase(users, tokens, NewEmailRateLimiter(rdb), mailer, sessions, biz.AccountEmailSettings{SiteOrigin: "http://127.0.0.1:3000"})
	if err = accountEmail.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = accountEmail.Stop(context.Background()) }()
	authConfig := jwtTestConfig("collection-fixture-signing-key", time.Hour)
	issuer, err := NewTokenIssuer(authConfig)
	if err != nil {
		t.Fatal(err)
	}
	authz, err := NewAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	authUC := biz.NewAuthUsecaseWithEmail(users, sessions, issuer, NewRateLimiter(rdb, authConfig), NewRegisterRateLimiter(rdb, authConfig), time.Hour, tokens)
	projectUC := biz.NewProjectUsecase(NewProjectRepo(d))
	_, _ = projectUC.Create(ctx, &biz.Project{Slug: "luohao-blog", Name: "luohao-blog", Stack: []string{}, Screenshots: []string{}, Published: true, Sort: 10})
	aboutUC := biz.NewAboutUsecase(NewAboutRepo(d))
	_, _ = aboutUC.Update(ctx, &biz.About{Headline: "全栈工程师", Contacts: []biz.AboutContact{{Label: "GitHub", Href: "https://github.com/luohao0308", Text: "github.com/luohao0308"}}})
	commentUC := biz.NewCommentUsecase(NewCommentRepo(d), articles, users, NewCommentRateLimiter(rdb, authConfig), NewCommentModerationPolicy(authConfig))
	mediaStore, err := NewMediaStore(&conf.Data{UploadsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	collectionUC := biz.NewCollectionUsecase(NewCollectionRepo(d))
	srv, err := blogserver.NewHTTPServer(&conf.Server{Http: &conf.Server_HTTP{}}, issuer, authz, service.NewArticleService(articles), service.NewCategoryService(biz.NewCategoryUsecase(NewCategoryRepo(d))), service.NewProjectService(projectUC), service.NewAboutService(aboutUC), service.NewCollectionService(collectionUC), service.NewAuthServiceWithEmail(authUC, nil, accountEmail, authConfig), service.NewUserService(users, nil), service.NewMediaService(biz.NewMediaUsecase(mediaStore)), service.NewCommentService(commentUC), service.NewSubscriberService(nil), service.NewArticleSearchService(articles, nil), service.NewChatService(nil), service.NewOfficialAccountService(nil))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(srv)
	defer server.Close()
	type result struct {
		status int
		body   []byte
		header http.Header
	}
	request := func(method, path, token string, body any) result {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, _ = json.Marshal(body)
		}
		req, reqErr := http.NewRequest(method, server.URL+path, bytes.NewReader(raw))
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, doErr := server.Client().Do(req)
		if doErr != nil {
			t.Fatal(doErr)
		}
		defer func() { _ = resp.Body.Close() }()
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return result{resp.StatusCode, buf.Bytes(), resp.Header.Clone()}
	}
	login := func(email string) string {
		got := request(http.MethodPost, "/v1/auth/login", "", map[string]string{"email": email, "password": "longenough1"})
		if got.status != 200 {
			t.Fatalf("login %s=%d %s", email, got.status, got.body)
		}
		var out map[string]any
		_ = json.Unmarshal(got.body, &out)
		return out["access_token"].(string)
	}
	adminToken, readerToken, reader2Token := login(author.Email), login(reader.Email), login(reader2.Email)
	if got := request(http.MethodGet, "/v1/collections/list", "", nil); got.status != http.StatusUnauthorized {
		t.Fatalf("anon list=%d", got.status)
	}
	if got := request(http.MethodPost, "/v1/collections/collection-fixture-000", readerToken, map[string]any{}); got.status != http.StatusOK {
		t.Fatalf("save=%d %s", got.status, got.body)
	} else {
		first := got.body
		again := request(http.MethodPost, "/v1/collections/collection-fixture-000", readerToken, map[string]any{})
		if again.status != 200 || !bytes.Equal(first, again.body) {
			t.Fatalf("duplicate changed=%d %s / %s", again.status, first, again.body)
		}
	}
	for _, slug := range []string{"collection-hidden-draft", "collection-hidden-deleted", "missing"} {
		if got := request(http.MethodPost, "/v1/collections/"+slug, readerToken, map[string]any{}); got.status != http.StatusNotFound {
			t.Fatalf("save hidden %s=%d", slug, got.status)
		}
		if got := request(http.MethodPost, "/v1/collections/"+slug, adminToken, map[string]any{}); got.status != http.StatusNotFound {
			t.Fatalf("admin save hidden %s=%d", slug, got.status)
		}
	}
	if got := request(http.MethodDelete, "/v1/collections/collection-fixture-000", reader2Token, nil); got.status != http.StatusOK {
		t.Fatalf("cross delete=%d", got.status)
	}
	if got := request(http.MethodGet, "/v1/collections/list", readerToken, nil); got.status != http.StatusOK || !bytes.Contains(got.body, []byte("collection-fixture-000")) {
		t.Fatalf("owner list=%d %s", got.status, got.body)
	} else if got.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("collection Cache-Control=%q", got.header.Get("Cache-Control"))
	}
	if got := request(http.MethodDelete, "/v1/collections/collection-fixture-000", readerToken, nil); got.status != http.StatusOK {
		t.Fatalf("delete=%d", got.status)
	}
	if got := request(http.MethodDelete, "/v1/collections/collection-fixture-000", readerToken, nil); got.status != http.StatusOK {
		t.Fatalf("repeat delete=%d", got.status)
	}
	if file := os.Getenv("BLOG_COLLECTION_BROWSER_FIXTURE"); file != "" {
		mr.FlushAll()
		mailer.reset()
		if err = os.WriteFile(file, []byte(server.URL), 0o600); err != nil {
			t.Fatal(err)
		}
		deadline := time.After(10 * time.Minute)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-deadline:
				return
			case <-ticker.C:
				if _, statErr := os.Stat(file + ".stop"); statErr == nil {
					return
				}
			}
		}
	}
}
