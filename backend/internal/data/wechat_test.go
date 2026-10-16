package data

import (
	"context"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

// The client hits baseURL (swapped for a test server here) exactly as it
// hits api.weixin.qq.com in production.
func TestWechatClientCode2Session(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/jscode2session" {
			t.Errorf("path = %q, want /sns/jscode2session", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("appid") != "wx-test" || q.Get("secret") != "secret-test" || q.Get("grant_type") != "authorization_code" {
			t.Errorf("query = %v, want appid/secret/grant_type", q)
		}
		switch q.Get("js_code") {
		case "good":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"openid":"openid-abc","session_key":"sk"}`))
		case "invalid":
			_, _ = w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
		case "replayed":
			_, _ = w.Write([]byte(`{"errcode":40163,"errmsg":"code been used"}`))
		default:
			_, _ = w.Write([]byte(`{"errcode":45011,"errmsg":"rate limited"}`))
		}
	}))
	defer srv.Close()

	c := &wechatClient{enabled: true, appID: "wx-test", appSecret: "secret-test", baseURL: srv.URL, http: srv.Client()}
	ctx := context.Background()

	openid, err := c.Code2Session(ctx, "good")
	if err != nil || openid != "openid-abc" {
		t.Fatalf("Code2Session(good) = %q, %v; want openid-abc", openid, err)
	}
	for _, code := range []string{"invalid", "replayed"} {
		if _, err := c.Code2Session(ctx, code); !kratoserrors.IsUnauthorized(err) {
			t.Fatalf("Code2Session(%s) error = %v, want unauthorized", code, err)
		}
	}
	if _, err := c.Code2Session(ctx, "other"); err == nil || kratoserrors.IsUnauthorized(err) {
		t.Fatalf("Code2Session(upstream failure) error = %v, want an internal error", err)
	}
}

func TestWechatClientNotConfigured(t *testing.T) {
	c := NewWechatClient(nil)
	if _, err := c.Code2Session(context.Background(), "code"); !kratoserrors.Is(err, biz.ErrWechatNotConfigured) {
		t.Fatalf("Code2Session(unconfigured) error = %v, want ErrWechatNotConfigured", err)
	}
}

func TestWechatClientExplicitConfiguration(t *testing.T) {
	for _, cfg := range []*conf.Wechat{nil, {}, {AppId: "wx-test", AppSecret: "secret"}, {Enabled: true, AppId: "wx-test"}, {Enabled: true, AppSecret: "secret"}, {Enabled: true, AppId: "wx:test", AppSecret: "secret"}} {
		c := NewWechatClient(cfg)
		if c.Configured() {
			t.Fatalf("incomplete/disabled client configured")
		}
		if _, err := c.Code2Session(context.Background(), "code"); !kratoserrors.Is(err, biz.ErrWechatNotConfigured) {
			t.Fatal(err)
		}
	}
	var nilClient *wechatClient
	if nilClient.Configured() || nilClient.AppID() != "" {
		t.Fatal("nil client must be disabled")
	}
	c := NewWechatClient(&conf.Wechat{Enabled: true, AppId: "wx-test", AppSecret: "secret"})
	if !c.Configured() || c.AppID() != "wx-test" {
		t.Fatal("complete enabled config must work")
	}
}

func TestWechatClientEscapesAndRedacts(t *testing.T) {
	const secret = "secret&js_code=injected"
	const code = "code+&secret=leak"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("secret") != secret || r.URL.Query().Get("js_code") != code {
			t.Error("query escaping failed")
		}
		_, _ = w.Write([]byte(`{"errcode":45011,"errmsg":"secret&js_code=injected code+&secret=leak"}`))
	}))
	c := &wechatClient{enabled: true, appID: "wx-test", appSecret: secret, baseURL: srv.URL, http: srv.Client()}
	_, err := c.Code2Session(context.Background(), code)
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), code) {
		t.Fatalf("unsafe upstream error: %v", err)
	}
	srv.Close()
	_, err = c.Code2Session(context.Background(), code)
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "js_code") {
		t.Fatalf("unsafe transport error: %v", err)
	}
}

func exerciseWechatBindingStore(t *testing.T, rdb redis.UniversalClient) {
	t.Helper()
	ctx := context.Background()
	store := NewWechatBindingStore(rdb)
	const app = "wx-ticket-test"
	ticket, err := store.Save(ctx, app, "openid-test", biz.DefaultWechatBindingTTL)
	if err != nil {
		t.Fatal(err)
	}
	key := wechatBindKeyPrefix + app + ":" + ticket
	t.Cleanup(func() { _ = rdb.Del(ctx, key).Err() })
	ttl, err := rdb.PTTL(ctx, key).Result()
	if err != nil || ttl <= 9*time.Minute || ttl > 10*time.Minute {
		t.Fatalf("TTL = %v, %v", ttl, err)
	}
	if _, err := store.Consume(ctx, "wx-other", ticket); !kratoserrors.Is(err, biz.ErrWechatTicketInvalid) {
		t.Fatal("cross-app ticket accepted")
	}
	if id, err := store.Consume(ctx, app, ticket); err != nil || id != "openid-test" {
		t.Fatalf("consume = %q,%v", id, err)
	}
	if _, err := store.Consume(ctx, app, ticket); !kratoserrors.Is(err, biz.ErrWechatTicketInvalid) {
		t.Fatal("replayed ticket accepted")
	}
	if rdb.Exists(ctx, key).Val() != 0 {
		t.Fatal("GETDEL did not delete ticket")
	}

	concurrentTicket, err := store.Save(ctx, app, "openid-concurrent", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := store.Consume(ctx, app, concurrentTicket)
			if err == nil && id == "openid-concurrent" {
				winners.Add(1)
			} else if !kratoserrors.Is(err, biz.ErrWechatTicketInvalid) {
				t.Errorf("unexpected concurrent consume error: %v", err)
			}
		}()
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("concurrent consume winners = %d, want 1", winners.Load())
	}
	for _, value := range []string{`legacy-openid`, `{"app_id":"wx-other","openid":"openid-test"}`, `{"app_id":"wx-ticket-test","openid":""}`, `{`} {
		bad, err := newOpaqueToken()
		if err != nil {
			t.Fatal(err)
		}
		key := wechatBindKeyPrefix + app + ":" + bad
		if err := rdb.Set(ctx, key, value, time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Consume(ctx, app, bad); !kratoserrors.Is(err, biz.ErrWechatTicketInvalid) {
			t.Fatal("malformed payload accepted")
		}
		if rdb.Exists(ctx, key).Val() != 0 {
			t.Fatal("malformed payload not consumed")
		}
	}
	old, err := newOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	legacyKey := "blog:wechatbind:" + old
	if err := rdb.Set(ctx, legacyKey, "openid-test", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.Del(ctx, legacyKey).Err() })
	if _, err := store.Consume(ctx, app, old); !kratoserrors.Is(err, biz.ErrWechatTicketInvalid) {
		t.Fatal("legacy namespace accepted")
	}
	if rdb.Exists(ctx, legacyKey).Val() != 1 {
		t.Fatal("legacy key changed")
	}
	for _, bad := range []string{"", "short", strings.Repeat("a", 44), strings.Repeat("a", 42) + "/", ticket + ":"} {
		if _, err := store.Consume(ctx, app, bad); !kratoserrors.Is(err, biz.ErrWechatTicketInvalid) {
			t.Fatal("malformed ticket accepted")
		}
	}
	for _, ttl := range []time.Duration{0, -time.Second, 11 * time.Minute} {
		if _, err := store.Save(ctx, app, "openid-test", ttl); !kratoserrors.Is(err, biz.ErrWechatTicketInvalid) {
			t.Fatal("unsafe TTL accepted")
		}
	}
	if _, err := store.Save(ctx, "wx:bad", "openid-test", time.Minute); !kratoserrors.Is(err, biz.ErrWechatTicketInvalid) {
		t.Fatal("namespace injection accepted")
	}
}

func TestWechatBindingStore(t *testing.T) {
	rdb, _ := newTestRedis(t)
	exerciseWechatBindingStore(t, rdb)
}

// REDIS_INTEGRATION_ADDR must point to an isolated local Redis instance. The
// release verification runs this explicitly; normal unit tests need no daemon.
func TestWechatBindingStoreRealRedis(t *testing.T) {
	addr := os.Getenv("REDIS_INTEGRATION_ADDR")
	if addr == "" {
		t.Skip("set REDIS_INTEGRATION_ADDR for real Redis validation")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	exerciseWechatBindingStore(t, rdb)
	store := NewWechatBindingStore(rdb)
	ticket, err := store.Save(context.Background(), "wx-expiry", "openid-expiry", 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := store.Consume(context.Background(), "wx-expiry", ticket); !kratoserrors.Is(err, biz.ErrWechatTicketInvalid) {
		t.Fatal("expired ticket accepted")
	}
}
