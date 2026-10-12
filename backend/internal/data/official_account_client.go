package data

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	wechatAPIBaseURL      = "https://api.weixin.qq.com"
	wechatRequestTimeout  = 15 * time.Second
	maxWechatResponseSize = 1 << 20
	maxArticleContentSize = 1 << 20
	maxArticleImageSize   = 1 << 20
)

type officialAccountPublisher struct {
	appID               string
	appSecret           string
	siteURL             string
	defaultThumbMediaID string
	author              string
	mode                string
	allowedImages       []allowedImageSource
	baseURL             string
	httpClient          *http.Client
	imageClient         *http.Client
	now                 func() time.Time
	lookupIP            func(context.Context, string) ([]net.IP, error)

	tokenMu      sync.Mutex
	accessToken  string
	tokenExpires time.Time
}

type allowedImageSource struct {
	scheme string
	host   string
}

// NewOfficialAccountPublisher builds the official-account client. Disabled or
// absent configuration is intentionally represented by a nil publisher.
func NewOfficialAccountPublisher(c *conf.Bootstrap) (biz.WechatPublisher, error) {
	if c == nil || c.GetOfficialAccount() == nil || !c.GetOfficialAccount().GetEnabled() {
		return nil, nil
	}
	cfg := c.GetOfficialAccount()
	if len(cfg.GetDefaultThumbMediaId()) > 255 || len(cfg.GetAuthor()) > 255 {
		return nil, errors.New("official account cover ID and author must be at most 255 bytes")
	}
	if strings.TrimSpace(cfg.GetAppId()) == "" || strings.TrimSpace(cfg.GetAppSecret()) == "" {
		return nil, errors.New("official account app_id and app_secret are required")
	}
	if strings.TrimSpace(cfg.GetDefaultThumbMediaId()) == "" {
		return nil, errors.New("official account default_thumb_media_id is required")
	}
	if strings.TrimSpace(cfg.GetSiteUrl()) == "" {
		return nil, errors.New("official account site_url is required")
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.GetMode()))
	if mode == "" {
		mode = "draft"
	}
	if mode != "draft" && mode != "publish" && mode != "mass" {
		return nil, errors.New("official account mode must be draft, publish, or mass")
	}

	lookup := func(ctx context.Context, host string) ([]net.IP, error) {
		return net.DefaultResolver.LookupIP(ctx, "ip", host)
	}
	allowed, siteURL, err := buildImageAllowlist(cfg.GetSiteUrl(), cfg.GetImageHosts())
	if err != nil {
		return nil, err
	}
	p := &officialAccountPublisher{
		appID:               strings.TrimSpace(cfg.GetAppId()),
		appSecret:           strings.TrimSpace(cfg.GetAppSecret()),
		siteURL:             siteURL,
		defaultThumbMediaID: strings.TrimSpace(cfg.GetDefaultThumbMediaId()),
		author:              strings.TrimSpace(cfg.GetAuthor()),
		mode:                mode,
		allowedImages:       allowed,
		baseURL:             wechatAPIBaseURL,
		httpClient:          &http.Client{Timeout: wechatRequestTimeout},
		now:                 time.Now,
		lookupIP:            lookup,
	}
	p.imageClient = newSafeImageClient(wechatRequestTimeout, lookup)
	return p, nil
}

func buildImageAllowlist(site string, hosts []string) ([]allowedImageSource, string, error) {
	var out []allowedImageSource
	normalizedSite := strings.TrimRight(strings.TrimSpace(site), "/")
	if normalizedSite != "" {
		u, err := url.Parse(normalizedSite)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
			return nil, "", errors.New("official account site_url must be an http(s) origin")
		}
		if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
			return nil, "", errors.New("official account site_url must be an origin without path, query, or fragment")
		}
		normalizedSite = u.Scheme + "://" + u.Host
		out = append(out, allowedImageSource{scheme: u.Scheme, host: strings.ToLower(u.Host)})
	}
	for _, item := range hosts {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !strings.Contains(item, "://") {
			item = "https://" + item
		}
		u, err := url.Parse(item)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return nil, "", errors.New("official account image_hosts entries must be HTTPS hosts or origins")
		}
		out = append(out, allowedImageSource{scheme: "https", host: strings.ToLower(u.Host)})
	}
	return out, normalizedSite, nil
}

func newSafeImageClient(timeout time.Duration, lookup func(context.Context, string) ([]net.IP, error)) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("invalid image address")
		}
		ips, err := lookup(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("image host resolution failed")
		}
		for _, ip := range ips {
			if !isPublicIP(ip) {
				return nil, errors.New("image host resolved to a non-public address")
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
	client := &http.Client{Transport: transport, Timeout: timeout}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many image redirects")
		}
		return nil
	}
	return client
}

func isPublicIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range blockedPublicLookalikes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

var blockedPublicLookalikes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

type wechatEnvelope struct {
	ErrCode int64  `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type stableTokenResponse struct {
	wechatEnvelope
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (p *officialAccountPublisher) token(ctx context.Context) (string, error) {
	p.tokenMu.Lock()
	defer p.tokenMu.Unlock()
	if p.accessToken != "" && p.now().Before(p.tokenExpires) {
		return p.accessToken, nil
	}
	payload := struct {
		GrantType    string `json:"grant_type"`
		AppID        string `json:"appid"`
		Secret       string `json:"secret"`
		ForceRefresh bool   `json:"force_refresh"`
	}{GrantType: "client_credential", AppID: p.appID, Secret: p.appSecret}
	var out stableTokenResponse
	if err := p.readJSON(ctx, http.MethodPost, "/cgi-bin/stable_token", payload, &out); err != nil {
		return "", err
	}
	if out.ErrCode != 0 {
		// token holds tokenMu while fetching; do not re-enter invalidation here.
		return "", remoteError(out.ErrCode, false)
	}
	if out.AccessToken == "" || out.ExpiresIn <= 0 {
		return "", &biz.WechatRemoteError{Code: "invalid_token_response"}
	}
	p.accessToken = out.AccessToken
	p.tokenExpires = p.now().Add(time.Duration(out.ExpiresIn)*time.Second - 5*time.Minute)
	return p.accessToken, nil
}

func (p *officialAccountPublisher) CreateDraft(ctx context.Context, article biz.WechatArticle) (string, error) {
	if strings.TrimSpace(article.Title) == "" {
		return "", localRemoteError("title_required")
	}
	if utf8.RuneCountInString(article.Title) > 32 {
		return "", localRemoteError("title_too_long")
	}
	if len(article.ContentHTML) >= maxArticleContentSize {
		return "", localRemoteError("content_too_large")
	}
	thumb := strings.TrimSpace(article.ThumbMediaID)
	if thumb == "" {
		thumb = p.defaultThumbMediaID
	}
	if thumb == "" {
		return "", localRemoteError("cover_required")
	}
	content, err := p.prepareContent(ctx, article.ContentHTML)
	if err != nil {
		return "", err
	}
	if len(content) >= maxArticleContentSize {
		return "", localRemoteError("content_too_large")
	}
	author := strings.TrimSpace(article.Author)
	if author == "" {
		author = p.author
	}
	sourceURL := strings.TrimSpace(article.SourceURL)
	if sourceURL == "" {
		sourceURL = p.siteURL
	}
	payload := map[string]any{"articles": []map[string]any{{
		"article_type":          "news",
		"title":                 article.Title,
		"content":               content,
		"thumb_media_id":        thumb,
		"author":                author,
		"digest":                article.Summary,
		"content_source_url":    sourceURL,
		"need_open_comment":     0,
		"only_fans_can_comment": 0,
	}}}
	var out struct {
		wechatEnvelope
		MediaID string `json:"media_id"`
	}
	if err := p.writeJSON(ctx, "/cgi-bin/draft/add", payload, &out); err != nil {
		return "", err
	}
	if out.ErrCode != 0 {
		return "", p.apiError(out.ErrCode)
	}
	if out.MediaID == "" {
		return "", unknownWriteError()
	}
	return out.MediaID, nil
}

func (p *officialAccountPublisher) Submit(ctx context.Context, draftID, mode, clientmsgid string) (string, error) {
	draftID = strings.TrimSpace(draftID)
	if draftID == "" {
		return "", errors.New("official account draft id is required")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = p.mode
	}
	if mode == "draft" {
		return draftID, nil
	}
	if mode == "publish" {
		var out struct {
			wechatEnvelope
			PublishID string `json:"publish_id"`
		}
		if err := p.writeJSON(ctx, "/cgi-bin/freepublish/submit", map[string]string{"media_id": draftID}, &out); err != nil {
			return "", err
		}
		if out.ErrCode != 0 {
			return "", p.apiError(out.ErrCode)
		}
		if out.PublishID == "" {
			return "", unknownWriteError()
		}
		return out.PublishID, nil
	}
	if mode != "mass" {
		return "", errors.New("official account submit mode must be draft, publish, or mass")
	}
	clientmsgid = stableClientMessageID(clientmsgid, draftID)
	payload := map[string]any{
		"filter":              map[string]bool{"is_to_all": true},
		"mpnews":              map[string]string{"media_id": draftID},
		"msgtype":             "mpnews",
		"clientmsgid":         clientmsgid,
		"send_ignore_reprint": 0,
	}
	var out struct {
		wechatEnvelope
		MessageID json.RawMessage `json:"msg_id"`
	}
	if err := p.writeJSON(ctx, "/cgi-bin/message/mass/sendall", payload, &out); err != nil {
		return "", err
	}
	messageID := rawID(out.MessageID)
	if out.ErrCode == 45065 && messageID != "" {
		return messageID, nil
	}
	if out.ErrCode != 0 {
		return "", p.apiError(out.ErrCode)
	}
	if messageID == "" {
		return "", unknownWriteError()
	}
	return messageID, nil
}

func stableClientMessageID(candidate, draftID string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate != "" && len(candidate) <= 32 {
		return candidate
	}
	sum := sha256.Sum256([]byte(candidate + "\x00" + draftID))
	return hex.EncodeToString(sum[:16])
}

func rawID(raw json.RawMessage) string {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}

func (p *officialAccountPublisher) Status(ctx context.Context, remoteID, mode string) (*biz.WechatPublishResult, error) {
	remoteID = strings.TrimSpace(remoteID)
	if remoteID == "" {
		return nil, errors.New("official account remote id is required")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = p.mode
	}
	if mode == "draft" {
		return &biz.WechatPublishResult{Succeeded: true}, nil
	}
	if mode == "publish" {
		var out struct {
			wechatEnvelope
			PublishStatus *int `json:"publish_status"`
			ArticleDetail struct {
				Items []struct {
					ArticleURL string `json:"article_url"`
				} `json:"item"`
			} `json:"article_detail"`
		}
		if err := p.readAPIJSON(ctx, "/cgi-bin/freepublish/get", map[string]string{"publish_id": remoteID}, &out); err != nil {
			return nil, err
		}
		if out.ErrCode != 0 {
			return nil, p.apiError(out.ErrCode)
		}
		if out.PublishStatus == nil {
			return nil, &biz.WechatRemoteError{Code: "invalid_response"}
		}
		result := &biz.WechatPublishResult{}
		switch *out.PublishStatus {
		case 0:
			result.Succeeded = true
			if len(out.ArticleDetail.Items) > 0 {
				result.ArticleURL = out.ArticleDetail.Items[0].ArticleURL
			}
		case 1:
			result.Pending = true
		default:
			result.ErrorCode = "publish_status_" + strconv.Itoa(*out.PublishStatus)
		}
		return result, nil
	}
	if mode != "mass" {
		return nil, errors.New("official account status mode must be draft, publish, or mass")
	}
	var out struct {
		wechatEnvelope
		MessageStatus string `json:"msg_status"`
	}
	if err := p.readAPIJSON(ctx, "/cgi-bin/message/mass/get", map[string]string{"msg_id": remoteID}, &out); err != nil {
		return nil, err
	}
	if out.ErrCode != 0 {
		return nil, p.apiError(out.ErrCode)
	}
	if out.MessageStatus == "" {
		return nil, &biz.WechatRemoteError{Code: "invalid_response"}
	}
	result := &biz.WechatPublishResult{}
	switch out.MessageStatus {
	case "SEND_SUCCESS":
		result.Succeeded = true
	case "SENDING":
		result.Pending = true
	case "SEND_FAIL", "DELETE":
		result.ErrorCode = strings.ToLower(out.MessageStatus)
	default:
		result.ErrorCode = "unknown_mass_status"
	}
	return result, nil
}

func (p *officialAccountPublisher) prepareContent(ctx context.Context, content string) (string, error) {
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(content), root)
	if err != nil {
		return "", localRemoteError("html_invalid")
	}
	for _, node := range nodes {
		root.AppendChild(node)
	}
	if err := p.cleanNode(ctx, root); err != nil {
		return "", err
	}
	var rendered strings.Builder
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		if err := html.Render(&rendered, node); err != nil {
			return "", localRemoteError("html_invalid")
		}
	}
	return rendered.String(), nil
}

func (p *officialAccountPublisher) cleanNode(ctx context.Context, parent *html.Node) error {
	for node := parent.FirstChild; node != nil; {
		next := node.NextSibling
		if node.Type == html.ElementNode && isRemovedElement(node.Data) {
			parent.RemoveChild(node)
			node = next
			continue
		}
		if node.Type == html.ElementNode {
			attrs := node.Attr[:0]
			var imageSource string
			for _, attr := range node.Attr {
				name := strings.ToLower(attr.Key)
				if !safeHTMLAttribute(node.Data, name, attr.Val) {
					continue
				}
				if node.Data == "img" && name == "src" {
					imageSource = attr.Val
					continue
				}
				attrs = append(attrs, attr)
			}
			node.Attr = attrs
			if node.Data == "img" {
				if imageSource == "" {
					return localRemoteError("image_invalid")
				}
				uploaded, err := p.uploadContentImage(ctx, imageSource)
				if err != nil {
					return err
				}
				node.Attr = append(node.Attr, html.Attribute{Key: "src", Val: uploaded})
			}
		}
		if err := p.cleanNode(ctx, node); err != nil {
			return err
		}
		node = next
	}
	return nil
}

func isRemovedElement(name string) bool {
	switch strings.ToLower(name) {
	case "script", "iframe", "object", "embed", "form", "input", "button", "link", "meta", "base", "svg", "math", "canvas", "audio", "video", "source":
		return true
	default:
		return false
	}
}

func safeHTMLAttribute(element, name, value string) bool {
	if name == "style" || name == "srcset" || strings.HasPrefix(name, "on") || strings.Contains(name, ":") {
		return false
	}
	switch name {
	case "alt", "title", "width", "height", "align", "colspan", "rowspan":
		return true
	case "href":
		if element != "a" {
			return false
		}
		u, err := url.Parse(strings.TrimSpace(value))
		return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
	case "src":
		return element == "img"
	default:
		return false
	}
}

func (p *officialAccountPublisher) uploadContentImage(ctx context.Context, source string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(source))
	if err != nil || u.User != nil {
		return "", localRemoteError("image_invalid")
	}
	if !u.IsAbs() {
		base, baseErr := url.Parse(p.siteURL + "/")
		if baseErr != nil || base.Host == "" {
			return "", localRemoteError("image_invalid")
		}
		u = base.ResolveReference(u)
	}
	if u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", localRemoteError("image_invalid")
	}
	if !p.imageURLAllowed(u) {
		return "", localRemoteError("image_host_not_allowed")
	}
	if err := p.validatePublicHost(ctx, u.Hostname()); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", localRemoteError("image_invalid")
	}
	imageClient := *p.imageClient
	imageClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !p.imageURLAllowed(req.URL) {
			return localRemoteError("image_host_not_allowed")
		}
		return p.validatePublicHost(req.Context(), req.URL.Hostname())
	}
	res, err := imageClient.Do(req)
	if err != nil {
		return "", localRemoteError("image_download_failed")
	}
	defer func() { _ = res.Body.Close() }()
	if res.Request == nil || res.Request.URL == nil || !p.imageURLAllowed(res.Request.URL) {
		return "", localRemoteError("image_host_not_allowed")
	}
	if err := p.validatePublicHost(ctx, res.Request.URL.Hostname()); err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", localRemoteError("image_download_failed")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxArticleImageSize+1))
	if err != nil || len(data) == 0 || len(data) > maxArticleImageSize {
		return "", localRemoteError("image_too_large")
	}
	kind := http.DetectContentType(data)
	if kind != "image/jpeg" && kind != "image/png" {
		return "", localRemoteError("image_type_unsupported")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	ext := ".jpg"
	if kind == "image/png" {
		ext = ".png"
	}
	part, err := writer.CreateFormFile("media", "article"+ext)
	if err != nil {
		return "", localRemoteError("image_upload_prepare_failed")
	}
	if _, err := part.Write(data); err != nil {
		return "", localRemoteError("image_upload_prepare_failed")
	}
	if err := writer.Close(); err != nil {
		return "", localRemoteError("image_upload_prepare_failed")
	}
	token, err := p.token(ctx)
	if err != nil {
		return "", err
	}
	endpoint := p.baseURL + "/cgi-bin/media/uploadimg?access_token=" + url.QueryEscape(token)
	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", localRemoteError("image_upload_prepare_failed")
	}
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	var out struct {
		wechatEnvelope
		URL string `json:"url"`
	}
	// A failed upload does not create a draft. It is safe for the task runner to
	// retry the complete conversion, so do not classify this as ambiguous.
	if err := p.doJSON(uploadReq, &out, false); err != nil {
		return "", err
	}
	if out.ErrCode != 0 {
		return "", p.apiError(out.ErrCode)
	}
	if out.URL == "" {
		return "", localRemoteError("image_upload_failed")
	}
	return out.URL, nil
}

func (p *officialAccountPublisher) imageURLAllowed(u *url.URL) bool {
	host := strings.ToLower(u.Host)
	for _, allowed := range p.allowedImages {
		if u.Scheme == allowed.scheme && host == allowed.host {
			return true
		}
	}
	return false
}

func (p *officialAccountPublisher) validatePublicHost(ctx context.Context, host string) error {
	ips, err := p.lookupIP(ctx, host)
	if err != nil || len(ips) == 0 {
		return localRemoteError("image_host_resolution_failed")
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return localRemoteError("image_host_not_public")
		}
	}
	return nil
}

func (p *officialAccountPublisher) writeJSON(ctx context.Context, path string, payload, out any) error {
	token, err := p.token(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := p.baseURL + path + "?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return unknownWriteError()
	}
	req.Header.Set("Content-Type", "application/json")
	return p.doJSON(req, out, true)
}

func (p *officialAccountPublisher) readAPIJSON(ctx context.Context, path string, payload, out any) error {
	token, err := p.token(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := p.baseURL + path + "?access_token=" + url.QueryEscape(token)
	return p.readJSON(ctx, http.MethodPost, endpoint, json.RawMessage(body), out)
}

func (p *officialAccountPublisher) readJSON(ctx context.Context, method, endpoint string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if strings.HasPrefix(endpoint, "/") {
		endpoint = p.baseURL + endpoint
	}
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		last = p.doJSON(req, out, false)
		if last == nil || ctx.Err() != nil || !retryableReadError(last) {
			return last
		}
	}
	return last
}

func (p *officialAccountPublisher) doJSON(req *http.Request, out any, write bool) error {
	res, err := p.httpClient.Do(req)
	if err != nil {
		if write {
			return unknownWriteError()
		}
		return &biz.WechatRemoteError{Code: "transport_error"}
	}
	defer func() { _ = res.Body.Close() }()
	response, err := io.ReadAll(io.LimitReader(res.Body, maxWechatResponseSize+1))
	if err != nil || len(response) == 0 || len(response) > maxWechatResponseSize {
		if write {
			return unknownWriteError()
		}
		return &biz.WechatRemoteError{Code: "invalid_response"}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if write && res.StatusCode >= 500 {
			return unknownWriteError()
		}
		return &biz.WechatRemoteError{Code: "http_status_" + strconv.Itoa(res.StatusCode)}
	}
	if err := json.Unmarshal(response, out); err != nil {
		if write {
			return unknownWriteError()
		}
		return &biz.WechatRemoteError{Code: "invalid_response"}
	}
	return nil
}

func retryableReadError(err error) bool {
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) {
		return false
	}
	if remote.Code == "transport_error" || remote.Code == "invalid_response" {
		return true
	}
	if strings.HasPrefix(remote.Code, "http_status_5") {
		return true
	}
	return false
}

func remoteError(code int64, ambiguous bool) error {
	return &biz.WechatRemoteError{Code: strconv.FormatInt(code, 10), Ambiguous: ambiguous}
}

func (p *officialAccountPublisher) apiError(code int64) error {
	p.invalidateToken(code)
	return remoteError(code, false)
}

func (p *officialAccountPublisher) invalidateToken(code int64) {
	if code != 40001 && code != 40014 && code != 42001 {
		return
	}
	p.tokenMu.Lock()
	p.accessToken = ""
	p.tokenExpires = time.Time{}
	p.tokenMu.Unlock()
}

func unknownWriteError() error {
	return &biz.WechatRemoteError{Code: "unknown", Ambiguous: true}
}

func localRemoteError(code string) error {
	return &biz.WechatRemoteError{Code: code}
}
