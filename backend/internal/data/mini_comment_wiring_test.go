package data

import (
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestWechatSafetyCacheIsDedicatedAndContextBounded(t *testing.T) {
	cfg := &conf.Data{Redis: &conf.Data_Redis{Addr: "127.0.0.1:1", ReadTimeout: durationpb.New(2 * time.Second), WriteTimeout: durationpb.New(2 * time.Second)}}
	shared := NewRedis(cfg)
	defer func() { _ = shared.Close() }()
	cache, cleanup := NewWechatSafetyCache(cfg)
	defer cleanup()
	opts := cache.client.Options()
	if !opts.ContextTimeoutEnabled || opts.ReadTimeout > 200*time.Millisecond || opts.WriteTimeout > 200*time.Millisecond {
		t.Fatal("safety cache ignores context or has unbounded I/O")
	}
	client := shared.(*redis.Client)
	if client.Options().ContextTimeoutEnabled || client.Options().ReadTimeout != 2*time.Second {
		t.Fatal("shared website Redis options were changed")
	}
}

func TestConfiguredWechatSafetyIsExplicitAndRefusesDebugLogging(t *testing.T) {
	cfg := &conf.Data{Redis: &conf.Data_Redis{Addr: "127.0.0.1:1"}}
	cache, cleanup := NewWechatSafetyCache(cfg)
	defer cleanup()
	wc := &conf.Wechat{Enabled: true, AppId: "wx-fixture", AppSecret: "synthetic-fixture"}
	checker, err := NewConfiguredWechatContentSafetyClient(wc, cfg, cache)
	if err != nil || checker.Configured() {
		t.Fatal("default unexpectedly enabled upstream checks")
	}
	wc.ContentSafetyEnabled = true
	checker, err = NewConfiguredWechatContentSafetyClient(wc, cfg, cache)
	if err != nil || !checker.Configured() {
		t.Fatalf("explicit opt-in unavailable: %v", err)
	}
	cfg.Database = &conf.Data_Database{Debug: true}
	if _, err := NewConfiguredWechatContentSafetyClient(wc, cfg, cache); err == nil {
		t.Fatal("database debug could log private identity parameters")
	}
}
