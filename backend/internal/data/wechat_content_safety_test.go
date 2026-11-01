package data

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/redis/go-redis/v9"
)

func safetyFixture(t *testing.T, h http.HandlerFunc) (*wechatContentSafetyClient, *miniredis.Miniredis) {
	t.Helper()
	m := miniredis.RunT(t)
	r := redis.NewClient(&redis.Options{Addr: m.Addr(), ContextTimeoutEnabled: true})
	t.Cleanup(func() { _ = r.Close() })
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	c := NewWechatContentSafetyClient(&conf.Wechat{Enabled: true, AppId: "wx-fixture", AppSecret: "secret-fixture"}, r, true).(*wechatContentSafetyClient)
	c.baseURL = s.URL
	return c, m
}
func TestWechatContentSafetyDecisions(t *testing.T) {
	var calls atomic.Int32
	c, m := safetyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		if r.Method != "POST" {
			t.Error("method")
		}
		if r.URL.Path == "/cgi-bin/stable_token" {
			calls.Add(1)
			if b["appid"] != "wx-fixture" || b["secret"] != "secret-fixture" || b["grant_type"] != "client_credential" || b["force_refresh"] == true || r.URL.RawQuery != "" {
				t.Error("token contract")
			}
			_, _ = w.Write([]byte(`{"access_token":"fixture-token","expires_in":345}`))
			return
		}
		if r.URL.Path != "/wxa/msg_sec_check" || r.URL.Query().Get("access_token") != "fixture-token" || b["openid"] != "openid-fixture" || b["version"] != float64(2) || b["scene"] != float64(2) {
			t.Error("check contract")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "result": map[string]any{"suggest": b["content"]}, "trace_id": "trace"})
	})
	for _, d := range []biz.ContentSafetyDecision{biz.ContentSafetyPass, biz.ContentSafetyReview, biz.ContentSafetyRisky} {
		v, e := c.CheckText(context.Background(), biz.ContentSafetyInput{OpenID: "openid-fixture", Content: string(d), Scene: biz.ContentSafetyComment})
		if e != nil || v.Decision != d {
			t.Fatalf("decision: %v", e)
		}
	}
	if calls.Load() != 1 || m.TTL(c.tokenKey()) > 345*time.Second {
		t.Fatal("cache lifetime")
	}
	m.FastForward(346 * time.Second)
	_, _ = c.CheckText(context.Background(), biz.ContentSafetyInput{OpenID: "openid-fixture", Content: "pass", Scene: biz.ContentSafetyComment})
	if calls.Load() != 2 {
		t.Fatal("cache expiry")
	}
}
func TestWechatContentSafetyInvalidResponse(t *testing.T) {
	for _, body := range []string{`{}`, `{"result":{"suggest":"pass"},"trace_id":"x"}`, `{"errcode":0,"result":{"suggest":"unknown"},"trace_id":"x"}`, `{"errcode":0,"result":{"suggest":"pass"}}`, `{"errcode":0,"result":{"suggest":"pass"},"trace_id":"x"} trailing`, `{"errcode":61010,"errmsg":"secret-fixture openid-fixture content-fixture"}`, strings.Repeat("x", 65537)} {
		c, _ := safetyFixture(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/cgi-bin/stable_token" {
				_, _ = w.Write([]byte(`{"access_token":"fixture-token","expires_in":7200}`))
				return
			}
			_, _ = w.Write([]byte(body))
		})
		v, e := c.CheckText(context.Background(), biz.ContentSafetyInput{OpenID: "openid-fixture", Content: "content-fixture", Scene: biz.ContentSafetyComment})
		if e == nil || v.Decision != "" {
			t.Fatal("response allowed")
		}
		for _, s := range []string{"fixture-token", "secret-fixture", "openid-fixture", "content-fixture", c.baseURL} {
			if strings.Contains(e.Error(), s) {
				t.Fatal("leaked error")
			}
		}
	}
}

func TestWechatContentSafetyKnownRiskIsNotRetryableOutage(t *testing.T) {
	c, _ := safetyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			_, _ = w.Write([]byte(`{"access_token":"fixture-token","expires_in":7200}`))
			return
		}
		_, _ = w.Write([]byte(`{"errcode":87014,"errmsg":"synthetic private response"}`))
	})
	result, err := c.CheckText(context.Background(), biz.ContentSafetyInput{OpenID: "openid-fixture", Content: "fixture", Scene: biz.ContentSafetyComment})
	if err != nil || result.Decision != biz.ContentSafetyRisky {
		t.Fatalf("known rejection misclassified as retryable service outage: %v / %v", result.Decision, err)
	}
}

func TestWechatContentSafetyDiagnosticsDoNotExposePrivateFields(t *testing.T) {
	for _, stage := range []string{"stable_token", "msg_sec_check"} {
		t.Run(stage, func(t *testing.T) {
			var output bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
			defer slog.SetDefault(previous)
			c, _ := safetyFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if stage == "msg_sec_check" && r.URL.Path == "/cgi-bin/stable_token" {
					_, _ = w.Write([]byte(`{"access_token":"fixture-token","expires_in":7200}`))
					return
				}
				code := 40125
				if stage == "msg_sec_check" {
					code = 61010
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"errcode": code, "errmsg": "secret-fixture openid-fixture content-fixture private-response-marker fixture-token"})
			})
			if _, err := c.CheckText(context.Background(), biz.ContentSafetyInput{OpenID: "openid-fixture", Content: "content-fixture", Scene: biz.ContentSafetyComment}); err == nil {
				t.Fatal("upstream failure accepted")
			}
			log := output.String()
			code := "40125"
			if stage == "msg_sec_check" {
				code = "61010"
			}
			if !strings.Contains(log, code) || !strings.Contains(log, stage) {
				t.Fatal("safe upstream code is not diagnosable")
			}
			for _, private := range []string{"secret-fixture", "openid-fixture", "content-fixture", "private-response-marker", "fixture-token", c.baseURL} {
				if strings.Contains(log, private) {
					t.Fatal("diagnostic exposed a private field")
				}
			}
		})
	}
}

func TestWechatContentSafetyDisabledAndInput(t *testing.T) {
	var calls atomic.Int32
	c, _ := safetyFixture(t, func(http.ResponseWriter, *http.Request) { calls.Add(1) })
	for _, input := range []biz.ContentSafetyInput{{Content: "x", Scene: biz.ContentSafetyComment}, {OpenID: "invalid:openid", Content: "x", Scene: biz.ContentSafetyComment}, {OpenID: "openid-fixture", Content: "", Scene: biz.ContentSafetyComment}, {OpenID: "openid-fixture", Content: strings.Repeat("文", 2501), Scene: biz.ContentSafetyComment}, {OpenID: "openid-fixture", Content: "x", Scene: 99}} {
		if _, e := c.CheckText(context.Background(), input); e == nil {
			t.Fatal("invalid input accepted")
		}
	}
	for _, cfg := range []*conf.Wechat{nil, {}, {Enabled: true, AppId: "wx-fixture", AppSecret: "secret-fixture"}} {
		checker := NewWechatContentSafetyClient(cfg, c.redis, false)
		checker.(*wechatContentSafetyClient).baseURL = c.baseURL
		if checker.Configured() {
			t.Fatal("opt in required")
		}
		if _, e := checker.CheckText(context.Background(), biz.ContentSafetyInput{}); e == nil {
			t.Fatal("disabled accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input contacted upstream")
	}
}
func TestWechatContentSafetyTransportAndTokenFailure(t *testing.T) {
	for _, mode := range []string{"redirect", "status", "invalid-token", "zero-expiry", "oversize-token", "cache-outage", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			var redirected atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
			defer target.Close()
			c, m := safetyFixture(t, func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "redirect":
					http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
				case "status":
					w.WriteHeader(http.StatusBadGateway)
				case "invalid-token":
					_, _ = w.Write([]byte(`{"access_token":"unsafe\nsecret-fixture","expires_in":7200}`))
				case "zero-expiry":
					_, _ = w.Write([]byte(`{"access_token":"fixture-token","expires_in":0}`))
				case "oversize-token":
					_, _ = w.Write([]byte(strings.Repeat("x", 65537)))
				case "timeout":
					select {
					case <-r.Context().Done():
					case <-time.After(100 * time.Millisecond):
					}
				}
			})
			if mode == "cache-outage" {
				m.Close()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			v, e := c.CheckText(ctx, biz.ContentSafetyInput{OpenID: "openid-fixture", Content: "content-fixture", Scene: biz.ContentSafetyComment})
			if e == nil || v.Decision != "" || redirected.Load() != 0 {
				t.Fatal("failure not closed")
			}
		})
	}
}
func TestWechatContentSafetyTokenInvalidation(t *testing.T) {
	c, m := safetyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":40001,"errmsg":"fixture-token"}`))
	})
	if err := m.Set(c.tokenKey(), "fixture-token"); err != nil {
		t.Fatal(err)
	}
	v, e := c.CheckText(context.Background(), biz.ContentSafetyInput{OpenID: "openid-fixture", Content: "x", Scene: biz.ContentSafetyComment})
	if e == nil || v.Decision != "" || m.Exists(c.tokenKey()) {
		t.Fatal("invalid token must not survive")
	}
}

func TestWechatContentSafetyTokenWaitCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	c, _ := safetyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-time.After(300 * time.Millisecond):
		}
		_, _ = w.Write([]byte(`{"access_token":"fixture-token","expires_in":7200}`))
	})
	done := make(chan error, 1)
	go func() { _, err := c.token(context.Background()); done <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.CheckText(ctx, biz.ContentSafetyInput{OpenID: "openid-fixture", Content: "pass", Scene: biz.ContentSafetyComment})
	elapsed := time.Since(start)
	close(release)
	<-done
	if err == nil || elapsed > 150*time.Millisecond {
		t.Fatalf("canceled token waiter blocked for %s", elapsed)
	}
}

func TestWechatContentSafetyRedisIOCancellation(t *testing.T) {
	for _, command := range []string{"GET", "SET", "EVAL"} {
		t.Run(command, func(t *testing.T) {
			c, m := safetyFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/cgi-bin/stable_token" {
					_, _ = w.Write([]byte(`{"access_token":"fixture-token","expires_in":7200}`))
					return
				}
				_, _ = w.Write([]byte(`{"errcode":40001}`))
			})
			if err := c.redis.Ping(context.Background()).Err(); err != nil {
				t.Fatal(err)
			}
			if command != "SET" {
				if err := m.Set(c.tokenKey(), "fixture-token"); err != nil {
					t.Fatal(err)
				}
			}
			release := make(chan struct{})
			var entered atomic.Bool
			m.Server().SetPreHook(func(_ *server.Peer, cmd string, _ ...string) bool {
				if cmd == command {
					entered.Store(true)
					select {
					case <-release:
					case <-time.After(300 * time.Millisecond):
					}
				}
				return false
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			start := time.Now()
			result, err := c.CheckText(ctx, biz.ContentSafetyInput{OpenID: "openid-fixture", Content: "pass", Scene: biz.ContentSafetyComment})
			elapsed := time.Since(start)
			close(release)
			if !entered.Load() {
				t.Fatal("expected established connection command was not observed")
			}
			if err == nil || result.Decision != "" || elapsed > 150*time.Millisecond {
				t.Fatalf("%s short context waited %s", command, elapsed)
			}
		})
	}
}

func TestWechatContentSafetyRejectsUnboundedSharedCache(t *testing.T) {
	m := miniredis.RunT(t)
	shared := redis.NewClient(&redis.Options{Addr: m.Addr()})
	defer func() { _ = shared.Close() }()
	checker := NewWechatContentSafetyClient(&conf.Wechat{Enabled: true, AppId: "wx-fixture", AppSecret: "secret-fixture"}, shared, true)
	endpoint := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unsafe cache contacted upstream") }))
	defer endpoint.Close()
	checker.(*wechatContentSafetyClient).baseURL = endpoint.URL
	if checker.Configured() {
		t.Fatal("cache without context I/O deadlines must be rejected")
	}
	if _, err := checker.CheckText(context.Background(), biz.ContentSafetyInput{OpenID: "openid-fixture", Content: "pass", Scene: biz.ContentSafetyComment}); err != biz.ErrContentSafetyDisabled {
		t.Fatal("unsafe cache must fail before upstream calls")
	}
	if shared.Options().ContextTimeoutEnabled {
		t.Fatal("constructor mutated shared cache options")
	}
}
