package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/data"
)

func TestWechatEnabledTemplateAndEnvironment(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "config.yaml")
	t.Setenv("KRATOS_WECHAT_APP_ID", "wx-config-fixture")
	t.Setenv("KRATOS_WECHAT_APP_SECRET", "fixture-secret")
	for _, tc := range []struct {
		name, value string
		set, want   bool
	}{
		{name: "default disabled"},
		{name: "explicit true", value: "true", set: true, want: true},
		{name: "explicit false", value: "false", set: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unsetEnvForTest(t, "WECHAT_ENABLED")
			unsetEnvForTest(t, "KRATOS_WECHAT_ENABLED")
			if tc.set {
				t.Setenv("KRATOS_WECHAT_ENABLED", tc.value)
			}
			cfg := loadBootstrap(t, path).GetWechat()
			if cfg.GetEnabled() != tc.want {
				t.Fatalf("enabled = %v, want %v", cfg.GetEnabled(), tc.want)
			}
			if data.NewWechatClient(cfg).Configured() != tc.want {
				t.Fatalf("client configured does not match enabled=%v", tc.want)
			}
		})
	}
}

func TestWechatDisabledYAMLOverride(t *testing.T) {
	unsetEnvForTest(t, "WECHAT_ENABLED")
	unsetEnvForTest(t, "KRATOS_WECHAT_ENABLED")
	dir := t.TempDir()
	base := "wechat:\n  enabled: true\n  app_id: wx-config-fixture\n  app_secret: fixture-secret\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(base), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secrets.local.yaml"), []byte("wechat:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := loadBootstrap(t, dir).GetWechat()
	if cfg.GetEnabled() || data.NewWechatClient(cfg).Configured() {
		t.Fatal("explicit YAML false must disable configured credentials")
	}
	if cfg.GetAppId() != "wx-config-fixture" || cfg.GetAppSecret() != "fixture-secret" {
		t.Fatal("disable override discarded credential pair")
	}
}

func TestWechatEnabledInvalidEnvironmentFailsFast(t *testing.T) {
	t.Setenv("KRATOS_WECHAT_ENABLED", "not-a-boolean")
	c := newTestConfig(filepath.Join("..", "..", "configs", "config.yaml"))
	defer func() { _ = c.Close() }()
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	if _, err := scanBootstrap(c); err == nil || !strings.Contains(err.Error(), "KRATOS_WECHAT_ENABLED") {
		t.Fatalf("invalid WeChat enabled environment accepted: %v", err)
	}
}

func TestWechatEnvironmentCanDisableConfiguredYAML(t *testing.T) {
	t.Setenv("KRATOS_WECHAT_ENABLED", "false")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("wechat:\n  enabled: true\n  app_id: wx-config-fixture\n  app_secret: fixture-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := loadBootstrap(t, path).GetWechat()
	if cfg.GetEnabled() || data.NewWechatClient(cfg).Configured() {
		t.Fatal("environment false must disable YAML true without removing credentials")
	}
	if cfg.GetAppId() != "wx-config-fixture" || cfg.GetAppSecret() != "fixture-secret" {
		t.Fatal("environment disable discarded credential pair")
	}
}

func TestWechatProductionTemplateDisabledAndExplicitEnable(t *testing.T) {
	path := filepath.Join("..", "..", "..", "deploy", "prod-conf", "config.yaml")
	t.Setenv("KRATOS_WECHAT_APP_ID", "wx-production-fixture")
	t.Setenv("KRATOS_WECHAT_APP_SECRET", "fixture-secret")
	unsetEnvForTest(t, "KRATOS_WECHAT_ENABLED")
	cfg := loadBootstrap(t, path).GetWechat()
	if cfg.GetEnabled() || data.NewWechatClient(cfg).Configured() {
		t.Fatal("production template must default disabled")
	}
	t.Setenv("KRATOS_WECHAT_ENABLED", "true")
	cfg = loadBootstrap(t, path).GetWechat()
	if !cfg.GetEnabled() || !data.NewWechatClient(cfg).Configured() {
		t.Fatal("production explicit enable with credential pair failed")
	}
}
