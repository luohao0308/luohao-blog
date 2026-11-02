package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	blogserver "github.com/luohao0308/luohao-blog/backend/internal/server"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
	"github.com/redis/go-redis/v9"
)

// The upstream fake implements the actual wire protocol. The real WeChat
// clients, JWT/RBAC, codec, domain flow, SQLite and Redis implementations run.
func TestMiniCommentRealClientsHTTPPipeline(t *testing.T) {
	const appID = "wx-upstream-fixture"
	const secret = "upstream-secret-fixture"
	const openID = "upstream-openid-fixture"
	const access = "upstream-token-fixture"
	var mode atomic.Value
	mode.Store("pass")
	var tokenCalls, checkCalls, codeCalls atomic.Int32
	var mu sync.Mutex
	type submitted struct {
		Scene   int
		Content string
	}
	var inputs []submitted
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/sns/jscode2session":
			codeCalls.Add(1)
			if r.Method != "GET" || r.URL.Query().Get("appid") != appID || r.URL.Query().Get("secret") != secret {
				t.Error("code exchange protocol mismatch")
			}
			if r.URL.Query().Get("js_code") != "valid-code" {
				_, _ = w.Write([]byte(`{"errcode":40029}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"openid": openID})
		case "/cgi-bin/stable_token":
			tokenCalls.Add(1)
			var b struct {
				GrantType    string `json:"grant_type"`
				AppID        string `json:"appid"`
				Secret       string `json:"secret"`
				ForceRefresh bool   `json:"force_refresh"`
			}
			if e := json.NewDecoder(r.Body).Decode(&b); e != nil {
				t.Error(e)
			}
			if r.Method != "POST" || b.GrantType != "client_credential" || b.AppID != appID || b.Secret != secret || b.ForceRefresh || r.URL.RawQuery != "" {
				t.Error("token protocol mismatch")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": access, "expires_in": 345})
		case "/wxa/msg_sec_check":
			checkCalls.Add(1)
			var b struct {
				Version int    `json:"version"`
				Scene   int    `json:"scene"`
				OpenID  string `json:"openid"`
				Content string `json:"content"`
			}
			if e := json.NewDecoder(r.Body).Decode(&b); e != nil {
				t.Error(e)
			}
			if r.Method != "POST" || r.URL.Query().Get("access_token") != access || b.Version != 2 || b.OpenID != openID || (b.Scene != 1 && b.Scene != 2) {
				t.Error("text protocol mismatch")
			}
			mu.Lock()
			inputs = append(inputs, submitted{b.Scene, b.Content})
			mu.Unlock()
			decision := "pass"
			trace := "trace-name-fixture"
			if b.Scene == 2 {
				trace = "trace-body-fixture"
				switch mode.Load().(string) {
				case "risky":
					decision = "risky"
				case "review":
					decision = "review"
				case "expired":
					_, _ = w.Write([]byte(`{"errcode":61010,"errmsg":"upstream-openid-fixture"}`))
					return
				case "http503":
					w.WriteHeader(503)
					_, _ = w.Write([]byte(secret + access))
					return
				case "invalid-token":
					_, _ = w.Write([]byte(`{"errcode":40014,"errmsg":"upstream-token-fixture"}`))
					return
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "trace_id": trace, "result": map[string]any{"suggest": decision, "label": 100}})
		default:
			t.Error("unexpected upstream path")
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	ctx := context.Background()
	comments, db := newTestCommentRepo(t)
	d := &Data{db: db}
	rdb, m := newTestRedis(t)
	cache := redis.NewClient(&redis.Options{Addr: m.Addr(), ContextTimeoutEnabled: true, ReadTimeout: 200 * time.Millisecond, WriteTimeout: 200 * time.Millisecond})
	t.Cleanup(func() { _ = cache.Close() })
	po, err := db.User.Create().SetEmail("protocol-reader@example.invalid").SetPasswordHash("synthetic-hash").SetDisplayName("协议昵称").SetRole(biz.UserRoleReader).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.WechatIdentity.Create().SetUserID(po.ID).SetAppID(appID).SetOpenid(openID).Save(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Article.Create().SetSlug("protocol-post").SetTitle("协议文章").SetContentMd("fixture").SetStatus(biz.ArticleStatusPublished).SetVersion(1).Save(ctx); err != nil {
		t.Fatal(err)
	}
	users := biz.NewUserUsecase(NewUserRepo(d))
	reader, err := users.ByID(ctx, po.ID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := jwtTestConfig("upstream-http-synthetic-jwt", time.Hour)
	issuer, err := NewTokenIssuer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	authz, err := NewAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	articles := biz.NewArticleUsecase(NewArticleRepo(d, rdb), nil)
	base := biz.NewCommentUsecase(comments, articles, users, NewCommentRateLimiter(rdb, cfg), NewCommentModerationPolicy(cfg))
	wxConfig := &conf.Wechat{Enabled: true, AppId: appID, AppSecret: secret}
	wxClient := NewWechatClient(wxConfig).(*wechatClient)
	wxClient.baseURL = upstream.URL
	safety := NewWechatContentSafetyClient(wxConfig, cache, true).(*wechatContentSafetyClient)
	safety.baseURL = upstream.URL
	mini := biz.NewMiniCommentUsecase(base, biz.NewMiniContentIdentityUsecase(users, wxClient), safety, NewMiniCommentRepo(d), NewMiniCommentLimiter(&WechatSafetyCache{client: cache}, cfg))
	auth := biz.NewAuthUsecase(users, NewSessionRepo(rdb), issuer, nil, nil, time.Hour)
	srv, err := blogserver.NewHTTPServer(&conf.Server{Http: &conf.Server_HTTP{}}, issuer, authz, service.NewArticleService(articles), service.NewCategoryService(biz.NewCategoryUsecase(NewCategoryRepo(d))), service.NewProjectService(nil), service.NewAboutService(nil), service.NewCollectionService(biz.NewCollectionUsecase(NewCollectionRepo(d))), service.NewAuthService(auth, nil, cfg), service.NewUserService(users, nil), service.NewMediaService(nil), service.NewCommentServiceWithMini(base, mini), service.NewSubscriberService(nil), service.NewArticleSearchService(articles, nil), service.NewChatService(nil), service.NewOfficialAccountService(nil))
	if err != nil {
		t.Fatal(err)
	}
	token, err := issuer.Issue(ctx, reader)
	if err != nil {
		t.Fatal(err)
	}
	request := func(t *testing.T, method, path string, body any, want int) map[string]any {
		t.Helper()
		payload, e := json.Marshal(body)
		if e != nil {
			t.Fatal(e)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token.Token)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		for _, private := range []string{secret, openID, access, "trace-name-fixture", "trace-body-fixture"} {
			if strings.Contains(rec.Body.String(), private) {
				t.Fatal("private upstream metadata leaked to HTTP caller")
			}
		}
		if rec.Code != want {
			t.Fatalf("%s %s status %d want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		var out map[string]any
		if e = json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	create := func(t *testing.T, key, code, content string) map[string]any {
		return request(t, "POST", "/v1/mini/comments", map[string]any{"article_slug": "protocol-post", "content": content, "wechat_code": code, "client_request_id": key}, 200)
	}
	first := create(t, "wire-pass", "valid-code", "  实际正文  ")
	firstComment := first["comment"].(map[string]any)
	if first["visible"] != true || first["can_recheck"] != false || firstComment["user_id"] != reader.ID.String() {
		t.Fatalf("pass wire fields: %v", first)
	}
	stored, err := db.Comment.Get(ctx, uuid.MustParse(firstComment["id"].(string)))
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	captured := append([]submitted(nil), inputs...)
	mu.Unlock()
	if len(captured) != 2 || captured[0].Scene != 1 || captured[0].Content != stored.DisplayName || captured[1].Scene != 2 || captured[1].Content != stored.Content {
		t.Fatalf("detected input is not stored snapshot: %v", captured)
	}
	replay := create(t, "wire-pass", "already-consumed-code", "实际正文")
	if replay["comment"].(map[string]any)["id"] != firstComment["id"] || checkCalls.Load() != 2 || codeCalls.Load() != 1 || tokenCalls.Load() != 1 {
		t.Fatal("idempotent receipt caused duplicate upstream calls")
	}
	for _, tc := range []struct {
		mode  string
		state float64
	}{{"review", 3}, {"risky", 4}, {"expired", 5}, {"http503", 5}, {"invalid-token", 5}} {
		t.Run(tc.mode, func(t *testing.T) {
			mode.Store(tc.mode)
			before := checkCalls.Load()
			held := create(t, "wire-"+tc.mode, "valid-code", "body-"+tc.mode)
			if held["visible"] != false || held["safety_state"] != tc.state || held["comment"].(map[string]any)["user_id"] != reader.ID.String() {
				t.Fatalf("failure incorrectly visible or missing receipt: %v", held)
			}
			if checkCalls.Load()-before != 2 {
				t.Fatal("check failure unexpectedly retried")
			}
			rows := request(t, "GET", "/v1/mini/articles/protocol-post/comments", nil, 200)["comments"].([]any)
			if len(rows) != 1 {
				t.Fatal("failed check leaked to public projection")
			}
		})
	}
	if tokenCalls.Load() != 1 {
		t.Fatal("cached token was refetched before invalid-token response")
	}
	if cache.Exists(ctx, safety.tokenKey()).Val() != 0 {
		t.Fatal("invalid token cache was not removed")
	}
	mode.Store("pass")
	create(t, "wire-after-token-reset", "valid-code", "after reset")
	if tokenCalls.Load() != 2 {
		t.Fatal("later request failed to reacquire token normally")
	}
	mine := request(t, "GET", "/v1/mini/articles/protocol-post/comments/mine", nil, 200)["records"].([]any)
	if len(mine) != 7 {
		t.Fatalf("saved receipts missing: %d", len(mine))
	}
}
