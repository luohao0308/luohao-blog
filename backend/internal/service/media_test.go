package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

type serviceMediaStore struct {
	data []byte
	fail error
}

func (s *serviceMediaStore) Save(_ context.Context, _ string, data []byte) (string, error) {
	if s.fail != nil {
		return "", s.fail
	}
	s.data = append([]byte(nil), data...)
	return "0123456789abcdef0123456789abcdef.png", nil
}

func TestMediaServiceDoesNotExposeStorageError(t *testing.T) {
	const sentinel = "/private/uploads/media/secret.png"
	svc := NewMediaService(biz.NewMediaUsecase(&serviceMediaStore{fail: errors.New("write " + sentinel)}))
	result, err := svc.UploadImage(context.Background(), &v1.UploadMediaImageRequest{Data: []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}})
	if result != nil || err == nil {
		t.Fatalf("UploadImage(storage failure) = (%+v, %v), want failure", result, err)
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("storage path leaked through service error: %v", err)
	}
}

func TestMediaServiceHTTPRoundTrip(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	srv := kratoshttp.NewServer()
	v1.RegisterMediaServiceHTTPServer(srv, NewMediaService(biz.NewMediaUsecase(&serviceMediaStore{})))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	payload, err := json.Marshal(&v1.UploadMediaImageRequest{Data: png})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(ts.URL+"/v1/admin/media/images", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}
	var uploaded v1.MediaImage
	if err = json.NewDecoder(resp.Body).Decode(&uploaded); err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	resp, err = http.Get(ts.URL + uploaded.GetUrl())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/png" || !bytes.Equal(got, png) {
		t.Fatalf("GET media = status %d content-type %q bytes %x", resp.StatusCode, resp.Header.Get("Content-Type"), got)
	}
}

func (s *serviceMediaStore) Load(_ context.Context, name string) ([]byte, string, error) {
	if name != "0123456789abcdef0123456789abcdef.png" {
		return nil, "", errors.New("missing")
	}
	return s.data, "image/png", nil
}

func TestMediaServiceUploadAndRead(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	store := &serviceMediaStore{data: png}
	svc := NewMediaService(biz.NewMediaUsecase(store))
	uploaded, err := svc.UploadImage(context.Background(), &v1.UploadMediaImageRequest{Data: png})
	if err != nil {
		t.Fatalf("UploadImage() error = %v", err)
	}
	if uploaded.GetUrl() != "/v1/assets/media/0123456789abcdef0123456789abcdef.png" {
		t.Fatalf("url = %q", uploaded.GetUrl())
	}
	body, err := svc.GetImage(context.Background(), &v1.GetMediaImageRequest{Name: "0123456789abcdef0123456789abcdef.png"})
	if err != nil || body.GetContentType() != "image/png" || string(body.GetData()) != string(png) {
		t.Fatalf("GetImage() = %+v, %v", body, err)
	}
}
