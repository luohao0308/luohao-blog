package data

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	blogserver "github.com/luohao0308/luohao-blog/backend/internal/server"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Real HTTP auth + SQLite outbox + worker + local fake WeChat API.
func TestOfficialAccountHTTPLifecycle(t *testing.T) {
	ctx := context.Background()
	repo, db := newTestArticleRepo(t)
	d := &Data{db: db}
	rdb, _ := newTestRedis(t)
	users := biz.NewUserUsecase(NewUserRepo(d))
	if _, err := users.CreateAuthor(ctx, "author@example.com", "longenough1", "测试站长"); err != nil {
		t.Fatal(err)
	}
	if _, err := users.CreateAccount(ctx, "reader@example.com", "longenough1", "测试读者", biz.UserRoleReader); err != nil {
		t.Fatal(err)
	}
	apiCalls := map[string]int{}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls[r.URL.Path]++
		var response string
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			response = `{"access_token":"fixture-token","expires_in":7200}`
		case "/cgi-bin/draft/add":
			response = `{"media_id":"fixture-draft"}`
		case "/cgi-bin/freepublish/submit":
			response = `{"publish_id":"fixture-publish","errcode":0}`
		case "/cgi-bin/freepublish/get":
			response = `{"publish_status":0,"article_detail":{"item":[{"article_url":"https://mp.weixin.qq.com/s/fixture"}]}}`
		default:
			t.Errorf("unexpected fake remote path %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	defer remote.Close()
	settings := biz.WechatSyncSettings{Enabled: true, Configured: true, Mode: "publish", SiteURL: "https://blog.example.com", ThumbMediaID: "cover", AccountID: "makerhao1", AccountFingerprint: "fixture-account", AccountName: "makerhao"}
	syncUC := biz.NewWechatSyncUsecase(NewWechatSyncRepo(d), repo, testOfficialAccountPublisher(remote), settings)
	articles := biz.NewArticleUsecaseWithWechat(repo, nil, syncUC)
	config := jwtTestConfig("wechat-fixture-only-signing-key", time.Hour)
	issuer, err := NewTokenIssuer(config)
	if err != nil {
		t.Fatal(err)
	}
	authz, err := NewAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	auth := biz.NewAuthUsecase(users, NewSessionRepo(rdb), issuer, NewRateLimiter(rdb, config), NewRegisterRateLimiter(rdb, config), time.Hour)
	uploadsDir := t.TempDir()
	mediaStore, err := NewMediaStore(&conf.Data{UploadsDir: uploadsDir})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := blogserver.NewHTTPServer(&conf.Server{Http: &conf.Server_HTTP{}}, issuer, authz,
		service.NewArticleService(articles), service.NewCategoryService(biz.NewCategoryUsecase(NewCategoryRepo(d))),
		service.NewAuthService(auth, nil, config), service.NewUserService(users, nil), service.NewMediaService(biz.NewMediaUsecase(mediaStore)), service.NewCommentService(nil),
		service.NewSubscriberService(nil), service.NewArticleSearchService(articles, nil), service.NewChatService(nil), service.NewOfficialAccountService(syncUC))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(srv)
	defer server.Close()
	request := func(method, path, token string, body any, want int) map[string]any {
		t.Helper()
		var data []byte
		if body != nil {
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, e := http.NewRequest(method, server.URL+path, bytes.NewReader(data))
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer func() { _ = res.Body.Close() }()
		raw, e := io.ReadAll(res.Body)
		if e != nil {
			t.Fatal(e)
		}
		if res.StatusCode != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, res.StatusCode, want, raw)
		}
		out := map[string]any{}
		if e = json.Unmarshal(raw, &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	login := func(email string) string {
		reply := request("POST", "/v1/auth/login", "", map[string]string{"email": email, "password": "longenough1"}, 200)
		return reply["access_token"].(string)
	}
	admin, reader := login("author@example.com"), login("reader@example.com")
	mediaDir := filepath.Join(uploadsDir, mediaDirName)
	mediaFiles := func() int {
		files, readErr := os.ReadDir(mediaDir)
		if readErr != nil {
			t.Fatal(readErr)
		}
		return len(files)
	}
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	request("POST", "/v1/admin/media/images", "", map[string]any{"data": png}, http.StatusUnauthorized)
	request("POST", "/v1/admin/media/images", reader, map[string]any{"data": png}, http.StatusForbidden)
	if got := mediaFiles(); got != 0 {
		t.Fatalf("unauthorized media uploads wrote %d file(s)", got)
	}
	uploaded := request("POST", "/v1/admin/media/images", admin, map[string]any{"data": png}, http.StatusOK)
	mediaURL, ok := uploaded["url"].(string)
	if !ok || !biz.ValidMediaURL(mediaURL) || mediaFiles() != 1 {
		t.Fatalf("media upload reply = %#v, files = %d", uploaded, mediaFiles())
	}
	mediaResponse, err := server.Client().Get(server.URL + mediaURL)
	if err != nil {
		t.Fatal(err)
	}
	mediaBytes, readErr := io.ReadAll(mediaResponse.Body)
	_ = mediaResponse.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if mediaResponse.StatusCode != http.StatusOK || mediaResponse.Header.Get("Content-Type") != "image/png" || !bytes.Equal(mediaBytes, png) {
		t.Fatalf("anonymous media GET = status %d type %q bytes %x", mediaResponse.StatusCode, mediaResponse.Header.Get("Content-Type"), mediaBytes)
	}
	if cache := mediaResponse.Header.Get("Cache-Control"); cache != "public, max-age=31536000, immutable" {
		t.Fatalf("media Cache-Control = %q", cache)
	}
	for _, path := range []string{"/v1/admin/wechat-sync/settings", "/v1/admin/wechat-sync/tasks"} {
		request("GET", path, "", nil, 401)
		request("GET", path, reader, nil, 403)
	}
	request("POST", "/v1/admin/wechat-sync/articles/fixture", reader, map[string]any{}, 403)
	request("POST", "/v1/admin/wechat-sync/tasks/invalid/retry", reader, map[string]any{}, 403)
	reply := request("GET", "/v1/admin/wechat-sync/settings", admin, nil, 200)
	if _, ok := reply["app_secret"]; ok {
		t.Fatal("secret exposed")
	}
	create := request("POST", "/v1/articles/create", admin, map[string]any{"slug": "wechat-integration", "title": "同步验证", "content_md": "# 正文\n这是本地内容"}, 200)
	// Creating a draft must never enqueue publication.
	rows, e := NewWechatSyncRepo(d).List(ctx, 10, 0)
	if e != nil || len(rows) != 0 {
		t.Fatal("draft enqueued")
	}
	request("PUT", "/v1/articles/update?update_mask=status", admin, map[string]any{"slug": create["slug"], "title": create["title"], "content_md": create["content_md"], "status": 2}, 200)
	rows, e = NewWechatSyncRepo(d).List(ctx, 10, 0)
	if e != nil || len(rows) != 1 || rows[0].State != "queued" {
		t.Fatalf("outbox: %+v %v", rows, e)
	}
	originalID := rows[0].ID
	for range 3 {
		if e := syncUC.Tick(ctx); e != nil {
			t.Fatal(e)
		}
	}
	rows, e = NewWechatSyncRepo(d).List(ctx, 10, 0)
	if e != nil || rows[0].State != "succeeded" {
		t.Fatalf("remote: %+v %v", *rows[0], e)
	}
	request("POST", "/v1/admin/wechat-sync/articles/wechat-integration", admin, map[string]any{}, 200)
	rows, _ = NewWechatSyncRepo(d).List(ctx, 10, 0)
	if len(rows) != 1 || rows[0].ID != originalID || apiCalls["/cgi-bin/freepublish/submit"] != 1 {
		t.Fatal("duplicate publication")
	}
	request("POST", "/v1/admin/wechat-sync/tasks/"+originalID+"/retry", admin, map[string]any{}, 409)
	request("GET", "/v1/admin/wechat-sync/tasks?page_size=20", admin, nil, 200)
	request("GET", "/v1/admin/wechat-sync/tasks?page_token=invalid", admin, nil, 400)
	// Browser fixture is opt-in and uses only disposable local accounts/data.
	if file := os.Getenv("BLOG_WECHAT_BROWSER_FIXTURE"); file != "" {
		if e := os.WriteFile(file, []byte(server.URL), 0600); e != nil {
			t.Fatal(e)
		}
		deadline := time.After(5 * time.Minute)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-deadline:
				t.Fatal("browser fixture timed out")
			case <-ticker.C:
				if _, e := os.Stat(file + ".stop"); e == nil {
					return
				}
			}
		}
	}
}
