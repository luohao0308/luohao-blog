// Command reindex rebuilds the Elasticsearch article index from MySQL: it
// drops and re-creates the index with the current mapping (including the
// dense_vector field when embedding is configured), then re-indexes every
// published article. Run it after enabling embedding, after changing the
// embedding model or its dimensions, or whenever the index drifted.
//
// Usage (from backend/):
//
//	KRATOS_EMBEDDING_BASE_URL=... KRATOS_EMBEDDING_API_KEY=... \
//	KRATOS_EMBEDDING_MODEL=... KRATOS_EMBEDDING_DIMENSIONS=1536 \
//	go run ./cmd/reindex -conf ./configs
//
// With no embedding env vars the index is recreated BM25-only.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/env"
	"github.com/go-kratos/kratos/v3/config/file"
)

var flagConf string

func init() {
	flag.StringVar(&flagConf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

func main() {
	flag.Parse()
	c := config.New(
		config.WithSource(
			file.NewSource(flagConf),
			env.NewSource("KRATOS"),
		),
	)
	defer func() { _ = c.Close() }()
	if err := c.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "reindex: load config: %v\n", err)
		os.Exit(1)
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		fmt.Fprintf(os.Stderr, "reindex: scan config: %v\n", err)
		os.Exit(1)
	}

	store, cleanup, err := data.NewData(bc.Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reindex: open data: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	indexer, err := data.NewEsIndexer(&bc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reindex: es client: %v\n", err)
		os.Exit(1)
	}
	if indexer == nil {
		fmt.Fprintln(os.Stderr, "reindex: elasticsearch is not configured or unreachable; nothing to do")
		os.Exit(1)
	}

	ctx := context.Background()
	if err := indexer.RecreateIndex(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "reindex: recreate index: %v\n", err)
		os.Exit(1)
	}

	articles := biz.NewArticleUsecase(data.NewArticleRepo(store, nil), nil)
	published, err := articles.ListArticles(ctx, biz.ListPublic(), biz.ListLimit(1000))
	if err != nil {
		fmt.Fprintf(os.Stderr, "reindex: list published articles: %v\n", err)
		os.Exit(1)
	}
	for _, a := range published {
		if err := indexer.IndexArticle(ctx, a); err != nil {
			fmt.Fprintf(os.Stderr, "reindex: index %s: %v\n", a.Slug, err)
			os.Exit(1)
		}
	}
	// The write path indexes with refresh=false; a rebuild must stay visible.
	if r, ok := indexer.(interface{ Refresh(context.Context) error }); ok {
		if err := r.Refresh(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "reindex: refresh: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("reindex: rebuilt index with %d published articles\n", len(published))
}
