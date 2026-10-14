package biz

import (
	"context"
	"errors"
	"strings"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type fakeMediaStore struct {
	files map[string][]byte
	saves int
	fail  error
}

func newFakeMediaStore() *fakeMediaStore { return &fakeMediaStore{files: map[string][]byte{}} }

func (f *fakeMediaStore) Save(_ context.Context, ext string, data []byte) (string, error) {
	f.saves++
	if f.fail != nil {
		return "", f.fail
	}
	name := strings.ReplaceAll(uuid.NewString(), "-", "") + "." + ext
	f.files[name] = append([]byte(nil), data...)
	return name, nil
}

func TestMediaUsecaseHidesStorageFailureDetails(t *testing.T) {
	const sentinel = "/private/uploads/media/secret-path.jpg"
	store := newFakeMediaStore()
	store.fail = errors.New("write " + sentinel + ": permission denied")
	url, err := NewMediaUsecase(store).SaveImage(context.Background(), pngHead)
	if url != "" || !kratoserrors.IsInternalServer(err) {
		t.Fatalf("SaveImage(storage failure) = (%q, %v), want empty internal error", url, err)
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("storage path leaked through API error: %v", err)
	}
	if store.saves != 1 {
		t.Fatalf("store save attempts = %d, want 1", store.saves)
	}
}

func (f *fakeMediaStore) Load(_ context.Context, name string) ([]byte, string, error) {
	data, ok := f.files[name]
	if !ok {
		return nil, "", errors.New("missing")
	}
	ext := name[strings.LastIndex(name, ".")+1:]
	return data, map[string]string{"jpg": "image/jpeg", "png": "image/png", "webp": "image/webp"}[ext], nil
}

func TestMediaUsecaseSaveAndServe(t *testing.T) {
	store := newFakeMediaStore()
	uc := NewMediaUsecase(store)
	url, err := uc.SaveImage(context.Background(), pngHead)
	if err != nil {
		t.Fatalf("SaveImage() error = %v", err)
	}
	if !ValidMediaURL(url) || !strings.HasPrefix(url, assetsMediaPrefix) {
		t.Fatalf("url = %q, want generated media URL", url)
	}
	data, contentType, err := uc.GetImage(context.Background(), strings.TrimPrefix(url, assetsMediaPrefix))
	if err != nil || contentType != "image/png" || string(data) != string(pngHead) {
		t.Fatalf("GetImage() = (%q, %d, %v), want stored png", contentType, len(data), err)
	}
}

func TestMediaUsecaseRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"text", []byte("not an image")},
		{"gif", []byte("GIF89a")},
		{"oversize", append(append([]byte{}, pngHead...), make([]byte, MaxMediaBytes)...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeMediaStore()
			if _, err := NewMediaUsecase(store).SaveImage(context.Background(), tc.data); !kratoserrors.IsBadRequest(err) {
				t.Fatalf("SaveImage() error = %v, want bad request", err)
			}
			if store.saves != 0 {
				t.Fatalf("rejected upload saved %d file(s)", store.saves)
			}
		})
	}

	uc := NewMediaUsecase(newFakeMediaStore())
	for _, name := range []string{"../../secret", "0123.png", "0123456789abcdef0123456789abcdef.svg"} {
		if _, _, err := uc.GetImage(context.Background(), name); !kratoserrors.IsNotFound(err) {
			t.Fatalf("GetImage(%q) error = %v, want not found", name, err)
		}
	}
}

func TestValidMediaURL(t *testing.T) {
	valid := "/v1/assets/media/0123456789abcdef0123456789abcdef.webp"
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"", true},
		{valid, true},
		{"https://example.com/cover.jpg", false},
		{"javascript:alert(1)", false},
		{"/v1/assets/media/../../secret.png", false},
		{"/v1/assets/avatars/0123456789abcdef0123456789abcdef.png", false},
	} {
		if got := ValidMediaURL(tc.url); got != tc.want {
			t.Errorf("ValidMediaURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}
