package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

// Embedder turns text inputs into vectors, in input order.
type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

// embeddingClient calls an OpenAI-compatible /embeddings endpoint. The
// request/response shape is the de-facto standard, so any provider exposing
// it (or a self-hosted replica) works without code changes.
type embeddingClient struct {
	baseURL string
	apiKey  string
	model   string
	dims    int
	http    *http.Client
}

// NewEmbeddingClient builds the client from the embedding config. It returns
// nil when the configuration is absent or incomplete: the indexer treats a
// nil embedder as "no vectors" and the search index stays BM25-only.
func NewEmbeddingClient(c *conf.Embedding) Embedder {
	if c == nil || c.GetBaseUrl() == "" || c.GetModel() == "" || c.GetDimensions() <= 0 {
		return nil
	}
	timeout := c.GetTimeout().AsDuration()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &embeddingClient{
		baseURL: strings.TrimRight(c.GetBaseUrl(), "/"),
		apiKey:  c.GetApiKey(),
		model:   c.GetModel(),
		dims:    int(c.GetDimensions()),
		http:    &http.Client{Timeout: timeout},
	}
}

// embeddingsRequest is the OpenAI embeddings request body.
type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embeddingsResponse is the OpenAI embeddings response body; entries are
// ordered by index but not guaranteed to arrive in order.
type embeddingsResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed returns one vector per input, re-ordered into input order and
// validated against the configured dimension.
func (e *embeddingClient) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embeddingsRequest{Model: e.model, Input: inputs})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	res, err := e.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings: status %d", res.StatusCode)
	}
	var out embeddingsResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) != len(inputs) {
		return nil, fmt.Errorf("embeddings: got %d vectors for %d inputs", len(out.Data), len(inputs))
	}
	vectors := make([][]float32, len(inputs))
	for _, entry := range out.Data {
		if entry.Index < 0 || entry.Index >= len(vectors) {
			return nil, fmt.Errorf("embeddings: index %d out of range", entry.Index)
		}
		if len(entry.Embedding) != e.dims {
			return nil, fmt.Errorf("embeddings: got %d dimensions, want %d", len(entry.Embedding), e.dims)
		}
		vectors[entry.Index] = entry.Embedding
	}
	return vectors, nil
}
