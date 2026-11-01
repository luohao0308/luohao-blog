package data

import (
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/redis/go-redis/v9"
)

// WechatSafetyCache owns a separate pool; never mutate the existing website
// Redis client's options to satisfy the safety flow's deadline contract.
type WechatSafetyCache struct{ client *redis.Client }

func NewWechatSafetyCache(c *conf.Data) (*WechatSafetyCache, func()) {
	rc := c.GetRedis()
	network := rc.GetNetwork()
	if network == "" {
		network = "tcp"
	}
	bounded := func(d time.Duration) time.Duration {
		if d <= 0 || d > 200*time.Millisecond {
			return 200 * time.Millisecond
		}
		return d
	}
	client := redis.NewClient(&redis.Options{Network: network, Addr: rc.GetAddr(), Password: rc.GetPassword(), ReadTimeout: bounded(rc.GetReadTimeout().AsDuration()), WriteTimeout: bounded(rc.GetWriteTimeout().AsDuration()), DialTimeout: time.Second, ContextTimeoutEnabled: true, MaxRetries: -1})
	return &WechatSafetyCache{client: client}, func() { _ = client.Close() }
}

func NewConfiguredWechatContentSafetyClient(c *conf.Wechat, d *conf.Data, cache *WechatSafetyCache) (biz.ContentSafetyChecker, error) {
	var client *redis.Client
	if cache != nil {
		client = cache.client
	}
	checker := NewWechatContentSafetyClient(c, client, c.GetContentSafetyEnabled())
	if c.GetContentSafetyEnabled() && (!checker.Configured() || d.GetDatabase().GetDebug() || d.GetRedis().GetAddr() == "") {
		// Ent debug output can include identity query arguments. Opt-in must
		// never silently enable this flow with private parameter logging.
		return nil, biz.ErrContentSafetyDisabled
	}
	return checker, nil
}

func NewMiniCommentLimiter(cache *WechatSafetyCache, c *conf.Auth) biz.MiniCommentLimiter {
	if cache == nil || cache.client == nil {
		return nil
	}
	return NewCommentRateLimiter(cache.client, c)
}
