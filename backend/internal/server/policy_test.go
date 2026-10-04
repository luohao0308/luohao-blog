package server

import (
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
	"github.com/luohao0308/luohao-blog/backend/internal/service"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

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
	srv := kratoshttp.NewServer()
	v1.RegisterArticleServiceHTTPServer(srv, service.NewArticleService(biz.NewArticleUsecase(nil, nil)))
	v1.RegisterCategoryServiceHTTPServer(srv, service.NewCategoryService(biz.NewCategoryUsecase(nil)))
	if err := validatePolicyCoverage(srv, authz); err != nil {
		t.Fatalf("policy coverage: %v", err)
	}
}
