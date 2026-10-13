package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
)

func TestDomainOverlayPreservesWechatSettingsAndBuildsHTTPSOriginalLink(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KRATOS_AUTH_SECURE_COOKIE", "true")
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("config.yaml", "official_account:\n  enabled: true\n  mode: draft\n  app_id: fixture-app\n  app_secret: fixture-secret\n  default_thumb_media_id: fixture-cover\n  author: fixture-author\n  image_hosts: [images.example.com]\n")
	write("zz-official-account-setup.local.yaml", "official_account:\n  site_url: http://193.112.128.245\n  default_thumb_media_id: fixture-cover\n")
	overlay, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", "official-account-domain.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	write("zzzz-domain-origin.local.yaml", string(overlay))
	b := loadBootstrap(t, dir)
	c := b.GetOfficialAccount()
	if c.GetSiteUrl() != "https://makerhao.cn" || !c.GetEnabled() || c.GetMode() != "draft" || c.GetAppId() != "fixture-app" || c.GetAppSecret() != "fixture-secret" || c.GetDefaultThumbMediaId() != "fixture-cover" || c.GetAuthor() != "fixture-author" || len(c.GetImageHosts()) != 1 || c.GetImageHosts()[0] != "images.example.com" || !b.GetAuth().GetSecureCookie() {
		t.Fatal("domain overlay changed unrelated settings or failed to replace old origin")
	}
	publisher, err := data.NewOfficialAccountPublisher(b)
	if err != nil {
		t.Fatal(err)
	}
	settings := data.NewWechatSyncSettings(b)
	uc := biz.NewWechatSyncUsecase(nil, nil, publisher, settings)
	task := uc.Task(&biz.Article{ID: uuid.New(), Slug: "domain-check", Title: "domain check", ContentHTML: "<p>domain check</p>", Status: biz.ArticleStatusPublished})
	if task == nil || task.Article.SourceURL != "https://makerhao.cn/posts/domain-check" || task.Mode != "draft" {
		t.Fatal("new draft task must use HTTPS original link and retain draft mode")
	}
}
