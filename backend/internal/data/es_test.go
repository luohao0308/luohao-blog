package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

type searchTestEmbedder struct{ fail bool }

func (e searchTestEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	if e.fail {
		return nil, fmt.Errorf("provider offline")
	}
	return [][]float32{{0.1, 0.2}}, nil
}

func TestEsSearchHTTPBoundary(t *testing.T) {
	for _, tc := range []struct {
		name             string
		ready, fail, knn bool
	}{
		{"BM25 without mapping", false, false, false},
		{"hybrid", true, false, true},
		{"embedding failure fallback", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/articles/_search" {
					t.Errorf("path=%s", r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				w.Header().Set("X-Elastic-Product", "Elasticsearch")
				_, _ = w.Write([]byte(`{"hits":{"hits":[{"_source":{"slug":"second"}},{"_source":{"slug":"first"}}]}}`))
			}))
			defer srv.Close()
			client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{srv.URL}})
			if err != nil {
				t.Fatal(err)
			}
			idx := &esIndexer{client: client, index: "articles", embedder: searchTestEmbedder{fail: tc.fail}, vectorReady: tc.ready}
			slugs, err := idx.Search(context.Background(), "中文 query", 5, 10)
			if err != nil || !reflect.DeepEqual(slugs, []string{"second", "first"}) {
				t.Fatalf("slugs=%v err=%v", slugs, err)
			}
			match := body["query"].(map[string]any)["multi_match"].(map[string]any)
			if match["query"] != "中文 query" || body["from"] != float64(10) || body["size"] != float64(5) {
				t.Fatalf("request=%v", body)
			}
			if !reflect.DeepEqual(match["fields"], []any{"title^3", "summary", "content", "tags"}) {
				t.Fatalf("fields=%v", match["fields"])
			}
			knn, present := body["knn"].(map[string]any)
			if present != tc.knn {
				t.Fatalf("knn=%v want %v", present, tc.knn)
			}
			if present && (knn["k"] != float64(15) || knn["num_candidates"] != float64(100) || knn["field"] != "embedding") {
				t.Fatalf("knn=%v", knn)
			}
		})
	}
}

func TestEsSearchRejectsHTTPAndMalformedResponses(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{{http.StatusBadRequest, `{}`}, {http.StatusOK, `not-json`}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Elastic-Product", "Elasticsearch")
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{srv.URL}, DisableRetry: true})
		if err != nil {
			t.Fatal(err)
		}
		_, err = (&esIndexer{client: client, index: "articles"}).Search(context.Background(), "q", 5, 0)
		srv.Close()
		if err == nil {
			t.Fatalf("status %d body %s: expected error", tc.status, tc.body)
		}
	}
}

func TestEsIndexSettings(t *testing.T) {
	withVec := esIndexSettings(1024)
	if !strings.Contains(withVec, "dense_vector") || !strings.Contains(withVec, "1024") {
		t.Fatal("expected dense_vector mapping")
	}
	withoutVec := esIndexSettings(0)
	if strings.Contains(withoutVec, "dense_vector") {
		t.Fatal("dense_vector should be absent when dims=0")
	}
}

func TestEmbeddingInput(t *testing.T) {
	a := &biz.Article{Title: "t", Summary: "s", ContentMD: "body"}
	got := embeddingInput(a)
	if !strings.Contains(got, "t") || !strings.Contains(got, "s") || !strings.Contains(got, "body") {
		t.Fatalf("unexpected embedding input: %q", got)
	}
}

func TestEsIndexDimensions(t *testing.T) {
	if esIndexDimensions(nil) != 0 {
		t.Fatal("nil embedder should return 0")
	}
}
