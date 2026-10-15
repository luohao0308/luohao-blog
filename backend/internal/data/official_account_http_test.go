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
	projectUC := biz.NewProjectUsecase(NewProjectRepo(d))
	for _, p := range []*biz.Project{
		{Slug: "luohao-blog", Name: "luohao-blog", Summary: "本站：Go + Kratos 后端、Nuxt 4 SSR 前端的全栈个人站", Description: "单体仓库全栈项目", Stack: []string{"Go", "Kratos", "Nuxt 4"}, Role: "独立设计与开发", RepoURL: "https://github.com/luohao0308/luohao-blog", Emoji: "🛠️", Year: "2026", Featured: true, Published: true, Sort: 10},
		{Slug: "dev-workflow", Name: "dev-workflow", Summary: "可分发的 AI 协作开发流程", Description: "项目画像、契约治理与交付门禁", Stack: []string{"Bash", "Python"}, Role: "作者与维护者", RepoURL: "https://github.com/luohao0308/dev-workflow", Emoji: "⚙️", Year: "2026", Featured: true, Published: true, Sort: 20},
		{Slug: "job-application-workbench", Name: "job-application-workbench", Summary: "求职投递工作台", Description: "岗位聚合、匹配度排序与投递状态管理", Stack: []string{"Vue 3", "Node.js"}, Role: "独立开发", Emoji: "📋", Year: "2025", Published: true, Sort: 30},
	} {
		if _, err := projectUC.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	aboutSeed := &biz.About{
		Headline: "全栈工程师 · Go / Vue 双栈 · AI Agent 工程实践者。", Intro: "相信工程治理和自动化流程是个人开发者的杠杆。",
		Skills:              []biz.AboutSkillGroup{{Group: "后端", Items: []string{"Go", "Kratos"}}, {Group: "前端", Items: []string{"Vue 3", "Nuxt 4"}}},
		Timeline:            []biz.AboutTimelineEntry{{Year: "2026", Text: "搭建 luohao-blog：全栈个人站 + AI 交付治理实践场，所有迭代过程写成文章。"}},
		Contacts:            []biz.AboutContact{{Label: "GitHub", Href: "https://github.com/luohao0308", Text: "github.com/luohao0308"}, {Label: "邮箱", Href: "mailto:2429260713@qq.com", Text: "2429260713@qq.com"}},
		RepresentativeLinks: []biz.AboutLink{{Label: "luohao-blog", Href: "https://github.com/luohao0308/luohao-blog"}},
	}
	aboutUC := biz.NewAboutUsecase(NewAboutRepo(d))
	if _, err := aboutUC.Update(ctx, aboutSeed); err != nil {
		t.Fatal(err)
	}
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
	commentUC := biz.NewCommentUsecase(NewCommentRepo(d), articles, users, NewCommentRateLimiter(rdb, config), NewCommentModerationPolicy(config))
	uploadsDir := t.TempDir()
	mediaStore, err := NewMediaStore(&conf.Data{UploadsDir: uploadsDir})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := blogserver.NewHTTPServer(&conf.Server{Http: &conf.Server_HTTP{}}, issuer, authz,
		service.NewArticleService(articles), service.NewCategoryService(biz.NewCategoryUsecase(NewCategoryRepo(d))),
		service.NewProjectService(projectUC), service.NewAboutService(aboutUC),
		service.NewCollectionService(biz.NewCollectionUsecase(NewCollectionRepo(d))),
		service.NewAuthService(auth, nil, config), service.NewUserService(users, nil), service.NewMediaService(biz.NewMediaUsecase(mediaStore)), service.NewCommentService(commentUC),
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
	publicProjects := request("GET", "/v1/projects/list", "", nil, http.StatusOK)
	if projects, ok := publicProjects["projects"].([]any); !ok || len(projects) != 3 {
		t.Fatalf("public projects = %#v", publicProjects)
	}
	request("POST", "/v1/admin/projects", reader, map[string]any{"slug": "fixture-draft", "name": "Fixture"}, http.StatusForbidden)
	request("POST", "/v1/admin/projects", admin, map[string]any{"slug": "fixture-draft", "name": "Fixture", "cover_url": mediaURL, "screenshots": []string{mediaURL}}, http.StatusOK)
	request("GET", "/v1/projects/fixture-draft", "", nil, http.StatusNotFound)
	request("GET", "/v1/projects/fixture-draft", admin, nil, http.StatusOK)
	request("PUT", "/v1/admin/projects/fixture-draft?update_mask=published", admin, map[string]any{"slug": "fixture-draft", "name": "Fixture", "published": true}, http.StatusOK)
	request("GET", "/v1/projects/fixture-draft", "", nil, http.StatusOK)
	request("DELETE", "/v1/admin/projects/fixture-draft", admin, nil, http.StatusOK)
	request("GET", "/v1/projects/fixture-draft", "", nil, http.StatusNotFound)
	aboutReply := request("GET", "/v1/about", "", nil, http.StatusOK)
	if aboutReply["headline"] != aboutSeed.Headline {
		t.Fatalf("public about = %#v", aboutReply)
	}
	request("PUT", "/v1/admin/about", reader, map[string]any{"headline": "forbidden"}, http.StatusForbidden)
	request("PUT", "/v1/admin/about", admin, map[string]any{"headline": aboutSeed.Headline, "intro": "temporary", "skills": aboutSeed.Skills, "timeline": aboutSeed.Timeline, "contacts": aboutSeed.Contacts, "representative_links": aboutSeed.RepresentativeLinks}, http.StatusOK)
	request("PUT", "/v1/admin/about", admin, map[string]any{"headline": aboutSeed.Headline, "intro": aboutSeed.Intro, "skills": aboutSeed.Skills, "timeline": aboutSeed.Timeline, "contacts": aboutSeed.Contacts, "representative_links": aboutSeed.RepresentativeLinks}, http.StatusOK)
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
	file := os.Getenv("BLOG_PORTFOLIO_BROWSER_FIXTURE")
	if file == "" {
		file = os.Getenv("BLOG_WECHAT_BROWSER_FIXTURE")
	}
	if file != "" {
		if e := os.WriteFile(file, []byte(server.URL), 0600); e != nil {
			t.Fatal(e)
		}
		deadline := time.After(5 * time.Minute)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-deadline:
				return
			case <-ticker.C:
				if _, e := os.Stat(file + ".stop"); e == nil {
					return
				}
			}
		}
	}
}
