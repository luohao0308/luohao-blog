package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	entcomment "github.com/luohao0308/luohao-blog/backend/internal/data/ent/comment"
	entuser "github.com/luohao0308/luohao-blog/backend/internal/data/ent/user"
	blogserver "github.com/luohao0308/luohao-blog/backend/internal/server"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
)

type miniHTTPBudget struct{}

func (miniHTTPBudget) Allow(context.Context, string) (bool, error) { return true, nil }

type miniHTTPWechat struct{}

func (miniHTTPWechat) AppID() string    { return "wx-mini-http" }
func (miniHTTPWechat) Configured() bool { return true }
func (miniHTTPWechat) Code2Session(_ context.Context, code string) (string, error) {
	switch code {
	case "reader-code":
		return "reader-openid", nil
	case "other-code":
		return "other-openid", nil
	}
	return "", biz.ErrWechatCodeInvalid
}

type miniHTTPChecker struct {
	mu       sync.Mutex
	decision biz.ContentSafetyDecision
	err      error
	calls    atomic.Int32
}

func (*miniHTTPChecker) AppID() string    { return "wx-mini-http" }
func (*miniHTTPChecker) Configured() bool { return true }
func (c *miniHTTPChecker) set(decision biz.ContentSafetyDecision, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.decision = decision
	c.err = err
}
func (c *miniHTTPChecker) CheckText(_ context.Context, _ biz.ContentSafetyInput) (biz.ContentSafetyResult, error) {
	c.calls.Add(1)
	c.mu.Lock()
	defer c.mu.Unlock()
	return biz.ContentSafetyResult{Decision: c.decision, TraceID: "http-fixture-trace"}, c.err
}

// Real route registration, JWT/RBAC middleware, codecs, usecases and SQLite
// storage; only the outbound WeChat services are replaced with local fakes.
func TestMiniCommentHTTPSafetyLifecycle(t *testing.T) { miniCommentHTTPSafetyLifecycle(t, nil) }

func miniCommentHTTPSafetyLifecycle(t *testing.T, db *ent.Client) {
	t.Helper()
	ctx := context.Background()
	if db == nil {
		_, db = newTestCommentRepo(t)
	}
	repo := NewCommentRepo(&Data{db: db})
	d := &Data{db: db}
	rdb, _ := newTestRedis(t)
	users := biz.NewUserUsecase(NewUserRepo(d))
	admin, err := users.CreateAuthor(ctx, "mini-admin@example.invalid", "fixture-password", "站长")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := users.CreateAccount(ctx, "mini-reader@example.invalid", "fixture-password", "读者", biz.UserRoleReader)
	if err != nil {
		t.Fatal(err)
	}
	other, err := users.CreateAccount(ctx, "mini-other@example.invalid", "fixture-password", "其他读者", biz.UserRoleReader)
	if err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{admin.ID, reader.ID, other.ID}
	t.Cleanup(func() {
		if _, e := db.Comment.Delete().Where(entcomment.UserIDIn(ids...)).Exec(context.Background()); e != nil {
			t.Error("fixture comment cleanup", e)
		}
		if _, e := db.User.Delete().Where(entuser.IDIn(ids...)).Exec(context.Background()); e != nil {
			t.Error("fixture user cleanup", e)
		}
	})
	for _, binding := range []struct {
		u      *biz.User
		openid string
	}{{reader, "reader-openid"}, {other, "other-openid"}} {
		if _, err = db.WechatIdentity.Create().SetUserID(binding.u.ID).SetAppID("wx-mini-http").SetOpenid(binding.openid).Save(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = users.UpdateAvatar(ctx, reader.ID, "/v1/assets/avatars/unsafe-user.png"); err != nil {
		t.Fatal(err)
	}
	articles := biz.NewArticleUsecase(NewArticleRepo(d, rdb), nil)
	createdArticle, err := articles.CreateArticle(ctx, &biz.Article{Slug: "mini-http", Title: "隔离文章", ContentMD: "body", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := db.Article.DeleteOneID(createdArticle.ID).Exec(context.Background()); e != nil {
			t.Error("fixture article cleanup", e)
		}
	})
	if _, err = articles.UpdateArticle(ctx, &biz.Article{Slug: "mini-http", Title: "隔离文章", ContentMD: "body", Status: biz.ArticleStatusPublished, Version: 1}); err != nil {
		t.Fatal(err)
	}
	cfg := jwtTestConfig("mini-http-synthetic-key", time.Hour)
	issuer, err := NewTokenIssuer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	authz, err := NewAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	auth := biz.NewAuthUsecase(users, NewSessionRepo(rdb), issuer, nil, nil, time.Hour)
	comments := biz.NewCommentUsecase(repo, articles, users, NewCommentRateLimiter(rdb, cfg), NewCommentModerationPolicy(cfg))
	checker := &miniHTTPChecker{decision: biz.ContentSafetyPass}
	mini := biz.NewMiniCommentUsecase(comments, biz.NewMiniContentIdentityUsecase(users, miniHTTPWechat{}), checker, NewMiniCommentRepo(d), miniHTTPBudget{})
	srv, err := blogserver.NewHTTPServer(&conf.Server{Http: &conf.Server_HTTP{}}, issuer, authz,
		service.NewArticleService(articles), service.NewCategoryService(biz.NewCategoryUsecase(NewCategoryRepo(d))), service.NewProjectService(nil), service.NewAboutService(nil), service.NewCollectionService(biz.NewCollectionUsecase(NewCollectionRepo(d))), service.NewAuthService(auth, nil, cfg), service.NewUserService(users, nil), service.NewMediaService(nil), service.NewCommentServiceWithMini(comments, mini), service.NewSubscriberService(nil), service.NewArticleSearchService(articles, nil), service.NewChatService(nil), service.NewOfficialAccountService(nil))
	if err != nil {
		t.Fatal(err)
	}
	token := func(u *biz.User) string {
		v, e := issuer.Issue(ctx, u)
		if e != nil {
			t.Fatal(e)
		}
		return v.Token
	}
	adminToken, readerToken, otherToken := token(admin), token(reader), token(other)
	request := func(method, path, credential string, body any, want int) map[string]any {
		t.Helper()
		payload, e := json.Marshal(body)
		if e != nil {
			t.Fatal(e)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		if credential != "" {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		var result map[string]any
		if e = json.Unmarshal(rec.Body.Bytes(), &result); e != nil {
			t.Fatalf("decode response: %v", e)
		}
		if rec.Code != want {
			t.Fatalf("%s %s = %d want %d: %v", method, path, rec.Code, want, result)
		}
		return result
	}
	body := func(key, code, text string) map[string]any {
		return map[string]any{"article_slug": "mini-http", "content": text, "wechat_code": code, "client_request_id": key}
	}
	listPath := "/v1/mini/articles/mini-http/comments"
	public := request("GET", listPath, "", nil, 200)
	if len(public["comments"].([]any)) != 0 {
		t.Fatal("initial projection nonempty")
	}
	request("POST", "/v1/mini/comments", "", body("anon", "reader-code", "hello"), 401)
	request("GET", listPath+"/mine", "", nil, 401)
	request("POST", "/v1/mini/comments/00000000-0000-0000-0000-000000000001/recheck", "", map[string]any{"wechat_code": "reader-code"}, 401)
	request("POST", "/v1/comments", readerToken, map[string]any{"article_slug": "mini-http", "content": "web-only"}, 200)
	public = request("GET", listPath, "", nil, 200)
	if len(public["comments"].([]any)) != 0 {
		t.Fatal("mini fell back to unchecked Web comments")
	}
	before, _ := db.Comment.Query().Count(ctx)
	request("POST", "/v1/mini/comments", readerToken, body("wrong-owner", "other-code", "wrong"), 403)
	request("POST", "/v1/mini/comments", readerToken, body("invalid-code", "bad-code", "invalid"), 412)
	after, _ := db.Comment.Query().Count(ctx)
	if before != after {
		t.Fatal("invalid identity persisted a comment")
	}
	first := request("POST", "/v1/mini/comments", readerToken, body("pass-one", "reader-code", "ordinary first"), 200)
	comment := first["comment"].(map[string]any)
	id := comment["id"].(string)
	if first["visible"] != true || first["safety_state"] != float64(2) || comment["user_id"] != reader.ID.String() || comment["avatar_url"] != "" {
		t.Fatalf("unsafe pass reply: %v", first)
	}
	if checker.calls.Load() != 2 {
		t.Fatal("both scenes must run")
	}
	replay := request("POST", "/v1/mini/comments", readerToken, body("pass-one", "bad-code", "ordinary first"), 200)
	if replay["comment"].(map[string]any)["id"] != id || checker.calls.Load() != 2 {
		t.Fatal("replay consumed identity or repeated checks")
	}
	request("POST", "/v1/mini/comments", readerToken, body("pass-one", "reader-code", "changed"), 409)
	var retryID string
	for _, tc := range []struct {
		name     string
		decision biz.ContentSafetyDecision
		err      error
		state    float64
	}{{"review", biz.ContentSafetyReview, nil, 3}, {"risky", biz.ContentSafetyRisky, nil, 4}, {"identity", biz.ContentSafetyPass, biz.ErrContentSafetyIdentity, 5}} {
		t.Run(tc.name, func(t *testing.T) {
			checker.set(tc.decision, tc.err)
			held := request("POST", "/v1/mini/comments", readerToken, body(tc.name, "reader-code", "held "+tc.name), 200)
			c := held["comment"].(map[string]any)
			if _, ok := held["can_recheck"].(bool); !ok {
				t.Fatal("false boolean fields must be present for the mini contract")
			}
			if tc.name == "identity" {
				retryID = c["id"].(string)
				if held["safety_error"] != "identity" {
					t.Fatal("identity error classification lost")
				}
			}
			if held["visible"] != false || held["safety_state"] != tc.state || c["status"] != float64(1) || c["avatar_url"] != "" {
				t.Fatalf("held reply: %v", held)
			}
			request("POST", "/v1/comments/"+c["id"].(string)+"/approve", adminToken, map[string]any{}, 200)
			rows := request("GET", listPath, "", nil, 200)["comments"].([]any)
			if len(rows) != 1 {
				t.Fatalf("Web approval bypassed mini safety: %v", rows)
			}
		})
	}
	checker.set(biz.ContentSafetyPass, nil)
	request("POST", "/v1/mini/comments/"+retryID+"/recheck", otherToken, map[string]any{"wechat_code": "other-code"}, 404)
	recovered := request("POST", "/v1/mini/comments/"+retryID+"/recheck", readerToken, map[string]any{"wechat_code": "reader-code"}, 200)
	if recovered["visible"] != true || recovered["can_recheck"] != false {
		t.Fatalf("retry did not preserve prior approval and pass detection: %v", recovered)
	}
	request("POST", "/v1/mini/comments", otherToken, body("other-pass", "other-code", "ordinary other"), 200)
	own := request("GET", listPath+"/mine", otherToken, nil, 200)["records"].([]any)
	if len(own) != 1 || own[0].(map[string]any)["comment"].(map[string]any)["user_id"] != other.ID.String() {
		t.Fatal("mine crossed accounts")
	}
	firstPage := request("GET", listPath+"?page_size=1", "", nil, 200)
	next, ok := firstPage["next_page_token"].(string)
	if !ok || next == "" {
		t.Fatal("missing next cursor")
	}
	secondPage := request("GET", listPath+"?page_size=1&page_token="+url.QueryEscape(next), "", nil, 200)
	if firstPage["comments"].([]any)[0].(map[string]any)["id"] == secondPage["comments"].([]any)[0].(map[string]any)["id"] {
		t.Fatal("cursor repeated comment")
	}
	request("DELETE", "/v1/comments/"+id, adminToken, nil, 200)
	request("POST", "/v1/mini/comments", readerToken, body("pass-one", "reader-code", "ordinary first"), 410)
}
