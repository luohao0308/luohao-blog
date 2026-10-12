package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

func TestNewOfficialAccountPublisherConfiguration(t *testing.T) {
	t.Parallel()
	for _, bootstrap := range []*conf.Bootstrap{nil, {}, {OfficialAccount: &conf.OfficialAccount{}}} {
		publisher, err := NewOfficialAccountPublisher(bootstrap)
		if err != nil || publisher != nil {
			t.Fatalf("disabled config returned publisher = %#v, err = %v", publisher, err)
		}
	}
	missing := &conf.Bootstrap{OfficialAccount: &conf.OfficialAccount{Enabled: true, AppId: "app", AppSecret: "secret", DefaultThumbMediaId: "cover"}}
	if _, err := NewOfficialAccountPublisher(missing); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("missing site_url error = %v", err)
	}
	configured := &conf.Bootstrap{OfficialAccount: &conf.OfficialAccount{
		Enabled: true, AppId: "app", AppSecret: "secret", SiteUrl: "https://blog.example.com", DefaultThumbMediaId: "cover",
	}}
	publisher, err := NewOfficialAccountPublisher(configured)
	if err != nil {
		t.Fatal(err)
	}
	client, ok := publisher.(*officialAccountPublisher)
	if !ok || client.baseURL != wechatAPIBaseURL || client.mode != "draft" {
		t.Fatalf("publisher = %#v", publisher)
	}
}

func testOfficialAccountPublisher(server *httptest.Server) *officialAccountPublisher {
	return &officialAccountPublisher{
		appID:               "app-id",
		appSecret:           "app-secret",
		defaultThumbMediaID: "cover-id",
		mode:                "publish",
		baseURL:             server.URL,
		httpClient:          server.Client(),
		imageClient:         server.Client(),
		now:                 time.Now,
		lookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("8.8.8.8")}, nil
		},
	}
}

func TestOfficialAccountCreateDraftReusesTokenAndCleansHTML(t *testing.T) {
	t.Parallel()
	var tokenCalls atomic.Int32
	var draftCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			tokenCalls.Add(1)
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request["appid"] != "app-id" || request["secret"] != "app-secret" || request["force_refresh"] != false {
				t.Fatalf("unexpected token request: %#v", request)
			}
			_, _ = io.WriteString(w, `{"access_token":"token-value","expires_in":7200}`)
		case "/cgi-bin/draft/add":
			draftCalls.Add(1)
			if got := r.URL.Query().Get("access_token"); got != "token-value" {
				t.Fatalf("access token = %q", got)
			}
			var request struct {
				Articles []struct {
					ArticleType string `json:"article_type"`
					Content     string `json:"content"`
					Thumb       string `json:"thumb_media_id"`
				} `json:"articles"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if len(request.Articles) != 1 || request.Articles[0].ArticleType != "news" || request.Articles[0].Thumb != "cover-id" {
				t.Fatalf("unexpected draft request: %#v", request)
			}
			content := request.Articles[0].Content
			if strings.Contains(content, "style=") || strings.Contains(content, "onclick=") || strings.Contains(content, "<script") {
				t.Fatalf("unsafe or incompatible HTML was retained: %s", content)
			}
			_, _ = io.WriteString(w, `{"media_id":"draft-id"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	article := biz.WechatArticle{Title: "文章", ContentHTML: `<p style="color:red" onclick="bad()">内容</p><script>alert(1)</script>`}
	for range 2 {
		mediaID, err := publisher.CreateDraft(context.Background(), article)
		if err != nil {
			t.Fatal(err)
		}
		if mediaID != "draft-id" {
			t.Fatalf("media id = %q", mediaID)
		}
	}
	if tokenCalls.Load() != 1 || draftCalls.Load() != 2 {
		t.Fatalf("token calls = %d, draft calls = %d", tokenCalls.Load(), draftCalls.Load())
	}
}

func TestOfficialAccountCreateDraftReturnsTypedBusinessError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
			return
		}
		_, _ = io.WriteString(w, `{"errcode":40001,"errmsg":"secret must not escape"}`)
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	_, err := publisher.CreateDraft(context.Background(), biz.WechatArticle{Title: "文章", ContentHTML: "<p>内容</p>"})
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) || remote.Code != "40001" || remote.Ambiguous {
		t.Fatalf("error = %#v", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("remote details leaked through error: %v", err)
	}
}

func TestOfficialAccountCreateDraftReturnsTypedLocalErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		article biz.WechatArticle
		code    string
	}{
		{name: "title required", article: biz.WechatArticle{}, code: "title_required"},
		{name: "title too long", article: biz.WechatArticle{Title: strings.Repeat("文", 33)}, code: "title_too_long"},
		{name: "content too large", article: biz.WechatArticle{Title: "文章", ContentHTML: strings.Repeat("x", maxArticleContentSize)}, code: "content_too_large"},
		{name: "cover required", article: biz.WechatArticle{Title: "文章", ContentHTML: "<p>内容</p>", ThumbMediaID: " "}, code: "cover_required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := &officialAccountPublisher{defaultThumbMediaID: "cover"}
			if test.code == "cover_required" {
				candidate.defaultThumbMediaID = ""
			}
			_, err := candidate.CreateDraft(context.Background(), test.article)
			var remote *biz.WechatRemoteError
			if !errors.As(err, &remote) || remote.Code != test.code || remote.Ambiguous {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestOfficialAccountWriteTransportFailureIsAmbiguousAndNotRetried(t *testing.T) {
	t.Parallel()
	var writes atomic.Int32
	publisher := &officialAccountPublisher{
		appID: "app", appSecret: "secret", defaultThumbMediaID: "cover", baseURL: "https://api.invalid",
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/cgi-bin/stable_token" {
				return jsonResponse(200, `{"access_token":"token","expires_in":7200}`), nil
			}
			writes.Add(1)
			return nil, context.DeadlineExceeded
		})},
		now: time.Now,
	}
	_, err := publisher.CreateDraft(context.Background(), biz.WechatArticle{Title: "文章", ContentHTML: "<p>内容</p>"})
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) || remote.Code != "unknown" || !remote.Ambiguous {
		t.Fatalf("error = %#v", err)
	}
	if writes.Load() != 1 {
		t.Fatalf("write attempts = %d", writes.Load())
	}
}

func TestOfficialAccountWriteHTTPClientErrorIsNotAmbiguous(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
			return
		}
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	_, err := publisher.CreateDraft(context.Background(), biz.WechatArticle{Title: "文章", ContentHTML: "<p>内容</p>"})
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) || remote.Code != "http_status_400" || remote.Ambiguous {
		t.Fatalf("error = %#v", err)
	}
}

func TestOfficialAccountSubmitMassUsesStableClientMessageID(t *testing.T) {
	t.Parallel()
	var firstID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
			return
		}
		var request struct {
			ClientMessageID string `json:"clientmsgid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.ClientMessageID) != 32 {
			t.Fatalf("clientmsgid length = %d", len(request.ClientMessageID))
		}
		if firstID == "" {
			firstID = request.ClientMessageID
		} else if request.ClientMessageID != firstID {
			t.Fatalf("clientmsgid changed: %q != %q", request.ClientMessageID, firstID)
		}
		_, _ = io.WriteString(w, `{"errcode":45065,"errmsg":"duplicate","msg_id":12345}`)
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	for range 2 {
		id, err := publisher.Submit(context.Background(), "draft-id", "mass", strings.Repeat("x", 40))
		if err != nil || id != "12345" {
			t.Fatalf("id = %q, err = %v", id, err)
		}
	}
}

func TestOfficialAccountStatusMappingsAndReadRetry(t *testing.T) {
	t.Parallel()
	var publishCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
		case "/cgi-bin/freepublish/get":
			if publishCalls.Add(1) == 1 {
				http.Error(w, "temporary", http.StatusBadGateway)
				return
			}
			_, _ = io.WriteString(w, `{"publish_status":0,"article_detail":{"item":[{"article_url":"https://mp.weixin.qq.com/s/ok"}]}}`)
		case "/cgi-bin/message/mass/get":
			_, _ = io.WriteString(w, `{"msg_status":"SENDING"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	published, err := publisher.Status(context.Background(), "publish-id", "publish")
	if err != nil || !published.Succeeded || published.ArticleURL == "" || publishCalls.Load() != 2 {
		t.Fatalf("publish result = %#v, calls = %d, err = %v", published, publishCalls.Load(), err)
	}
	mass, err := publisher.Status(context.Background(), "123", "mass")
	if err != nil || !mass.Pending || mass.Succeeded {
		t.Fatalf("mass result = %#v, err = %v", mass, err)
	}
}

func TestOfficialAccountStatusRefreshesTokenOnNextCallAfterExpiry(t *testing.T) {
	t.Parallel()
	var tokenCalls atomic.Int32
	var statusCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			call := tokenCalls.Add(1)
			_, _ = io.WriteString(w, `{"access_token":"token-`+string(rune('0'+call))+`","expires_in":7200}`)
		case "/cgi-bin/freepublish/get":
			call := statusCalls.Add(1)
			if call == 1 {
				if got := r.URL.Query().Get("access_token"); got != "token-1" {
					t.Fatalf("first status token = %q", got)
				}
				_, _ = io.WriteString(w, `{"errcode":42001,"errmsg":"expired token with sensitive detail"}`)
				return
			}
			if got := r.URL.Query().Get("access_token"); got != "token-2" {
				t.Fatalf("second status token = %q", got)
			}
			_, _ = io.WriteString(w, `{"publish_status":0,"article_detail":{"item":[{"article_url":"https://mp.weixin.qq.com/s/ok"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	_, err := publisher.Status(context.Background(), "publish-id", "publish")
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) || remote.Code != "42001" || remote.Ambiguous {
		t.Fatalf("first status error = %#v", err)
	}
	if strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("remote detail leaked: %v", err)
	}
	result, err := publisher.Status(context.Background(), "publish-id", "publish")
	if err != nil || !result.Succeeded {
		t.Fatalf("second status result = %#v, err = %v", result, err)
	}
	if tokenCalls.Load() != 2 || statusCalls.Load() != 2 {
		t.Fatalf("token calls = %d, status calls = %d", tokenCalls.Load(), statusCalls.Load())
	}
}

func TestOfficialAccountWriteDoesNotReplayExpiredToken(t *testing.T) {
	t.Parallel()
	var tokenCalls atomic.Int32
	var draftCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			tokenCalls.Add(1)
			_, _ = io.WriteString(w, `{"access_token":"expired-token","expires_in":7200}`)
		case "/cgi-bin/draft/add":
			draftCalls.Add(1)
			_, _ = io.WriteString(w, `{"errcode":42001,"errmsg":"expired token"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	_, err := publisher.CreateDraft(context.Background(), biz.WechatArticle{Title: "文章", ContentHTML: "<p>内容</p>"})
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) || remote.Code != "42001" || remote.Ambiguous {
		t.Fatalf("error = %#v", err)
	}
	if tokenCalls.Load() != 1 || draftCalls.Load() != 1 {
		t.Fatalf("token calls = %d, draft calls = %d", tokenCalls.Load(), draftCalls.Load())
	}
	publisher.tokenMu.Lock()
	cachedToken := publisher.accessToken
	publisher.tokenMu.Unlock()
	if cachedToken != "" {
		t.Fatalf("expired token remained cached")
	}
}

func TestOfficialAccountInvalidatesAllKnownExpiredTokenCodes(t *testing.T) {
	t.Parallel()
	for _, code := range []int64{40001, 40014, 42001} {
		publisher := &officialAccountPublisher{accessToken: "cached", tokenExpires: time.Now().Add(time.Hour)}
		publisher.invalidateToken(code)
		if publisher.accessToken != "" || !publisher.tokenExpires.IsZero() {
			t.Fatalf("code %d did not invalidate token", code)
		}
	}
	publisher := &officialAccountPublisher{accessToken: "cached", tokenExpires: time.Now().Add(time.Hour)}
	publisher.invalidateToken(48001)
	if publisher.accessToken != "cached" {
		t.Fatalf("non-token error invalidated token")
	}
}

func TestOfficialAccountRejectsUnsafeImagesBeforeDownload(t *testing.T) {
	t.Parallel()
	var downloads atomic.Int32
	publisher := &officialAccountPublisher{
		defaultThumbMediaID: "cover",
		allowedImages:       []allowedImageSource{{scheme: "https", host: "images.example.com"}},
		lookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("169.254.169.254")}, nil
		},
		imageClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			downloads.Add(1)
			return jsonResponse(200, "image"), nil
		})},
	}
	_, err := publisher.prepareContent(context.Background(), `<img src="https://images.example.com/a.png">`)
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) || remote.Code != "image_host_not_public" || remote.Ambiguous {
		t.Fatalf("error = %#v", err)
	}
	if downloads.Load() != 0 {
		t.Fatalf("unsafe image was downloaded")
	}
	_, err = publisher.prepareContent(context.Background(), `<img src="https://evil.example/a.png">`)
	if !errors.As(err, &remote) || remote.Code != "image_host_not_allowed" || remote.Ambiguous {
		t.Fatalf("error = %#v", err)
	}
}

func TestOfficialAccountUploadsAllowedContentImage(t *testing.T) {
	t.Parallel()
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
		case "/cgi-bin/media/uploadimg":
			if err := r.ParseMultipartForm(maxArticleImageSize + 1024); err != nil {
				t.Fatal(err)
			}
			file, _, err := r.FormFile("media")
			if err != nil {
				t.Fatal(err)
			}
			_ = file.Close()
			_, _ = io.WriteString(w, `{"url":"https://mmbiz.qpic.cn/converted.png"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	publisher.siteURL = "https://blog.example.com"
	publisher.allowedImages = []allowedImageSource{{scheme: "https", host: "blog.example.com"}}
	publisher.imageClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://blog.example.com/images/a.png" {
			t.Fatalf("relative image resolved to %q", r.URL.String())
		}
		response := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(png)), Header: make(http.Header), Request: r}
		response.Header.Set("Content-Type", "image/png")
		return response, nil
	})}
	content, err := publisher.prepareContent(context.Background(), `<p><img style="width:1px" src="/images/a.png"></p>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `src="https://mmbiz.qpic.cn/converted.png"`) || strings.Contains(content, "style=") {
		t.Fatalf("content = %s", content)
	}
}

func TestOfficialAccountImageNetworkErrorDoesNotLeakURL(t *testing.T) {
	t.Parallel()
	publisher := &officialAccountPublisher{
		siteURL:       "https://blog.example.com",
		allowedImages: []allowedImageSource{{scheme: "https", host: "blog.example.com"}},
		lookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("8.8.8.8")}, nil
		},
		imageClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("GET https://blog.example.com/private.png?access_token=secret-token failed")
		})},
	}
	_, err := publisher.prepareContent(context.Background(), `<img src="/private.png?access_token=secret-token">`)
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) || remote.Code != "image_download_failed" || remote.Ambiguous {
		t.Fatalf("error = %#v", err)
	}
	if strings.Contains(err.Error(), "blog.example.com") || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("network detail leaked: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
