package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func TestRefreshCookieAttributesFollowAuthConfig(t *testing.T) {
	tests := []struct {
		name   string
		secure bool
	}{
		{name: "local HTTP", secure: false},
		{name: "production HTTPS", secure: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewAuthService(nil, nil, &conf.Auth{SecureCookie: tt.secure})
			cookie := svc.refreshCookie("opaque-refresh-token", time.Hour)
			if cookie.Secure != tt.secure {
				t.Fatalf("Secure = %v, want %v", cookie.Secure, tt.secure)
			}
			if !cookie.HttpOnly {
				t.Fatal("HttpOnly = false, want true")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("SameSite = %v, want Lax", cookie.SameSite)
			}
			if cookie.Path != refreshCookiePath {
				t.Fatalf("Path = %q, want %q", cookie.Path, refreshCookiePath)
			}
			if cookie.MaxAge <= 0 {
				t.Fatalf("MaxAge = %d, want positive", cookie.MaxAge)
			}
		})
	}
}

func TestLogoutCookieUsesConfigAndIgnoresForwardedProto(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP config", true: "HTTPS config"}[configured], func(t *testing.T) {
			srv := kratoshttp.NewServer()
			v1.RegisterAuthServiceHTTPServer(srv, NewAuthService(nil, nil, &conf.Auth{SecureCookie: configured}))
			ts := httptest.NewServer(srv)
			defer ts.Close()

			req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/auth/logout", strings.NewReader("{}"))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			if configured {
				req.Header.Set("X-Forwarded-Proto", "http")
			} else {
				req.Header.Set("X-Forwarded-Proto", "https")
			}
			resp, err := ts.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			cookies := resp.Cookies()
			if len(cookies) != 1 {
				t.Fatalf("cookies = %v, want one refresh deletion cookie", cookies)
			}
			cookie := cookies[0]
			if cookie.Name != refreshCookieName || cookie.Value != "" || cookie.MaxAge != -1 {
				t.Fatalf("deletion cookie = %+v", cookie)
			}
			if cookie.Secure != configured {
				t.Fatalf("configured=%v forged X-Forwarded-Proto=%q produced Secure=%v", configured, req.Header.Get("X-Forwarded-Proto"), cookie.Secure)
			}
			if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != refreshCookiePath {
				t.Fatalf("security attributes = %+v", cookie)
			}
		})
	}
}

// 伪造的最左跳不能成为限流键：反代链每跳都在右侧追加，最右才是可信代理
// 看到的真实客户端地址（2026-10 review P1 的回归锚点）。
func TestRightmostForwardedIP(t *testing.T) {
	cases := []struct {
		name string
		xff  string
		want string
	}{
		{"single hop", "203.0.113.7", "203.0.113.7"},
		{"spoofed first hop", "9.9.9.9, 203.0.113.7", "203.0.113.7"},
		{"multiple hops", "10.0.0.1, 10.0.0.2, 203.0.113.7", "203.0.113.7"},
		{"trailing comma", "203.0.113.7, ", "203.0.113.7"},
		{"extra whitespace", "  9.9.9.9 ,  203.0.113.7  ", "203.0.113.7"},
		{"empty", "", ""},
		{"only commas", ", ,", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rightmostForwardedIP(tc.xff); got != tc.want {
				t.Fatalf("rightmostForwardedIP(%q) = %q, want %q", tc.xff, got, tc.want)
			}
		})
	}
}
