package data

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

func NewWechatSyncSettings(bc *conf.Bootstrap) biz.WechatSyncSettings {
	c := bc.GetOfficialAccount()
	mode := strings.ToLower(strings.TrimSpace(c.GetMode()))
	if mode == "" {
		mode = "draft"
	}
	fingerprint := sha256.Sum256([]byte(strings.TrimSpace(c.GetAppId())))
	return biz.WechatSyncSettings{AccountFingerprint: hex.EncodeToString(fingerprint[:]), Enabled: c.GetEnabled(), Configured: c.GetAppId() != "" && c.GetAppSecret() != "" && c.GetSiteUrl() != "" && c.GetDefaultThumbMediaId() != "", Mode: mode, SiteURL: strings.TrimRight(strings.TrimSpace(c.GetSiteUrl()), "/"), ThumbMediaID: strings.TrimSpace(c.GetDefaultThumbMediaId()), Author: strings.TrimSpace(c.GetAuthor()), AccountName: c.GetAccountName(), AccountID: c.GetAccountId()}
}
