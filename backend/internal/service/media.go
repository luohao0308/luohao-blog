package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/go-kratos/kratos/v3/transport"
	httpbody "google.golang.org/genproto/googleapis/api/httpbody"
)

type MediaService struct {
	v1.UnimplementedMediaServiceServer
	media *biz.MediaUsecase
}

func NewMediaService(media *biz.MediaUsecase) *MediaService { return &MediaService{media: media} }

func (s *MediaService) UploadImage(ctx context.Context, req *v1.UploadMediaImageRequest) (*v1.MediaImage, error) {
	url, err := s.media.SaveImage(ctx, req.GetData())
	if err != nil {
		return nil, err
	}
	return &v1.MediaImage{Url: url}, nil
}

func (s *MediaService) GetImage(ctx context.Context, req *v1.GetMediaImageRequest) (*httpbody.HttpBody, error) {
	data, contentType, err := s.media.GetImage(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	return &httpbody.HttpBody{ContentType: contentType, Data: data}, nil
}
