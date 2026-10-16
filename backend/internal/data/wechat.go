package data

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/redis/go-redis/v9"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

const wechatBindKeyPrefix = "blog:wechatbind:v2:"

type wechatClient struct {
	enabled   bool
	appID     string
	appSecret string
	baseURL   string
	http      *http.Client
}

// NewWechatClient leaves endpoints wired but disabled unless explicitly enabled
// with a complete credential pair. Configuration changes require a restart.
func NewWechatClient(c *conf.Wechat) biz.WechatClient {
	return &wechatClient{
		enabled:   c.GetEnabled(),
		appID:     c.GetAppId(),
		appSecret: c.GetAppSecret(),
		baseURL:   "https://api.weixin.qq.com",
		http:      &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *wechatClient) AppID() string {
	if c == nil {
		return ""
	}
	return c.appID
}
func (c *wechatClient) Configured() bool {
	return c != nil && c.enabled && validWechatIdentifier(c.appID, 32) && strings.TrimSpace(c.appSecret) != ""
}

type code2SessionResponse struct {
	OpenID  string `json:"openid"`
	ErrCode int    `json:"errcode"`
}

func (c *wechatClient) Code2Session(ctx context.Context, code string) (string, error) {
	if !c.Configured() {
		return "", biz.ErrWechatNotConfigured
	}
	if code == "" {
		return "", biz.ErrWechatCodeInvalid
	}
	q := url.Values{"appid": {c.appID}, "secret": {c.appSecret}, "js_code": {code}, "grant_type": {"authorization_code"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/sns/jscode2session?"+q.Encode(), nil)
	if err != nil {
		return "", errors.InternalServer("WECHAT_UPSTREAM", "wechat request unavailable")
	}
	resp, err := c.http.Do(req)
	// Transport errors contain the entire request URL, including the secret.
	if err != nil {
		return "", errors.InternalServer("WECHAT_UPSTREAM", "wechat request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", errors.InternalServer("WECHAT_UPSTREAM", "wechat returned an unsuccessful response")
	}
	var body code2SessionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err != nil {
		return "", errors.InternalServer("WECHAT_UPSTREAM", "wechat returned an invalid response")
	}
	if body.ErrCode == 40029 || body.ErrCode == 40163 {
		return "", biz.ErrWechatCodeInvalid
	}
	if body.ErrCode != 0 {
		return "", errors.InternalServer("WECHAT_UPSTREAM", fmt.Sprintf("wechat jscode2session failed: %d", body.ErrCode))
	}
	if !validWechatIdentifier(body.OpenID, 64) {
		return "", errors.InternalServer("WECHAT_UPSTREAM", "wechat returned an invalid openid")
	}
	return body.OpenID, nil
}

// validWechatIdentifier also prevents namespace delimiters and control bytes.
func validWechatIdentifier(value string, maxLen int) bool {
	if value == "" || len(value) > maxLen {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9', ch == '_', ch == '-':
		default:
			return false
		}
	}
	return true
}

func validWechatTicket(ticket string) bool {
	if len(ticket) != 43 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(ticket)
	return err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == ticket
}

type wechatBinding struct {
	AppID  string `json:"app_id"`
	OpenID string `json:"openid"`
}
type wechatBindingStore struct{ rdb redis.UniversalClient }

func NewWechatBindingStore(rdb redis.UniversalClient) biz.WechatBindingStore {
	return &wechatBindingStore{rdb: rdb}
}

func (s *wechatBindingStore) Save(ctx context.Context, appID, openid string, ttl time.Duration) (string, error) {
	if !validWechatIdentifier(appID, 32) || !validWechatIdentifier(openid, 64) || ttl <= 0 || ttl > biz.DefaultWechatBindingTTL {
		return "", biz.ErrWechatTicketInvalid
	}
	ticket, err := newOpaqueToken()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(wechatBinding{AppID: appID, OpenID: openid})
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, wechatBindKeyPrefix+appID+":"+ticket, payload, ttl).Err(); err != nil {
		return "", err
	}
	return ticket, nil
}
func (s *wechatBindingStore) Consume(ctx context.Context, appID, ticket string) (string, error) {
	if !validWechatIdentifier(appID, 32) || !validWechatTicket(ticket) {
		return "", biz.ErrWechatTicketInvalid
	}
	payload, err := s.rdb.GetDel(ctx, wechatBindKeyPrefix+appID+":"+ticket).Result()
	if err != nil {
		if stderrors.Is(err, redis.Nil) {
			return "", biz.ErrWechatTicketInvalid
		}
		return "", err
	}
	var binding wechatBinding
	if json.Unmarshal([]byte(payload), &binding) != nil || binding.AppID != appID || !validWechatIdentifier(binding.OpenID, 64) {
		return "", biz.ErrWechatTicketInvalid
	}
	return binding.OpenID, nil
}
