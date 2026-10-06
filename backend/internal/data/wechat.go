package data

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/redis/go-redis/v9"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

// wechatBindKeyPrefix namespaces the single-use binding tickets in Redis.
const wechatBindKeyPrefix = "blog:wechatbind:"

// wechatClient talks to the WeChat mini-program code2session endpoint.
type wechatClient struct {
	appID     string
	appSecret string
	baseURL   string // overridable in tests
	http      *http.Client
}

// NewWechatClient creates the WechatClient. Missing credentials are not an
// error at construction time: the endpoints stay wired and fail with a
// clear domain error per call, so a misconfigured secret can never take the
// whole service down.
func NewWechatClient(c *conf.Wechat) biz.WechatClient {
	return &wechatClient{
		appID:     c.GetAppId(),
		appSecret: c.GetAppSecret(),
		baseURL:   "https://api.weixin.qq.com",
		http:      &http.Client{Timeout: 5 * time.Second},
	}
}

// code2SessionResponse is the subset of WeChat's jscode2session reply this
// client consumes. SessionKey is deliberately not persisted or logged: this
// service never needs it (no phone-number decryption).
type code2SessionResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func (c *wechatClient) Code2Session(ctx context.Context, code string) (string, error) {
	if c.appID == "" || c.appSecret == "" || code == "" {
		return "", biz.ErrWechatNotConfigured
	}
	url := fmt.Sprintf("%s/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		c.baseURL, c.appID, c.appSecret, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	var body code2SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("wechat: decode jscode2session response: %w", err)
	}
	if body.ErrCode != 0 {
		// 40029 invalid code, 40163 code already used, 45011 api rate limit —
		// all resolve to "run wx.login again" from the client's perspective.
		if body.ErrCode == 40029 || body.ErrCode == 40163 {
			return "", biz.ErrWechatCodeInvalid
		}
		return "", errors.InternalServer("WECHAT_UPSTREAM", fmt.Sprintf("wechat jscode2session failed: %d %s", body.ErrCode, body.ErrMsg))
	}
	if body.OpenID == "" {
		return "", errors.InternalServer("WECHAT_UPSTREAM", "wechat jscode2session returned no openid")
	}
	return body.OpenID, nil
}

// wechatBindingStore keeps single-use tickets in Redis with GETDEL consume,
// the same atomicity rule as the refresh session store.
type wechatBindingStore struct {
	rdb redis.UniversalClient
}

// NewWechatBindingStore creates a WechatBindingStore.
func NewWechatBindingStore(rdb redis.UniversalClient) biz.WechatBindingStore {
	return &wechatBindingStore{rdb: rdb}
}

func (s *wechatBindingStore) Save(ctx context.Context, openid string, ttl time.Duration) (string, error) {
	ticket, err := newOpaqueToken()
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, wechatBindKeyPrefix+ticket, openid, ttl).Err(); err != nil {
		return "", err
	}
	return ticket, nil
}

func (s *wechatBindingStore) Consume(ctx context.Context, ticket string) (string, error) {
	openid, err := s.rdb.GetDel(ctx, wechatBindKeyPrefix+ticket).Result()
	if err != nil {
		if stderrors.Is(err, redis.Nil) {
			return "", biz.ErrWechatTicketInvalid
		}
		return "", err
	}
	return openid, nil
}
