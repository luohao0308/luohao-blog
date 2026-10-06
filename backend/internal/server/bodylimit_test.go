package server

import (
	"net/http"
	"strings"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
)

// decodeBody runs the wired request decoder against a synthetic request.
func decodeBody(t *testing.T, method, path, body string, v any) error {
	t.Helper()
	r, err := http.NewRequest(method, path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	r.Header.Set("Content-Type", "application/json")
	return limitedBodyDecoder(r, v)
}

// A body past the default cap must fail the decode instead of being buffered
// in full: this is the memory bound for every non-avatar route.
func TestLimitedBodyDecoderRejectsOversizedDefaultRoute(t *testing.T) {
	oversized := `{"email":"a@b.c","password":"` + strings.Repeat("x", 1<<20) + `"}`
	err := decodeBody(t, http.MethodPost, "/v1/auth/login", oversized, &v1.LoginRequest{})
	if err == nil || !strings.Contains(err.Error(), "request body too large") {
		t.Fatalf("expected body-too-large error, got %v", err)
	}
}

// The avatar route is the one endpoint whose payload legitimately exceeds the
// default cap (base64 of the 2 MiB image); 3 MiB must decode there.
func TestLimitedBodyDecoderAllowsAvatarUpload(t *testing.T) {
	avatarSized := `{"unknown_field":"` + strings.Repeat("a", 3<<20) + `"}`
	if err := decodeBody(t, http.MethodPut, "/v1/user/avatar", avatarSized, &v1.UploadAvatarRequest{}); err != nil {
		t.Fatalf("3MiB avatar payload should decode within the 4MiB cap: %v", err)
	}
}

// The cap must not change decoding semantics for normal payloads.
func TestLimitedBodyDecoderDecodesSmallBodies(t *testing.T) {
	req := &v1.LoginRequest{}
	err := decodeBody(t, http.MethodPost, "/v1/auth/login", `{"email":"a@b.c","password":"secret"}`, req)
	if err != nil {
		t.Fatalf("small body should decode: %v", err)
	}
	if req.GetEmail() != "a@b.c" || req.GetPassword() != "secret" {
		t.Fatalf("unexpected decode result: %+v", req)
	}
}
