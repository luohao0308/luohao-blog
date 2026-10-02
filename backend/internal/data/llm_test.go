package data

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

func chatTestBootstrap(url string) *conf.Bootstrap {
	return &conf.Bootstrap{Llm: &conf.Llm{BaseUrl: url, ApiKey: "test-key", Model: "test-model"}}
}

func TestNewChatLLMDisabled(t *testing.T) {
	for name, b := range map[string]*conf.Bootstrap{
		"absent":       {},
		"nil llm":      {Llm: nil},
		"no base url":  {Llm: &conf.Llm{Model: "m"}},
		"no model":     {Llm: &conf.Llm{BaseUrl: "http://x"}},
	} {
		if got := NewChatLLM(b); got != nil {
			t.Fatalf("%s: client = %v, want nil (chat disabled)", name, got)
		}
	}
}

func TestChatLLMGenerate(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody chatCompletionsRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "生成的回答"}},
			},
		})
	}))
	defer srv.Close()

	client := NewChatLLM(chatTestBootstrap(srv.URL))
	answer, err := client.Generate(context.Background(), "系统指令", "用户问题")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("auth = %q, want bearer token", gotAuth)
	}
	if gotBody.Model != "test-model" || len(gotBody.Messages) != 2 ||
		gotBody.Messages[0].Role != "system" || gotBody.Messages[0].Content != "系统指令" ||
		gotBody.Messages[1].Content != "用户问题" {
		t.Fatalf("request body = %+v", gotBody)
	}
	if answer != "生成的回答" {
		t.Fatalf("answer = %q", answer)
	}
}

func TestChatLLMErrors(t *testing.T) {
	t.Run("http error surfaces", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		if _, err := NewChatLLM(chatTestBootstrap(srv.URL)).Generate(context.Background(), "s", "u"); err == nil {
			t.Fatal("Generate() succeeded, want HTTP error")
		}
	})
	t.Run("empty choices rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
		}))
		defer srv.Close()
		if _, err := NewChatLLM(chatTestBootstrap(srv.URL)).Generate(context.Background(), "s", "u"); err == nil {
			t.Fatal("Generate() succeeded, want empty-response error")
		}
	})
}
