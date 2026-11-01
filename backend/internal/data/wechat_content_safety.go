package data

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/redis/go-redis/v9"
)

// The separate opt-in is deliberately unwired during S1. Mini-program tokens
// must never share the official-account or login/binding token namespace.
// Supply a dedicated context-enabled Redis client; its owner closes it. Shared
// clients are rejected without mutating their options. Deadline cancellation
// bounds socket I/O; cancellation without a deadline is bounded by total 5s.
type wechatContentSafetyClient struct {
	enabled                bool
	appID, secret, baseURL string
	http                   *http.Client
	redis                  *redis.Client
	tokenGate              chan struct{}
}

func NewWechatContentSafetyClient(c *conf.Wechat, r *redis.Client, enabled bool) biz.ContentSafetyChecker {
	return &wechatContentSafetyClient{enabled: enabled && c.GetEnabled(), appID: c.GetAppId(), secret: c.GetAppSecret(), baseURL: "https://api.weixin.qq.com", redis: r, tokenGate: make(chan struct{}, 1), http: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *wechatContentSafetyClient) AppID() string {
	if c == nil {
		return ""
	}
	return c.appID
}
func (c *wechatContentSafetyClient) Configured() bool {
	return c != nil && c.enabled && validWechatIdentifier(c.appID, 32) && strings.TrimSpace(c.secret) != "" && c.redis != nil && c.redis.Options().ContextTimeoutEnabled
}
func (c *wechatContentSafetyClient) tokenKey() string {
	return "blog:wechat:content-safety:token:v1:" + c.appID
}
func (c *wechatContentSafetyClient) post(ctx context.Context, path string, payload any, dst any) error {
	body, e := json.Marshal(payload)
	if e != nil {
		return biz.ErrContentSafetyUnavailable
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if e != nil {
		return biz.ErrContentSafetyUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := c.http.Do(req)
	if e != nil {
		return biz.ErrContentSafetyUnavailable
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return biz.ErrContentSafetyUnavailable
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if e != nil || len(raw) > 64<<10 || json.Unmarshal(raw, dst) != nil {
		return biz.ErrContentSafetyUnavailable
	}
	return nil
}
func validSafetyToken(s string) bool {
	if len(s) == 0 || len(s) > 2048 {
		return false
	}
	for _, r := range s {
		if r <= 32 || r > 126 {
			return false
		}
	}
	return true
}
func (c *wechatContentSafetyClient) token(ctx context.Context) (string, error) {
	select {
	case c.tokenGate <- struct{}{}:
		defer func() { <-c.tokenGate }()
	case <-ctx.Done():
		return "", biz.ErrContentSafetyUnavailable
	}
	token, e := c.redis.Get(ctx, c.tokenKey()).Result()
	if e == nil {
		if !validSafetyToken(token) {
			return "", biz.ErrContentSafetyUnavailable
		}
		return token, nil
	}
	if e != redis.Nil {
		return "", biz.ErrContentSafetyUnavailable
	}
	var response struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
	}
	e = c.post(ctx, "/cgi-bin/stable_token", map[string]any{"grant_type": "client_credential", "appid": c.appID, "secret": c.secret, "force_refresh": false}, &response)
	if e != nil || response.ErrCode != 0 || !validSafetyToken(response.AccessToken) || response.ExpiresIn <= 0 || response.ExpiresIn > 7200 {
		return "", biz.ErrContentSafetyUnavailable
	}
	ttl := time.Duration(response.ExpiresIn) * time.Second * 9 / 10
	if c.redis.Set(ctx, c.tokenKey(), response.AccessToken, ttl).Err() != nil {
		return "", biz.ErrContentSafetyUnavailable
	}
	return response.AccessToken, nil
}
func (c *wechatContentSafetyClient) CheckText(ctx context.Context, input biz.ContentSafetyInput) (biz.ContentSafetyResult, error) {
	zero := biz.ContentSafetyResult{}
	if !c.Configured() {
		return zero, biz.ErrContentSafetyDisabled
	}
	if !validWechatIdentifier(input.OpenID, 64) {
		return zero, biz.ErrContentSafetyIdentity
	}
	// The API says 2500 characters; rune count is an implementation
	// interpretation, not a claim about the upstream's grapheme accounting.
	if strings.TrimSpace(input.Content) == "" || !utf8.ValidString(input.Content) || utf8.RuneCountInString(input.Content) > 2500 || (input.Scene != biz.ContentSafetyProfile && input.Scene != biz.ContentSafetyComment) {
		return zero, biz.ErrContentSafetyInput
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	token, e := c.token(ctx)
	if e != nil {
		return zero, e
	}
	var response struct {
		ErrCode *int `json:"errcode"`
		Result  *struct {
			Suggest string `json:"suggest"`
			Label   int    `json:"label"`
		} `json:"result"`
		TraceID string `json:"trace_id"`
	}
	e = c.post(ctx, "/wxa/msg_sec_check?access_token="+url.QueryEscape(token), map[string]any{"version": 2, "scene": int(input.Scene), "openid": input.OpenID, "content": input.Content}, &response)
	if e != nil {
		return zero, e
	}
	if response.ErrCode == nil {
		return zero, biz.ErrContentSafetyUnavailable
	}
	switch *response.ErrCode {
	case 87014:
		// The generic WeChat error table identifies this as rejected content.
		// It has no v2 success trace; keep it terminal and non-public instead
		// of offering repeated retries as if the service were unavailable.
		return biz.ContentSafetyResult{Decision: biz.ContentSafetyRisky}, nil
	case 40001, 40014, 42001:
		// Do not invalidate a token installed concurrently by another instance.
		_ = c.redis.Eval(ctx, `if redis.call('GET',KEYS[1]) == ARGV[1] then return redis.call('DEL',KEYS[1]) end return 0`, []string{c.tokenKey()}, token).Err()
		return zero, biz.ErrContentSafetyUnavailable
	case 40003, 43104, 61010:
		return zero, biz.ErrContentSafetyIdentity
	case 0:
	default:
		return zero, biz.ErrContentSafetyUnavailable
	}
	if response.Result == nil || !validSafetyTrace(response.TraceID) {
		return zero, biz.ErrContentSafetyUnavailable
	}
	decision := biz.ContentSafetyDecision(response.Result.Suggest)
	if decision != biz.ContentSafetyPass && decision != biz.ContentSafetyReview && decision != biz.ContentSafetyRisky {
		return zero, biz.ErrContentSafetyUnavailable
	}
	return biz.ContentSafetyResult{Decision: decision, TraceID: response.TraceID, Label: response.Result.Label}, nil
}

// Trace IDs are opaque upstream metadata, not WeChat identity namespaces.
func validSafetyTrace(value string) bool {
	if value == "" || len(value) > 256 || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
