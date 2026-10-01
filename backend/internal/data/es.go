package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"

	"github.com/elastic/go-elasticsearch/v8"
)

// esIndexPrefix namespaces the article index mapping. The smartcn analyzer is
// bundled with stock Elasticsearch and handles Chinese segmentation far
// better than the default per-character standard analyzer.
const esIndexSettings = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0
  },
  "mappings": {
    "properties": {
      "slug":         { "type": "keyword" },
      "title":        { "type": "text", "analyzer": "smartcn", "fields": { "keyword": { "type": "keyword" } } },
      "summary":      { "type": "text", "analyzer": "smartcn" },
      "content":      { "type": "text", "analyzer": "smartcn" },
      "tags":         { "type": "keyword" },
      "published_at": { "type": "date" }
    }
  }
}`

// esArticleDoc is the indexed shape of a published article.
type esArticleDoc struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	PublishedAt time.Time `json:"published_at"`
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
	client *elasticsearch.Client
	index  string
}

// NewEsIndexer builds the Elasticsearch-backed article search index. A nil or
// empty config yields a nil indexer: the usecase treats that as "search
// disabled" and the write path skips indexing entirely.
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
	idx := &esIndexer{client: client, index: index}
	if err := idx.ensureIndex(context.Background()); err != nil {
		// Elasticsearch is an optional search enhancement. Keep the API and
		// authoring path available when the cluster is temporarily offline;
		// indexing/search calls will continue to degrade independently.
		log.Printf("es: unavailable during startup, search disabled: %v", err)
		return nil, nil
	}
	return idx, nil
}

// ensureIndex creates the index with the article mapping when missing.
func (e *esIndexer) ensureIndex(ctx context.Context) error {
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
		e.client.Indices.Create.WithBody(bytes.NewReader([]byte(esIndexSettings))))
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
// are only written for published articles; anything else is removed.
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
	doc, err := json.Marshal(esArticleDoc{
		Slug:        a.Slug,
		Title:       a.Title,
		Summary:     a.Summary,
		Content:     a.ContentMD,
		Tags:        a.Tags,
		PublishedAt: published,
	})
	if err != nil {
		return err
	}
	res, err := e.client.Index(e.index, bytes.NewReader(doc),
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
