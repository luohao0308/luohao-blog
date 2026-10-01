package data

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// embeddingTestConf returns a complete embedding config pointing at the given
// test server.
func embeddingTestConf(url string) *conf.Embedding {
	return &conf.Embedding{
		BaseUrl:    url,
		ApiKey:     "test-key",
		Model:      "test-model",
		Dimensions: 3,
		Timeout:    durationpb.New(0),
	}
}

func TestNewEmbeddingClientDisabled(t *testing.T) {
	for name, c := range map[string]*conf.Embedding{
		"absent":                  nil,
		"no base url":             {Model: "m", Dimensions: 3},
		"no model":                {BaseUrl: "http://x"},
		"no dimensions":           {BaseUrl: "http://x", Model: "m"},
		"non-positive dimensions": {BaseUrl: "http://x", Model: "m", Dimensions: -1},
	} {
		if got := NewEmbeddingClient(c); got != nil {
			t.Fatalf("%s: client = %v, want nil (embedding disabled)", name, got)
		}
	}
}

func TestEmbeddingClientEmbed(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody embeddingsRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		// Out-of-order response on purpose: entries must be re-ordered by index.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"index": 1, "embedding": []float32{4, 5, 6}},
				{"index": 0, "embedding": []float32{1, 2, 3}},
			},
		})
	}))
	defer srv.Close()

	client := NewEmbeddingClient(embeddingTestConf(srv.URL))
	vectors, err := client.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if gotPath != "/embeddings" {
		t.Fatalf("path = %q, want /embeddings", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("auth = %q, want bearer token", gotAuth)
	}
	if gotBody.Model != "test-model" || len(gotBody.Input) != 2 || gotBody.Input[0] != "first" {
		t.Fatalf("request body = %+v", gotBody)
	}
	if len(vectors) != 2 || vectors[0][0] != 1 || vectors[1][2] != 6 {
		t.Fatalf("vectors = %v, want input-ordered", vectors)
	}
}

func TestEmbeddingClientErrors(t *testing.T) {
	t.Run("dimension mismatch", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"index": 0, "embedding": []float32{1, 2}}},
			})
		}))
		defer srv.Close()
		if _, err := NewEmbeddingClient(embeddingTestConf(srv.URL)).Embed(context.Background(), []string{"x"}); err == nil {
			t.Fatal("Embed() succeeded, want dimension mismatch error")
		}
	})
	t.Run("http error surfaces", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		if _, err := NewEmbeddingClient(embeddingTestConf(srv.URL)).Embed(context.Background(), []string{"x"}); err == nil {
			t.Fatal("Embed() succeeded, want HTTP error")
		}
	})
	t.Run("empty input skips the call", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
		}))
		defer srv.Close()
		if _, err := NewEmbeddingClient(embeddingTestConf(srv.URL)).Embed(context.Background(), nil); err != nil {
			t.Fatalf("Embed(nil) error = %v", err)
		}
		if called {
			t.Fatal("empty input hit the API")
		}
	})
}
