package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/go-kratos/kratos/v3/transport"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CollectionService struct {
	v1.UnimplementedCollectionServiceServer
	uc *biz.CollectionUsecase
}

func NewCollectionService(uc *biz.CollectionUsecase) *CollectionService {
	return &CollectionService{uc: uc}
}

func (s *CollectionService) ListCollections(ctx context.Context, req *v1.ListCollectionsRequest) (*v1.CollectionSet, error) {
	markCollectionNoStore(ctx)
	claims, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	token, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, invalidListArgument(err)
	}
	if !pageOffsetWithinWindow(token.Offset) {
		return nil, invalidListArgument(errPageOffsetOutOfRange)
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	clampPageSize(&req.PageSize)
	items, err := s.uc.List(ctx, claims.UserID, int(req.PageSize), int(token.Offset))
	if err != nil {
		return nil, err
	}
	out := &v1.CollectionSet{Collections: make([]*v1.Collection, 0, len(items))}
	for _, item := range items {
		out.Collections = append(out.Collections, convertCollectionReply(item))
	}
	if len(items) >= int(req.PageSize) {
		out.NextPageToken = token.Next(req).String()
	}
	return out, nil
}
func (s *CollectionService) SaveCollection(ctx context.Context, req *v1.SaveCollectionRequest) (*v1.Collection, error) {
	markCollectionNoStore(ctx)
	claims, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	item, err := s.uc.Save(ctx, claims.UserID, req.GetSlug())
	if err != nil {
		return nil, err
	}
	return convertCollectionReply(item), nil
}
func (s *CollectionService) DeleteCollection(ctx context.Context, req *v1.DeleteCollectionRequest) (*emptypb.Empty, error) {
	markCollectionNoStore(ctx)
	claims, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	if err := s.uc.Delete(ctx, claims.UserID, req.GetSlug()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func markCollectionNoStore(ctx context.Context) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Cache-Control", "no-store")
	}
}
func convertCollectionReply(in *biz.Collection) *v1.Collection {
	if in == nil {
		return nil
	}
	return &v1.Collection{Article: convertArticleReply(in.Article), SavedAt: timestamppb.New(in.SavedAt)}
}
