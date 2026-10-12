package data

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

func validCoverJPEG(t *testing.T) []byte {
	t.Helper()
	var encoded bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func TestOfficialAccountCoverPublisherClonesConfiguration(t *testing.T) {
	t.Parallel()
	original := &conf.Bootstrap{OfficialAccount: &conf.OfficialAccount{
		Enabled: false, AppId: "app", AppSecret: "secret", Mode: "draft",
	}}
	publisher, err := officialAccountCoverPublisher(original, "https://blog.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if publisher.siteURL != "https://blog.example.com" || publisher.defaultThumbMediaID != setupCoverPlaceholder {
		t.Fatalf("publisher setup = %#v", publisher)
	}
	if original.GetOfficialAccount().GetEnabled() || original.GetOfficialAccount().GetSiteUrl() != "" || original.GetOfficialAccount().GetDefaultThumbMediaId() != "" {
		t.Fatalf("original configuration was modified: %#v", original.GetOfficialAccount())
	}
}

func TestUploadPermanentCoverSuccess(t *testing.T) {
	t.Parallel()
	var tokenCalls atomic.Int32
	var uploadCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			tokenCalls.Add(1)
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
		case "/cgi-bin/material/add_material":
			uploadCalls.Add(1)
			if r.URL.Query().Get("access_token") != "token" || r.URL.Query().Get("type") != "thumb" {
				t.Fatalf("unexpected upload query")
			}
			file, header, err := r.FormFile("media")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = file.Close() }()
			if header.Filename != "cover.jpg" {
				t.Fatalf("filename = %q", header.Filename)
			}
			uploaded, err := io.ReadAll(file)
			if err != nil || len(uploaded) == 0 {
				t.Fatalf("uploaded cover invalid: bytes=%d err=%v", len(uploaded), err)
			}
			_, _ = io.WriteString(w, `{"media_id":"permanent-cover-id","url":"https://mmbiz.qpic.cn/cover.jpg"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	mediaID, err := publisher.uploadPermanentCover(context.Background(), validCoverJPEG(t))
	if err != nil || mediaID != "permanent-cover-id" {
		t.Fatalf("media id = %q, err = %v", mediaID, err)
	}
	if tokenCalls.Load() != 1 || uploadCalls.Load() != 1 {
		t.Fatalf("token calls = %d, upload calls = %d", tokenCalls.Load(), uploadCalls.Load())
	}
}

func TestUploadPermanentCoverBusinessErrorDoesNotLeakSecret(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
			return
		}
		_, _ = io.WriteString(w, `{"errcode":48001,"errmsg":"api unauthorized appsecret=do-not-leak"}`)
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	_, err := publisher.uploadPermanentCover(context.Background(), validCoverJPEG(t))
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) || remote.Code != "48001" || remote.Ambiguous {
		t.Fatalf("error = %#v", err)
	}
	if strings.Contains(err.Error(), "do-not-leak") || strings.Contains(err.Error(), server.URL) || strings.Contains(err.Error(), "access_token") {
		t.Fatalf("sensitive detail leaked: %v", err)
	}
}

func TestUploadPermanentCoverTimeoutIsAmbiguous(t *testing.T) {
	t.Parallel()
	var uploadCalls atomic.Int32
	publisher := &officialAccountPublisher{
		appID: "app", appSecret: "secret", baseURL: "https://api.invalid", now: time.Now,
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/cgi-bin/stable_token" {
				return jsonResponse(http.StatusOK, `{"access_token":"token","expires_in":7200}`), nil
			}
			uploadCalls.Add(1)
			return nil, context.DeadlineExceeded
		})},
	}
	_, err := publisher.uploadPermanentCover(context.Background(), validCoverJPEG(t))
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) || remote.Code != "unknown" || !remote.Ambiguous {
		t.Fatalf("error = %#v", err)
	}
	if uploadCalls.Load() != 1 {
		t.Fatalf("upload calls = %d", uploadCalls.Load())
	}
}

func TestUploadPermanentCoverMissingMediaIDIsAmbiguous(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
			return
		}
		_, _ = io.WriteString(w, `{"url":"https://mmbiz.qpic.cn/cover.jpg"}`)
	}))
	defer server.Close()
	publisher := testOfficialAccountPublisher(server)
	_, err := publisher.uploadPermanentCover(context.Background(), validCoverJPEG(t))
	var remote *biz.WechatRemoteError
	if !errors.As(err, &remote) || remote.Code != "unknown" || !remote.Ambiguous {
		t.Fatalf("error = %#v", err)
	}
}

func TestUploadPermanentCoverRejectsInvalidAndOversizeInput(t *testing.T) {
	t.Parallel()
	publisher := &officialAccountPublisher{}
	tests := []struct {
		name  string
		image []byte
		code  string
	}{
		{name: "invalid", image: []byte("not jpeg"), code: "cover_invalid"},
		{name: "truncated jpeg", image: []byte{0xff, 0xd8, 0xff}, code: "cover_invalid"},
		{name: "oversize", image: bytes.Repeat([]byte{0xff}, maxOfficialAccountCoverSize+1), code: "cover_too_large"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := publisher.uploadPermanentCover(context.Background(), test.image)
			var remote *biz.WechatRemoteError
			if !errors.As(err, &remote) || remote.Code != test.code || remote.Ambiguous {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}
