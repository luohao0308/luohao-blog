package biz

import (
	"context"
	"errors"
	"strings"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

// pngHead is the minimal byte sequence the PNG magic-byte sniff accepts.
var pngHead = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

// fakeAvatarStore keeps avatars in memory and records Save calls, so tests
// can assert that rejected uploads never reach the store.
type fakeAvatarStore struct {
	files map[string][]byte
	saves int
	fail  error
}

func newFakeAvatarStore() *fakeAvatarStore {
	return &fakeAvatarStore{files: map[string][]byte{}}
}

func (f *fakeAvatarStore) Save(_ context.Context, ext string, data []byte) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	f.saves++
	// Same name derivation as the real store: a v4 UUID flattened to 32 hex
	// chars, which is exactly the shape the biz URL pattern accepts.
	name := strings.ReplaceAll(uuid.NewString(), "-", "") + "." + ext
	f.files[name] = data
	return name, nil
}

func (f *fakeAvatarStore) Load(_ context.Context, name string) ([]byte, string, error) {
	data, ok := f.files[name]
	if !ok {
		return nil, "", errors.New("no such file")
	}
	ext := name[strings.LastIndex(name, ".")+1:]
	contentTypes := map[string]string{"jpg": "image/jpeg", "png": "image/png", "webp": "image/webp"}
	return data, contentTypes[ext], nil
}

func newTestAvatarUsecase() (*AvatarUsecase, *fakeAvatarStore, *User) {
	store := newFakeAvatarStore()
	users := NewUserUsecase(newFakeUserRepo())
	author, err := users.CreateAuthor(context.Background(), "author@example.com", "longenough1", "作者")
	if err != nil {
		panic(err)
	}
	return NewAvatarUsecase(store, users), store, author
}

func TestAvatarUsecaseSaveAndServe(t *testing.T) {
	uc, store, author := newTestAvatarUsecase()
	ctx := context.Background()

	u, err := uc.SaveAvatar(ctx, author.ID, pngHead)
	if err != nil {
		t.Fatalf("SaveAvatar() error = %v", err)
	}
	if !strings.HasPrefix(u.AvatarURL, assetsAvatarPrefix) {
		t.Fatalf("avatar url = %q, want %q prefix", u.AvatarURL, assetsAvatarPrefix)
	}
	name := strings.TrimPrefix(u.AvatarURL, assetsAvatarPrefix)
	if !avatarNamePattern.MatchString(name) {
		t.Fatalf("stored name = %q, want generator-shaped filename", name)
	}
	if store.saves != 1 {
		t.Fatalf("store saves = %d, want 1", store.saves)
	}

	// A second upload replaces the reference; the old file stays but the
	// account points at the fresh one. The first URL is snapshotted because
	// the fake repo hands back the same mutable record.
	firstURL := u.AvatarURL
	second, err := uc.SaveAvatar(ctx, author.ID, pngHead)
	if err != nil {
		t.Fatalf("SaveAvatar(second) error = %v", err)
	}
	if second.AvatarURL == firstURL {
		t.Fatal("two uploads share one filename; names are not random")
	}

	data, contentType, err := uc.GetAvatar(ctx, strings.TrimPrefix(second.AvatarURL, assetsAvatarPrefix))
	if err != nil {
		t.Fatalf("GetAvatar() error = %v", err)
	}
	if contentType != "image/png" || string(data) != string(pngHead) {
		t.Fatalf("GetAvatar() = (%q, %d bytes), want the stored png back", contentType, len(data))
	}
}

func TestAvatarUsecaseRejectsInvalidUploads(t *testing.T) {
	oversize := append(append([]byte{}, pngHead...), make([]byte, MaxAvatarBytes)...)

	cases := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"plain text", []byte("definitely not an image")},
		{"gif is not accepted", []byte("GIF89a\x01\x00\x01\x00")},
		{"truncated png", pngHead[:4]},
		{"oversize", oversize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, store, author := newTestAvatarUsecase()
			if _, err := uc.SaveAvatar(context.Background(), author.ID, tc.data); !kratoserrors.IsBadRequest(err) {
				t.Fatalf("SaveAvatar(%s) error = %v, want bad request", tc.name, err)
			}
			if store.saves != 0 {
				t.Fatalf("rejected upload reached the store %d time(s)", store.saves)
			}
		})
	}
}

func TestAvatarUsecaseGetAvatarGuards(t *testing.T) {
	uc, _, _ := newTestAvatarUsecase()

	cases := []struct {
		name string
		in   string
	}{
		{"path traversal", "../../../../etc/passwd"},
		{"dot segments", "..%2f..%2fsecret.png"},
		{"wrong extension", "0123456789abcdef0123456789abcdef.txt"},
		{"uppercase hex", "0123456789ABCDEF0123456789ABCDEF.png"},
		{"too short", "0123abcd.png"},
		{"no extension", "0123456789abcdef0123456789abcdef"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := uc.GetAvatar(context.Background(), tc.in); !kratoserrors.IsNotFound(err) {
				t.Fatalf("GetAvatar(%q) error = %v, want not found", tc.in, err)
			}
		})
	}
	// A well-formed name that was never stored is a not-found too.
	if _, _, err := uc.GetAvatar(context.Background(), "0123456789abcdef0123456789abcdef.png"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("GetAvatar(unknown) error = %v, want not found", err)
	}
}

func TestSniffImageExt(t *testing.T) {
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	webp := append([]byte("RIFF"), append([]byte{0x00, 0x00, 0x00, 0x00}, []byte("WEBP")...)...)

	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"jpeg", jpeg, "jpg"},
		{"png", pngHead, "png"},
		{"webp", webp, "webp"},
		{"gif", []byte("GIF89a"), ""},
		{"text", []byte("hello"), ""},
		{"empty", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext, ok := sniffImageExt(tc.data)
			if tc.want == "" {
				if ok {
					t.Fatalf("sniffImageExt(%s) = (%q, true), want rejected", tc.name, ext)
				}
				return
			}
			if !ok || ext != tc.want {
				t.Fatalf("sniffImageExt(%s) = (%q, %v), want %q", tc.name, ext, ok, tc.want)
			}
		})
	}
}
