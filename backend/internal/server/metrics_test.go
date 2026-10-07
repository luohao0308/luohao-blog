package server

import (
	"context"
	"testing"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsRecordsSuccessAndError(t *testing.T) {
	mw := Metrics()

	ok := mw(func(ctx context.Context, req any) (any, error) { return "ok", nil })
	if _, err := ok(newTestTransport("/blog.v1.ArticleService/ListArticle", nil), nil); err != nil {
		t.Fatalf("success handler returned error: %v", err)
	}
	if got := testutil.ToFloat64(apiRequestsTotal.WithLabelValues("grpc", "/blog.v1.ArticleService/ListArticle", "200")); got != 1 {
		t.Fatalf("success counter = %v, want 1", got)
	}

	notFound := mw(func(ctx context.Context, req any) (any, error) {
		return nil, kerrors.NotFound("article", "missing")
	})
	ctx := newTestTransport("/blog.v1.ArticleService/GetArticle", nil)
	if _, err := notFound(ctx, nil); err == nil {
		t.Fatal("error handler returned nil error")
	}
	if got := testutil.ToFloat64(apiRequestsTotal.WithLabelValues("grpc", "/blog.v1.ArticleService/GetArticle", "404")); got != 1 {
		t.Fatalf("error counter = %v, want 1", got)
	}
}

func TestMetricsSkipsNonTransportContext(t *testing.T) {
	// Requests without a server transport context (should not happen on a
	// real server) must be served but not recorded — no empty labels.
	handler := Metrics()(func(ctx context.Context, req any) (any, error) { return "ok", nil })
	if _, err := handler(context.Background(), nil); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	// No series must ever carry empty labels; asserting the child value
	// (0 = never incremented) keeps this independent of other tests sharing
	// the default registry.
	if got := testutil.ToFloat64(apiRequestsTotal.WithLabelValues("", "", "")); got != 0 {
		t.Fatalf("recorded %v requests with empty labels, want 0", got)
	}
}
