package server

import (
	"context"
	nethttp "net/http"
	"strconv"
	"time"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// API metrics on the default registry, scraped by Prometheus at /metrics
// (mounted on the internal HTTP server; the container publishes no ports so
// the endpoint is unreachable from the public internet).
var (
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "blog_api_requests_total",
			Help: "Total API requests by method, operation template and status code.",
		},
		[]string{"method", "operation", "code"},
	)
	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "blog_api_request_duration_seconds",
			Help:    "API request latency in seconds by method and operation template.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"method", "operation"},
	)
	apiRequestsInflight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "blog_api_requests_inflight",
		Help: "Number of API requests currently being served.",
	})
)

func init() {
	prometheus.MustRegister(apiRequestsTotal, apiRequestDuration, apiRequestsInflight)
}

// metricsHandler serves the Prometheus scrape endpoint on the Kratos HTTP
// server. Registered via Server.Handle so it bypasses the API middleware
// chain (no auth/rate limit — the port is compose-internal only).
func metricsHandler() nethttp.Handler {
	return promhttp.Handler()
}

// requestLabels derives the label pair from the server transport context.
// HTTP uses the route template ("/v1/articles/{id}") so high-cardinality
// path segments never become label values; gRPC uses the full method.
func requestLabels(ctx context.Context) (method, operation string) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return "", ""
	}
	operation = tr.Operation()
	if ht, ok := tr.(khttp.Transporter); ok {
		if r := ht.Request(); r != nil {
			return r.Method, operation
		}
	}
	return "grpc", operation
}

// Metrics records per-operation request count, latency and in-flight gauge.
// Place it first in the middleware chain so recovery and auth cost is
// included in the measurement.
func Metrics() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			method, operation := requestLabels(ctx)
			if method == "" {
				return handler(ctx, req)
			}
			apiRequestsInflight.Inc()
			start := time.Now()
			reply, err := handler(ctx, req)
			apiRequestsInflight.Dec()
			code := "200"
			if err != nil {
				code = strconv.Itoa(kerrors.Code(err))
			}
			elapsed := time.Since(start).Seconds()
			apiRequestsTotal.WithLabelValues(method, operation, code).Inc()
			apiRequestDuration.WithLabelValues(method, operation).Observe(elapsed)
			return reply, err
		}
	}
}
