package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/go-kratos/kratos/v3/transport"
	httpbody "google.golang.org/genproto/googleapis/api/httpbody"
)

// UserService serves the authenticated account's profile: the display name
// and the avatar. Every operation takes its identity from the verified
// access token — the payload never names a target account, so the policy
// layer's "authenticated" check plus this usecase-level re-check mean an
// account can only ever edit itself.
type UserService struct {
	v1.UnimplementedUserServiceServer

	users   *biz.UserUsecase
	avatars *biz.AvatarUsecase
}

// NewUserService new a User service.
func NewUserService(users *biz.UserUsecase, avatars *biz.AvatarUsecase) *UserService {
	return &UserService{users: users, avatars: avatars}
}

// UpdateProfile renames the calling account.
func (s *UserService) UpdateProfile(ctx context.Context, req *v1.UpdateProfileRequest) (*v1.User, error) {
	claims, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	u, err := s.users.UpdateProfile(ctx, claims.UserID, req.GetDisplayName())
	if err != nil {
		return nil, err
	}
	return convertUser(u), nil
}

// UploadAvatar stores a new avatar for the calling account and returns the
// updated account with its fresh avatar_url.
func (s *UserService) UploadAvatar(ctx context.Context, req *v1.UploadAvatarRequest) (*v1.User, error) {
	claims, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	u, err := s.avatars.SaveAvatar(ctx, claims.UserID, req.GetData())
	if err != nil {
		return nil, err
	}
	return convertUser(u), nil
}

// GetAvatar serves the stored avatar bytes as a raw HTTP body. Public: the
// header menu and comment lists render avatars for every visitor.
func (s *UserService) GetAvatar(ctx context.Context, req *v1.GetAvatarRequest) (*httpbody.HttpBody, error) {
	data, contentType, err := s.avatars.GetAvatar(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	// Random filenames make avatar URLs effectively content-addressed: a new
	// upload changes the name, so previously served copies can cache hard.
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	return &httpbody.HttpBody{ContentType: contentType, Data: data}, nil
}
