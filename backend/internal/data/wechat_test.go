package data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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

	c := &wechatClient{appID: "wx-test", appSecret: "secret-test", baseURL: srv.URL, http: srv.Client()}
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
