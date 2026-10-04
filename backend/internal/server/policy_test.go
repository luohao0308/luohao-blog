package server

import (
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
	"github.com/luohao0308/luohao-blog/backend/internal/service"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func registerCategoryRoutes(t *testing.T) *kratoshttp.Server {
	t.Helper()
	srv := kratoshttp.NewServer()
	v1.RegisterArticleServiceHTTPServer(srv, service.NewArticleService(biz.NewArticleUsecase(nil, nil)))
	v1.RegisterCategoryServiceHTTPServer(srv, service.NewCategoryService(biz.NewCategoryUsecase(nil)))
	v1.RegisterSubscriberServiceHTTPServer(srv, service.NewSubscriberService(biz.NewSubscriberUsecase(nil, nil)))
	return srv
}

// TestPolicyCoversCategoryRoutes registers the article and category services
// against the real embedded policy and runs the same coverage check the boot
// path runs. It pins every /v1/categories route to a policy row: default
// deny would otherwise turn a missing row into an unreachable route at
// runtime instead of a failed boot.
func TestPolicyCoversCategoryRoutes(t *testing.T) {
	authz, err := data.NewAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePolicyCoverage(registerCategoryRoutes(t), authz); err != nil {
		t.Fatalf("policy coverage: %v", err)
	}
}

// TestListRouteRegistersBeforeSlugRoute pins the registration order. The
// gorilla/mux router matches in registration order, so a GET
// /v1/categories/{slug} registered before the static /v1/categories/list
// captures the list path and inherits GetCategory's admin-only policy — the
// anonymous 401 that shipped with the category slice. The same convention
// keeps /v1/articles/list ahead of /v1/articles/{slug}.
func TestListRouteRegistersBeforeSlugRoute(t *testing.T) {
	var order []string
	if err := registerCategoryRoutes(t).WalkRoute(func(r kratoshttp.RouteInfo) error {
		if r.Method == "GET" && (r.Path == "/v1/categories/list" || r.Path == "/v1/categories/{slug}") {
			order = append(order, r.Path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "/v1/categories/list" {
		t.Fatalf("GET route registration order = %v, want list before {slug}", order)
	}
}
