package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	blogserver "github.com/luohao0308/luohao-blog/backend/internal/server"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
)

// This fixture uses the real HTTP middleware, services, usecases, SQLite repos
// and Redis limiter/session implementations, without production configuration.
func TestCommentHTTPModerationLifecycle(t *testing.T) {
	ctx := context.Background()
	comments, db := newTestCommentRepo(t)
	d := &Data{db: db}
	rdb, _ := newTestRedis(t)
	users := biz.NewUserUsecase(NewUserRepo(d))
	admin, err := users.CreateAuthor(ctx, "author@example.com", "longenough1", "测试站长")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := users.CreateAccount(ctx, "reader@example.com", "longenough1", "测试读者", biz.UserRoleReader)
	if err != nil {
		t.Fatal(err)
	}
	articles := biz.NewArticleUsecase(NewArticleRepo(d, rdb), nil)
	if _, err = articles.CreateArticle(ctx, &biz.Article{Slug: "ai-delivery-guard", Title: "评论验证", ContentMD: "# 评论验证\n本地测试文章"}); err != nil {
		t.Fatal(err)
	}
	if _, err = articles.UpdateArticle(ctx, &biz.Article{Slug: "ai-delivery-guard", Title: "评论验证", ContentMD: "# 评论验证\n本地测试文章", Status: biz.ArticleStatusPublished}); err != nil {
		t.Fatal(err)
	}
	authConfig := jwtTestConfig("comment-fixture-only-signing-key", time.Hour)
	issuer, err := NewTokenIssuer(authConfig)
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := NewAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	auth := biz.NewAuthUsecase(users, NewSessionRepo(rdb), issuer, NewRateLimiter(rdb, authConfig), NewRegisterRateLimiter(rdb, authConfig), time.Hour)
	uc := biz.NewCommentUsecase(comments, articles, users, NewCommentRateLimiter(rdb, authConfig), NewCommentModerationPolicy(authConfig))
	srv, err := blogserver.NewHTTPServer(&conf.Server{Http: &conf.Server_HTTP{}}, issuer, authorizer,
		service.NewArticleService(articles), service.NewCategoryService(biz.NewCategoryUsecase(NewCategoryRepo(d))),
		service.NewAuthService(auth, nil, authConfig), service.NewUserService(users, nil), service.NewMediaService(nil), service.NewCommentService(uc),
		service.NewSubscriberService(nil), service.NewArticleSearchService(articles, nil), service.NewChatService(nil), service.NewOfficialAccountService(biz.NewWechatSyncUsecase(NewWechatSyncRepo(d), NewArticleRepo(d, rdb), nil, biz.WechatSyncSettings{})))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	request := func(method, path, token string, body any, want int) map[string]any {
		t.Helper()
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		var result map[string]any
		if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want {
			t.Fatalf("%s %s = %d, want %d: %v", method, path, resp.StatusCode, want, result)
		}
		return result
	}
	adminToken, err := issuer.Issue(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	readerToken, err := issuer.Issue(ctx, reader)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"article_slug": "ai-delivery-guard", "content": "正常技术讨论", "status": 2, "display_name": "伪造身份"}
	request("POST", "/v1/comments", "", body, http.StatusUnauthorized)
	ordinary := request("POST", "/v1/comments", readerToken.Token, body, http.StatusOK)
	if ordinary["status"] != float64(2) || ordinary["display_name"] != reader.DisplayName || ordinary["user_id"] != reader.ID.String() {
		t.Fatalf("ordinary reply = %v", ordinary)
	}
	body["content"] = "参考 https://example.com"
	pending := request("POST", "/v1/comments", readerToken.Token, body, http.StatusOK)
	if pending["status"] != float64(1) {
		t.Fatalf("pending reply = %v", pending)
	}
	public := request("GET", "/v1/articles/ai-delivery-guard/comments", "", nil, http.StatusOK)
	if len(public["comments"].([]any)) != 1 {
		t.Fatalf("pending leaked: %v", public)
	}
	approvePath := "/v1/comments/" + pending["id"].(string) + "/approve"
	request("POST", approvePath, readerToken.Token, map[string]any{}, http.StatusForbidden)
	request("POST", approvePath, adminToken.Token, map[string]any{}, http.StatusOK)
	public = request("GET", "/v1/articles/ai-delivery-guard/comments", "", nil, http.StatusOK)
	if len(public["comments"].([]any)) != 2 {
		t.Fatalf("approval not visible: %v", public)
	}
	first := request("GET", "/v1/articles/ai-delivery-guard/comments?page_size=1", "", nil, http.StatusOK)
	second := request("GET", "/v1/articles/ai-delivery-guard/comments?page_size=1&page_token="+url.QueryEscape(first["next_page_token"].(string)), "", nil, http.StatusOK)
	if first["comments"].([]any)[0].(map[string]any)["id"] == second["comments"].([]any)[0].(map[string]any)["id"] {
		t.Fatal("pagination repeated a row")
	}
	if _, err = articles.UpdateArticle(ctx, &biz.Article{Slug: "ai-delivery-guard", Title: "评论验证", ContentMD: "content", Status: biz.ArticleStatusDraft}); err != nil {
		t.Fatal(err)
	}
	public = request("GET", "/v1/articles/ai-delivery-guard/comments", "", nil, http.StatusOK)
	if len(public["comments"].([]any)) != 0 {
		t.Fatal("unpublished comments leaked")
	}

	// Opt-in local browser fixture, shares the same tested HTTP stack. The URL
	// file contains no credentials; all accounts above are disposable examples.
	if output := os.Getenv("BLOG_COMMENT_BROWSER_FIXTURE"); output != "" {
		if _, err = articles.UpdateArticle(ctx, &biz.Article{Slug: "ai-delivery-guard", Title: "评论验证", ContentMD: "# 评论验证\n本地测试文章", Status: biz.ArticleStatusPublished}); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 76; i++ {
			status := biz.CommentStatusPending
			if i < 22 {
				status = biz.CommentStatusApproved
			}
			if _, err = comments.Create(ctx, &biz.Comment{ArticleSlug: "ai-delivery-guard", DisplayName: reader.DisplayName, UserID: &reader.ID, Content: fmt.Sprintf("fixture-%02d", i), Status: status}); err != nil {
				t.Fatal(err)
			}
		}
		stopped := make(chan struct{}, 1)
		srv.HandleFunc("/fixture/stop", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			select {
			case stopped <- struct{}{}:
			default:
			}
			w.WriteHeader(http.StatusNoContent)
		})
		if err = os.WriteFile(output, []byte(ts.URL), 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("local browser fixture ready")
		select {
		case <-stopped:
		case <-time.After(10 * time.Minute):
			t.Fatal("browser fixture timeout")
		}
	}
}
