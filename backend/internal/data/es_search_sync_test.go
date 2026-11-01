package data

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

func syncTestIndexer(t *testing.T, handler http.HandlerFunc) *esIndexer {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}, DisableRetry: true})
	if err != nil {
		t.Fatal(err)
	}
	return &esIndexer{client: client, index: "articles"}
}
func TestArticleIndexSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name              string
		status            int
		body              string
		wantError, exists bool
	}{
		{"present", 200, `{"found":true,"_source":{"slug":"test","title":"title","summary":"summary","content":"正文","tags":["go"],"published_at":"2026-01-01T00:00:00Z"}}`, false, true},
		{"missing document", 404, `{"found":false}`, false, false},
		{"missing index", 404, `{"error":{"type":"index_not_found_exception"}}`, true, false},
		{"upstream failure", 503, `{}`, true, false},
		{"invalid JSON", 200, `invalid`, true, false},
		{"missing source", 200, `{"found":true}`, true, false},
		{"wrong source", 200, `{"found":true,"_source":{"slug":"other"}}`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx := syncTestIndexer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/articles/_doc/test" || r.URL.Query().Get("realtime") != "true" {
					t.Errorf("request %s %s", r.Method, r.URL)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			snapshot, err := idx.GetArticleIndexSnapshot(context.Background(), "test")
			if (err != nil) != tc.wantError {
				t.Fatalf("snapshot=%+v err=%v", snapshot, err)
			}
			if err == nil && (snapshot.Exists != tc.exists || snapshot.SemanticExpected) {
				t.Fatal(snapshot)
			}
			if err == nil && tc.exists && (snapshot.Article.ContentMD != "正文" || snapshot.Article.Title != "title" || snapshot.Article.PublishedAt == nil) {
				t.Fatal(snapshot.Article)
			}
		})
	}
}

type snapshotNoCallEmbedder struct{}

func (snapshotNoCallEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	panic("snapshot must not call embedding")
}

func TestArticleIndexSnapshotLiveVectorMapping(t *testing.T) {
	for _, tc := range []struct {
		name, mapping, vector        string
		ready, repairable, wantError bool
	}{
		{"ready", `{"type":"dense_vector","dims":2}`, `[0.1,0.2]`, true, true, false},
		{"missing vector", `{"type":"dense_vector","dims":2}`, `[]`, false, true, false},
		{"wrong vector dimensions", `{"type":"dense_vector","dims":2}`, `[0.1]`, false, true, false},
		{"index disabled", `{"type":"dense_vector","dims":2,"index":false}`, `[0.1,0.2]`, false, false, false},
		{"missing dense mapping", `{"type":"float"}`, `[0.1,0.2]`, false, false, false},
		{"mapping fails", ``, `[]`, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx := syncTestIndexer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/articles/_mapping" {
					if tc.mapping == "" {
						w.WriteHeader(503)
						return
					}
					_, _ = w.Write([]byte(`{"articles":{"mappings":{"properties":{"embedding":` + tc.mapping + `}}}}`))
					return
				}
				_, _ = w.Write([]byte(`{"found":true,"_source":{"slug":"test","embedding":` + tc.vector + `}}`))
			})
			idx.embedder = snapshotNoCallEmbedder{}
			snapshot, err := idx.GetArticleIndexSnapshot(context.Background(), "test")
			if (err != nil) != tc.wantError {
				t.Fatal(err)
			}
			if err == nil && (!snapshot.SemanticExpected || snapshot.SemanticReady != tc.ready || snapshot.SemanticRepairable != tc.repairable) {
				t.Fatalf("%+v", snapshot)
			}
		})
	}
}
func TestRetryArticleIndexWaitsAndRetainsBM25(t *testing.T) {
	var refreshes []string
	var doc esArticleDoc
	idx := syncTestIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(200)
			return
		}
		refreshes = append(refreshes, r.URL.Query().Get("refresh"))
		if r.Method == "PUT" {
			if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
				t.Error(err)
			}
		}
		_, _ = w.Write([]byte(`{"result":"updated"}`))
	})
	idx.embedder = searchTestEmbedder{fail: true}
	now := time.Now()
	a := &biz.Article{Slug: "test", Title: "title", ContentMD: "body", Status: biz.ArticleStatusPublished, PublishedAt: &now}
	if err := idx.IndexArticle(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if err := idx.RetryArticleIndex(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	a.Status = biz.ArticleStatusDraft
	if err := idx.RetryArticleIndex(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if len(refreshes) != 3 || refreshes[0] != "false" || refreshes[1] != "wait_for" || refreshes[2] != "wait_for" || len(doc.Embedding) != 0 || doc.Content != "body" {
		t.Fatalf("refreshes=%v doc=%+v", refreshes, doc)
	}
}
func TestRetryArticleIndexErrors(t *testing.T) {
	idx := syncTestIndexer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	if err := idx.RetryArticleIndex(context.Background(), &biz.Article{Slug: "test", Status: biz.ArticleStatusPublished}); err == nil {
		t.Fatal("expected write failure")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := idx.GetArticleIndexSnapshot(ctx, "test"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
func TestProbeVectorMappingUsesActualESShape(t *testing.T) {
	idx := syncTestIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"articles":{"mappings":{"properties":{"embedding":{"type":"dense_vector","dims":2}}}}}`))
	})
	ready, err := idx.probeVectorMapping(context.Background())
	if err != nil || !ready {
		t.Fatalf("%v %v", ready, err)
	}
}

func TestSnapshotMissingDocPreservesSemanticConfiguration(t *testing.T) {
	idx := syncTestIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/articles/_mapping" {
			_, _ = w.Write([]byte(`{"articles":{"mappings":{"properties":{"embedding":{"type":"dense_vector","dims":2}}}}}`))
			return
		}
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"found":false}`))
	})
	idx.embedder = &embeddingClient{dims: 2}
	snapshot, err := idx.GetArticleIndexSnapshot(context.Background(), "test")
	if err != nil || snapshot.Exists || !snapshot.SemanticExpected || !snapshot.SemanticRepairable || snapshot.SemanticReady {
		t.Fatalf("%+v %v", snapshot, err)
	}
	idx.embedder = &embeddingClient{dims: 3}
	snapshot, err = idx.GetArticleIndexSnapshot(context.Background(), "test")
	if err != nil || snapshot.SemanticRepairable {
		t.Fatalf("mapping dimensions mismatch: %+v %v", snapshot, err)
	}
}

func TestSnapshotRepairsVectorReadinessConcurrently(t *testing.T) {
	var mappingReady atomic.Bool
	var sawKNN atomic.Bool
	mappingReady.Store(true)
	idx := syncTestIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/articles/_mapping":
			field := `{"type":"float"}`
			if mappingReady.Load() {
				field = `{"type":"dense_vector","dims":2}`
			}
			_, _ = w.Write([]byte(`{"articles":{"mappings":{"properties":{"embedding":` + field + `}}}}`))
		case "/articles/_search":
			var query map[string]any
			if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
				t.Error(err)
			}
			_, knn := query["knn"]
			sawKNN.Store(knn)
			_, _ = w.Write([]byte(`{"hits":{"hits":[]}}`))
		default:
			_, _ = w.Write([]byte(`{"found":true,"_source":{"slug":"test","embedding":[0.1,0.2]}}`))
		}
	})
	idx.embedder = searchTestEmbedder{}
	if idx.getVectorReady() {
		t.Fatal("expected startup cache false")
	}
	snapshot, err := idx.GetArticleIndexSnapshot(context.Background(), "test")
	if err != nil || !snapshot.SemanticReady || !idx.getVectorReady() {
		t.Fatalf("snapshot failed to repair cache: %+v %v", snapshot, err)
	}
	if _, err := idx.Search(context.Background(), "q", 4, 0); err != nil || !sawKNN.Load() {
		t.Fatalf("repaired mapping must enable actual kNN query: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if i%2 == 0 {
					if _, err := idx.GetArticleIndexSnapshot(context.Background(), "test"); err != nil {
						t.Error(err)
					}
				} else {
					if _, err := idx.Search(context.Background(), "q", 4, 0); err != nil {
						t.Error(err)
					}
				}
			}
		}(i)
	}
	wg.Wait()
	mappingReady.Store(false)
	snapshot, err = idx.GetArticleIndexSnapshot(context.Background(), "test")
	if err != nil || snapshot.SemanticReady || idx.getVectorReady() {
		t.Fatalf("unsupported mapping should disable cache: %+v %v", snapshot, err)
	}
	if _, err := idx.Search(context.Background(), "q", 4, 0); err != nil || sawKNN.Load() {
		t.Fatalf("unsupported mapping must disable actual kNN query: %v", err)
	}
}

func TestRetryIndexMissingNeverCreatesImplicitMapping(t *testing.T) {
	for _, status := range []biz.ArticleStatus{biz.ArticleStatusPublished, biz.ArticleStatusDraft} {
		idx := syncTestIndexer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "HEAD" || r.URL.Path != "/articles" {
				t.Errorf("unexpected document operation %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(404)
		})
		idx.embedder = snapshotNoCallEmbedder{}
		err := idx.RetryArticleIndex(context.Background(), &biz.Article{Slug: "test", Status: status})
		if !errors.Is(err, biz.ErrArticleSearchIndexMissing) {
			t.Fatalf("expected missing index, got %v", err)
		}
	}
	idx := syncTestIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"type":"index_not_found_exception"}}`))
	})
	if _, err := idx.GetArticleIndexSnapshot(context.Background(), "test"); !errors.Is(err, biz.ErrArticleSearchIndexMissing) {
		t.Fatalf("snapshot missing index %v", err)
	}
}
