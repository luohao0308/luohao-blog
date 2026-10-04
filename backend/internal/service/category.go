package service

import (
	"context"
	"errors"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var allowedCategoryUpdateMaskPaths = map[string]struct{}{
	"name": {},
	"sort": {},
}

// CategoryService is a category service.
type CategoryService struct {
	v1.UnimplementedCategoryServiceServer

	uc *biz.CategoryUsecase
}

// NewCategoryService new a category service.
func NewCategoryService(uc *biz.CategoryUsecase) *CategoryService {
	return &CategoryService{uc: uc}
}

// CreateCategory creates a category.
func (s *CategoryService) CreateCategory(ctx context.Context, req *v1.CreateCategoryRequest) (*v1.Category, error) {
	category, err := s.uc.Create(ctx, convertCategory(req.GetCategory()))
	if err != nil {
		return nil, err
	}
	return convertCategoryReply(category), nil
}

// GetCategory returns a category by slug.
func (s *CategoryService) GetCategory(ctx context.Context, req *v1.GetCategoryRequest) (*v1.Category, error) {
	category, err := s.uc.Get(ctx, req.GetSlug())
	if err != nil {
		return nil, err
	}
	return convertCategoryReply(category), nil
}

// ListCategories lists categories with their published-article counts.
func (s *CategoryService) ListCategories(ctx context.Context, req *v1.ListCategoriesRequest) (*v1.CategorySet, error) {
	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, invalidListArgument(err)
	}
	if !pageOffsetWithinWindow(pageToken.Offset) {
		return nil, invalidListArgument(errPageOffsetOutOfRange)
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	clampPageSize(&req.PageSize)
	categories, err := s.uc.List(ctx, int(req.PageSize), int(pageToken.Offset))
	if err != nil {
		return nil, err
	}
	set := &v1.CategorySet{
		Categories: make([]*v1.Category, 0, len(categories)),
	}
	if len(categories) >= int(req.PageSize) {
		set.NextPageToken = pageToken.Next(req).String()
	}
	for _, category := range categories {
		set.Categories = append(set.Categories, convertCategoryReply(category))
	}
	return set, nil
}

// UpdateCategory updates a category via field mask. The slug is immutable
// and only identifies the target record.
func (s *CategoryService) UpdateCategory(ctx context.Context, req *v1.UpdateCategoryRequest) (*v1.Category, error) {
	if req.GetCategory().GetSlug() == "" || req.GetUpdateMask() == nil || len(req.GetUpdateMask().GetPaths()) == 0 {
		return nil, biz.ErrCategoryInvalidArgument
	}
	for _, path := range req.GetUpdateMask().GetPaths() {
		if _, ok := allowedCategoryUpdateMaskPaths[path]; !ok {
			return nil, biz.ErrCategoryInvalidArgument.WithCause(errors.New("unknown update_mask path: " + path))
		}
	}
	current, err := s.GetCategory(ctx, &v1.GetCategoryRequest{Slug: req.GetCategory().GetSlug()})
	if err != nil {
		return nil, err
	}
	fieldmask.Update(req.GetUpdateMask(), current, req.GetCategory())
	category, err := s.uc.Update(ctx, convertCategory(current))
	if err != nil {
		return nil, err
	}
	return convertCategoryReply(category), nil
}

// DeleteCategory deletes a category; referencing articles become
// uncategorized.
func (s *CategoryService) DeleteCategory(ctx context.Context, req *v1.DeleteCategoryRequest) (*emptypb.Empty, error) {
	if err := s.uc.Delete(ctx, req.GetSlug()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// convertCategory parses an incoming proto into a DO. Server-assigned fields
// (id, article_count, timestamps) are omitted: only the mutable fields cross
// over. The slug rides along as the create payload and the update identifier.
func convertCategory(in *v1.Category) *biz.Category {
	if in == nil {
		return nil
	}
	return &biz.Category{
		Slug: in.GetSlug(),
		Name: in.GetName(),
		Sort: in.GetSort(),
	}
}

func convertCategoryReply(in *biz.Category) *v1.Category {
	if in == nil {
		return nil
	}
	return &v1.Category{
		Id:           in.ID.String(),
		Slug:         in.Slug,
		Name:         in.Name,
		Sort:         in.Sort,
		ArticleCount: in.ArticleCount,
		CreatedAt:    timestamppb.New(in.CreatedAt),
		UpdatedAt:    timestamppb.New(in.UpdatedAt),
	}
}
