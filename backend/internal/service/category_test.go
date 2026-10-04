package service

import (
	"context"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// stubCategoryRepo only implements the CRUD surface the service tests
// exercise; list returns a fixed page so pagination tokens can be asserted.
type stubCategoryRepo struct {
	created *biz.Category
}

func (s *stubCategoryRepo) FindBySlug(context.Context, string) (*biz.Category, error) {
	return nil, biz.ErrCategoryNotFound
}
func (s *stubCategoryRepo) ListCategories(_ context.Context, _ int, _ int) ([]*biz.Category, error) {
	return []*biz.Category{
		{Slug: "engineering", Name: "工程实践", ArticleCount: 2},
		{Slug: "notes", Name: "随笔", ArticleCount: 0},
	}, nil
}
func (s *stubCategoryRepo) CreateCategory(_ context.Context, c *biz.Category) (*biz.Category, error) {
	s.created = c
	return c, nil
}
func (s *stubCategoryRepo) UpdateCategory(_ context.Context, c *biz.Category) (*biz.Category, error) {
	return c, nil
}
func (s *stubCategoryRepo) DeleteCategory(context.Context, string) error {
	return nil
}

func TestCreateCategoryValidation(t *testing.T) {
	svc := NewCategoryService(biz.NewCategoryUsecase(&stubCategoryRepo{}))
	for _, tc := range []struct {
		name string
		in   *v1.Category
	}{
		{"bad slug", &v1.Category{Slug: "Not A Slug", Name: "x"}},
		{"empty name", &v1.Category{Slug: "ok", Name: ""}},
		{"negative sort", &v1.Category{Slug: "ok", Name: "x", Sort: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateCategory(context.Background(), &v1.CreateCategoryRequest{Category: tc.in})
			if !kratoserrors.IsBadRequest(err) {
				t.Fatalf("error = %v, want bad request", err)
			}
		})
	}
}

func TestCreateCategoryCarriesName(t *testing.T) {
	repo := &stubCategoryRepo{}
	svc := NewCategoryService(biz.NewCategoryUsecase(repo))
	out, err := svc.CreateCategory(context.Background(), &v1.CreateCategoryRequest{
		Category: &v1.Category{Slug: "engineering", Name: "工程实践"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Slug != "engineering" || out.Name != "工程实践" {
		t.Fatalf("reply = %+v, want slug/name preserved", out)
	}
}

func TestUpdateCategoryRejectsUnknownUpdateMaskPath(t *testing.T) {
	svc := NewCategoryService(biz.NewCategoryUsecase(&stubCategoryRepo{}))
	_, err := svc.UpdateCategory(context.Background(), &v1.UpdateCategoryRequest{
		Category:   &v1.Category{Slug: "engineering"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"slug"}},
	})
	if !kratoserrors.IsBadRequest(err) {
		t.Fatalf("error = %v, want bad request", err)
	}
}

func TestListCategoriesCarriesCounts(t *testing.T) {
	svc := NewCategoryService(biz.NewCategoryUsecase(&stubCategoryRepo{}))
	set, err := svc.ListCategories(context.Background(), &v1.ListCategoriesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Categories) != 2 {
		t.Fatalf("categories = %d, want 2", len(set.Categories))
	}
	if set.Categories[0].ArticleCount != 2 || set.Categories[1].ArticleCount != 0 {
		t.Fatalf("counts = %d/%d, want 2/0", set.Categories[0].ArticleCount, set.Categories[1].ArticleCount)
	}
	if set.NextPageToken != "" {
		t.Fatalf("next page token = %q, want empty for a short page", set.NextPageToken)
	}
}
