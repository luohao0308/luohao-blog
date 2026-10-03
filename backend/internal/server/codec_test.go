package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/encoding"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// 无标签文章必须序列化出 "tags":[]：这是 2026-10 review P0（前端对缺失
// tags 取下标导致全站 SSR 500）的根治验收点。
func TestJSONCodecMarshalEmitsUnpopulated(t *testing.T) {
	data, err := hybridJSONCodec.Marshal(&v1.Article{
		Slug:   "hello-world",
		Title:  "Hello",
		Status: v1.ArticleStatus_ARTICLE_STATUS_DRAFT,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decoded: %v", err)
	}
	tags, ok := body["tags"].([]any)
	if !ok || len(tags) != 0 {
		t.Fatalf("tags should be an empty array, got %v", body["tags"])
	}
	// UseProtoNames：字段名保持 proto 原名（snake_case）
	for _, key := range []string{"content_md", "content_html", "view_count", "published_at", "updated_at"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("field %q should be emitted, got %s", key, data)
		}
	}
	// UseEnumNumbers：枚举保持数字，前端比较逻辑不变
	if status, ok := body["status"].(float64); !ok || int(status) != 1 {
		t.Fatalf("status should be numeric 1, got %v", body["status"])
	}
	// int64/uint64 按 protojson 语义输出字符串
	if vc, ok := body["view_count"].(string); !ok || vc != "0" {
		t.Fatalf("view_count should be string \"0\", got %v", body["view_count"])
	}
}

// 非 proto 载荷（kratos errors.Error）保持 stdlib 形状，错误体契约不变。
func TestJSONCodecMarshalKratosError(t *testing.T) {
	data, err := hybridJSONCodec.Marshal(kratoserrors.Unauthorized("AUTH_TEST", "bad credentials"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decoded: %v", err)
	}
	if body["reason"] != "AUTH_TEST" || body["message"] != "bad credentials" {
		t.Fatalf("unexpected error body: %s", data)
	}
	if code, ok := body["code"].(float64); !ok || int(code) != 401 {
		t.Fatalf("code should be numeric 401, got %v (%s)", body["code"], data)
	}
}

// 请求方向：protojson 同时接受 proto 原名与 camelCase，未知字段丢弃。
func TestJSONCodecUnmarshal(t *testing.T) {
	req := &v1.LoginRequest{}
	if err := hybridJSONCodec.Unmarshal([]byte(`{"email":"a@b.c","password":"pw"}`), req); err != nil {
		t.Fatalf("snake_case unmarshal: %v", err)
	}
	if req.GetEmail() != "a@b.c" || req.GetPassword() != "pw" {
		t.Fatalf("unexpected request: %+v", req)
	}
	if err := hybridJSONCodec.Unmarshal([]byte(`{"email":"a@b.c","unknown_field":1}`), req); err != nil {
		t.Fatalf("unknown field should be discarded: %v", err)
	}
	if err := hybridJSONCodec.Unmarshal(nil, req); err != nil {
		t.Fatalf("empty body should be a no-op: %v", err)
	}
}

func TestJSONCodecName(t *testing.T) {
	if hybridJSONCodec.Name() != "json" {
		t.Fatalf("codec must override the default json name, got %q", hybridJSONCodec.Name())
	}
	if encoding.GetCodec("json") == nil {
		t.Fatal("codec should be registered under the json name")
	}
}

func TestEmptyMarshal(t *testing.T) {
	data, err := hybridJSONCodec.Marshal(&emptypb.Empty{})
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	if strings.TrimSpace(string(data)) != "{}" {
		t.Fatalf("Empty should marshal to {}, got %s", data)
	}
}
