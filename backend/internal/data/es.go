package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"

	"github.com/elastic/go-elasticsearch/v8"
)

// esIndexSettings returns the article index mapping using analyzers available
// in the stock Elasticsearch image. The standard analyzer keeps the index
// self-contained; Chinese text is still searchable without an optional plugin.
// dims > 0 adds a dense_vector field for semantic search; a BM25-only index
// (embedding disabled) omits it, matching the S1 mapping exactly.
func esIndexSettings(dims int) string {
	properties := `{
      "slug":         { "type": "keyword" },
      "title":        { "type": "text", "analyzer": "standard", "fields": { "keyword": { "type": "keyword" } } },
      "summary":      { "type": "text", "analyzer": "standard" },
      "content":      { "type": "text", "analyzer": "standard" },
      "tags":         { "type": "keyword" },
      "published_at": { "type": "date" }`
	if dims > 0 {
		properties += fmt.Sprintf(`,
      "embedding":    { "type": "dense_vector", "dims": %d, "index": true, "similarity": "cosine" }`, dims)
	}
	return `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0
  },
  "mappings": {
    "properties": ` + properties + `
    }
  }
}`
}

// embeddingInput builds the single text fed to the embedding model. Content is
// truncated: provider token limits vary and the vector only needs to represent
// the article's topic, not carry the full text.
func embeddingInput(a *biz.Article) string {
	const maxContentRunes = 4000
	content := a.ContentMD
	if runes := []rune(content); len(runes) > maxContentRunes {
		content = string(runes[:maxContentRunes])
	}
	switch {
	case a.Summary != "" && a.Title != "":
		return a.Title + "\n" + a.Summary + "\n" + content
	case a.Title != "":
		return a.Title + "\n" + content
	default:
		return content
	}
}

// esArticleDoc is the indexed shape of a published article. Embedding is
// omitted when the embedder is disabled or the call failed, so those documents
// stay BM25-only.
type esArticleDoc struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	PublishedAt time.Time `json:"published_at"`
	Embedding   []float32 `json:"embedding,omitempty"`
}

// esSearchResponse carries the fields this client reads back from a search.
type esSearchResponse struct {
	Hits struct {
		Hits []struct {
			Source esArticleDoc `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type esIndexer struct {
	client   *elasticsearch.Client
	index    string
	embedder Embedder
}

// NewEsIndexer builds the Elasticsearch-backed article search index. A nil or
// empty config yields a nil indexer: the usecase treats that as "search
// disabled" and the write path skips indexing entirely. An absent or
// incomplete embedding config yields a BM25-only index; vectors appear once
// the embedding env vars are set and the index is rebuilt.
func NewEsIndexer(c *conf.Bootstrap) (biz.ArticleSearchIndex, error) {
	es := c.GetEs()
	if es == nil || len(es.GetAddresses()) == 0 {
		return nil, nil
	}
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: es.GetAddresses(),
	})
	if err != nil {
		return nil, fmt.Errorf("es client: %w", err)
	}
	index := es.GetIndex()
	if index == "" {
		index = "articles"
	}
	embedder := NewEmbeddingClient(c.GetEmbedding())
	idx := &esIndexer{client: client, index: index, embedder: embedder}
	if err := idx.ensureIndex(context.Background(), esIndexDimensions(embedder)); err != nil {
		// Elasticsearch is an optional search enhancement. Keep the API and
		// authoring path available when the cluster is temporarily offline;
		// indexing/search calls will continue to degrade independently.
		log.Printf("es: unavailable during startup, search disabled: %v", err)
		return nil, nil
	}
	if embedder != nil {
		if err := idx.checkVectorMapping(context.Background()); err != nil {
			log.Printf("es: vector mapping check skipped: %v", err)
		}
	}
	return idx, nil
}

// esIndexDimensions reports the dense_vector dimension for the current
// configuration; 0 means no vector field.
func esIndexDimensions(e Embedder) int {
	if c, ok := e.(*embeddingClient); ok {
		return c.dims
	}
	return 0
}

// checkVectorMapping warns when the live index cannot hold vectors (e.g. it
// was created before embedding was enabled). The index keeps working BM25-only;
// re-running the reindex tool recreates it with the dense_vector mapping.
func (e *esIndexer) checkVectorMapping(ctx context.Context) error {
	res, err := e.client.Indices.GetMapping(e.client.Indices.GetMapping.WithContext(ctx),
		e.client.Indices.GetMapping.WithIndex(e.index))
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return fmt.Errorf("es mapping: %s", res.String())
	}
	var out struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	// The response nests the mapping under the index name.
	wrapper := map[string]json.RawMessage{}
	if err := json.NewDecoder(res.Body).Decode(&wrapper); err != nil {
		return err
	}
	for _, raw := range wrapper {
		if err := json.Unmarshal(raw, &out); err != nil {
			return err
		}
		break
	}
	if field, ok := out.Properties["embedding"]; !ok || field.Type != "dense_vector" {
		log.Printf("es: index %s has no dense_vector mapping; semantic vectors are dropped until reindex", e.index)
	}
	return nil
}

// ensureIndex creates the index with the article mapping when missing.
func (e *esIndexer) ensureIndex(ctx context.Context, dims int) error {
	res, err := e.client.Indices.Exists([]string{e.index},
		e.client.Indices.Exists.WithContext(ctx))
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == 200 {
		return nil
	}
	if res.StatusCode != 404 {
		return fmt.Errorf("es exists: unexpected status %d", res.StatusCode)
	}
	res, err = e.client.Indices.Create(e.index,
		e.client.Indices.Create.WithContext(ctx),
		e.client.Indices.Create.WithBody(strings.NewReader(esIndexSettings(dims))))
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return fmt.Errorf("es create index: %s", res.String())
	}
	log.Printf("es: created index %s", e.index)
	return nil
}

// IndexArticle upserts a published article document keyed by slug. Documents
// are only written for published articles; anything else is removed. When an
// embedder is configured the document carries a vector; an embedding failure
// is downgraded to a BM25-only document, never a failed article write.
func (e *esIndexer) IndexArticle(ctx context.Context, a *biz.Article) error {
	if a == nil || a.Status != biz.ArticleStatusPublished {
		slug := ""
		if a != nil {
			slug = a.Slug
		}
		return e.RemoveArticle(ctx, slug)
	}
	published := time.Now()
	if a.PublishedAt != nil {
		published = *a.PublishedAt
	}
	doc := esArticleDoc{
		Slug:        a.Slug,
		Title:       a.Title,
		Summary:     a.Summary,
		Content:     a.ContentMD,
		Tags:        a.Tags,
		PublishedAt: published,
	}
	if e.embedder != nil {
		embedCtx, cancel := context.WithTimeout(ctx, embedCallTimeout)
		vectors, err := e.embedder.Embed(embedCtx, []string{embeddingInput(a)})
		cancel()
		if err != nil {
			log.Printf("es: embedding failed for %s, indexing BM25-only: %v", a.Slug, err)
		} else if len(vectors) == 1 {
			doc.Embedding = vectors[0]
		}
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	res, err := e.client.Index(e.index, bytes.NewReader(body),
		e.client.Index.WithContext(ctx),
		e.client.Index.WithDocumentID(a.Slug),
		e.client.Index.WithRefresh("false"))
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return fmt.Errorf("es index: %s", res.String())
	}
	return nil
}

// embedCallTimeout bounds the synchronous embedding call inside the article
// write path so a slow provider cannot stall authoring.
const embedCallTimeout = 15 * time.Second

// RecreateIndex drops and re-creates the index with the current mapping,
// including the dense_vector field when an embedder is configured. The index
// is derived state rebuilt from MySQL, so recreation is the mapping-evolution
// path documented in the M4 plan.
func (e *esIndexer) RecreateIndex(ctx context.Context) error {
	res, err := e.client.Indices.Delete([]string{e.index},
		e.client.Indices.Delete.WithContext(ctx))
	if err != nil {
		return err
	}
	_ = res.Body.Close()
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("es delete index: %s", res.String())
	}
	return e.ensureIndex(ctx, esIndexDimensions(e.embedder))
}

// Refresh flushes pending index writes so re-indexed documents are searchable
// immediately instead of after the next 1s auto-refresh.
func (e *esIndexer) Refresh(ctx context.Context) error {
	res, err := e.client.Indices.Refresh(e.client.Indices.Refresh.WithContext(ctx),
		e.client.Indices.Refresh.WithIndex(e.index))
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return fmt.Errorf("es refresh: %s", res.String())
	}
	return nil
}

// RemoveArticle drops the article document, tolerating a missing one.
func (e *esIndexer) RemoveArticle(ctx context.Context, slug string) error {
	if slug == "" {
		return nil
	}
	res, err := e.client.Delete(e.index, slug,
		e.client.Delete.WithContext(ctx))
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("es delete: %s", res.String())
	}
	return nil
}

// Search runs a multi_match BM25 query over title (boosted), summary,
// content, and tags, returning slugs best-match first.
func (e *esIndexer) Search(ctx context.Context, query string, limit, offset int) ([]string, error) {
	body, err := json.Marshal(map[string]any{
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":  query,
				"fields": []string{"title^3", "summary", "content", "tags"},
			},
		},
		"from": offset,
		"size": limit,
	})
	if err != nil {
		return nil, err
	}
	res, err := e.client.Search(
		e.client.Search.WithContext(ctx),
		e.client.Search.WithIndex(e.index),
		e.client.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return nil, fmt.Errorf("es search: %s", res.String())
	}
	var out esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	slugs := make([]string, 0, len(out.Hits.Hits))
	for _, hit := range out.Hits.Hits {
		slugs = append(slugs, hit.Source.Slug)
	}
	return slugs, nil
}
