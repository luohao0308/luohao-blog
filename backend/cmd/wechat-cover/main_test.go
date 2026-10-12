package main

import (
	"context"
	"encoding/json"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupFixture(t *testing.T) (*conf.Bootstrap, string, string) {
	t.Helper()
	dir := t.TempDir()
	return &conf.Bootstrap{OfficialAccount: &conf.OfficialAccount{AppId: "fixture-app", AppSecret: "private-fixture"}}, dir, filepath.Join(dir, "state.json")
}
func TestCoverSetupSavesOnlyPublicFieldsAndReusesJournal(t *testing.T) {
	bc, dir, journal := setupFixture(t)
	calls := 0
	uploader := func(context.Context, *conf.Bootstrap, []byte, string) (string, error) {
		calls++
		return "fixture-permanent-cover", nil
	}
	for range 2 {
		state, err := setup(context.Background(), bc, []byte("cover"), "http://193.112.128.245", journal, dir, uploader)
		if err != nil || state.MediaID != "fixture-permanent-cover" {
			t.Fatal(state, err)
		}
	}
	if calls != 1 {
		t.Fatal("cover uploaded twice")
	}
	for _, path := range []string{journal, filepath.Join(dir, "zz-official-account-setup.local.yaml")} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "private-fixture") {
			t.Fatal("secret persisted")
		}
	}
	if bc.GetOfficialAccount().GetEnabled() {
		t.Fatal("enabled publishing")
	}
}
func TestCoverSetupUnknownCannotReplay(t *testing.T) {
	bc, dir, journal := setupFixture(t)
	calls := 0
	uploader := func(context.Context, *conf.Bootstrap, []byte, string) (string, error) {
		calls++
		return "", &biz.WechatRemoteError{Code: "unknown", Ambiguous: true}
	}
	for range 2 {
		if _, err := setup(context.Background(), bc, []byte("cover"), "http://193.112.128.245", journal, dir, uploader); err == nil {
			t.Fatal("unknown accepted")
		}
	}
	if calls != 1 {
		t.Fatal("unknown replayed")
	}
}
func TestCoverSetupDefinitiveFailureCanRetry(t *testing.T) {
	bc, dir, journal := setupFixture(t)
	calls := 0
	uploader := func(context.Context, *conf.Bootstrap, []byte, string) (string, error) {
		calls++
		if calls == 1 {
			return "", &biz.WechatRemoteError{Code: "40164"}
		}
		return "cover-id", nil
	}
	if _, err := setup(context.Background(), bc, []byte("cover"), "http://193.112.128.245", journal, dir, uploader); err == nil {
		t.Fatal("whitelist error ignored")
	}
	if _, err := setup(context.Background(), bc, []byte("cover"), "http://193.112.128.245", journal, dir, uploader); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}
func TestCoverSetupDoesNotReuseOtherAccount(t *testing.T) {
	bc, dir, journal := setupFixture(t)
	calls := 0
	uploader := func(context.Context, *conf.Bootstrap, []byte, string) (string, error) {
		calls++
		return "cover-id", nil
	}
	if _, err := setup(context.Background(), bc, []byte("cover"), "http://193.112.128.245", journal, dir, uploader); err != nil {
		t.Fatal(err)
	}
	bc.OfficialAccount.AppId = "other-account"
	if _, err := setup(context.Background(), bc, []byte("cover"), "http://193.112.128.245", journal, dir, uploader); err == nil {
		t.Fatal("account mismatch ignored")
	}
	if calls != 1 {
		t.Fatal("wrong account called")
	}
}
func TestGeneratedSetupOverridesEmptyFieldsWithoutChangingSecret(t *testing.T) {
	_, dir, _ := setupFixture(t)
	write := func(name string, body any) {
		t.Helper()
		data, _ := json.Marshal(body)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("config.yaml", map[string]any{"official_account": map[string]any{"enabled": false, "site_url": "", "default_thumb_media_id": ""}})
	write("secrets.local.yaml", map[string]any{"official_account": map[string]any{"app_id": "fixture-app", "app_secret": "private-fixture", "site_url": ""}})
	if err := writeSetup(dir, "http://193.112.128.245", "cover-id", "fixture-app"); err != nil {
		t.Fatal(err)
	}
	c := config.New(config.WithSource(file.NewSource(dir)))
	defer func() { _ = c.Close() }()
	if err := c.Load(); err != nil {
		t.Fatal("config load failed")
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		t.Fatal("config decode failed")
	}
	if bc.GetOfficialAccount().GetAppSecret() != "private-fixture" || bc.GetOfficialAccount().GetSiteUrl() != "http://193.112.128.245" || bc.GetOfficialAccount().GetDefaultThumbMediaId() != "cover-id" || bc.GetOfficialAccount().GetEnabled() {
		t.Fatal("setup merge changed secret or enabled sending")
	}
}

func TestCoverSetupCorruptStatusNeverUploads(t *testing.T) {
	for _, value := range []string{"garbage", "uploaded"} {
		t.Run(value, func(t *testing.T) {
			bc, dir, journal := setupFixture(t)
			calls := 0
			uploader := func(context.Context, *conf.Bootstrap, []byte, string) (string, error) {
				calls++
				return "cover-id", nil
			}
			state, err := setup(context.Background(), bc, []byte("cover"), "http://193.112.128.245", journal, dir, func(context.Context, *conf.Bootstrap, []byte, string) (string, error) {
				return "", &biz.WechatRemoteError{Code: "40164"}
			})
			if err == nil {
				t.Fatal("expected preparation failure")
			}
			state.Status = value
			state.MediaID = ""
			if err := writeState(journal, state); err != nil {
				t.Fatal(err)
			}
			if _, err := setup(context.Background(), bc, []byte("cover"), "http://193.112.128.245", journal, dir, uploader); err == nil {
				t.Fatal("corrupt journal accepted")
			}
			if calls != 0 {
				t.Fatal("corrupt state repeated remote write")
			}
		})
	}
}
