package biz

import (
	"context"
	"regexp"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"

	"github.com/go-kratos/kratos/v3/errors"
)

const (
	// MaxMediaBytes bounds decoded article images. The JSON transport has a
	// separate 4 MiB cap to accommodate base64 expansion.
	MaxMediaBytes     = 2 << 20
	assetsMediaPrefix = "/v1/assets/media/"
)

var (
	ErrMediaInvalidArgument = errors.BadRequest(v1.ErrorReason_MEDIA_INVALID_ARGUMENT.String(), "invalid media image")
	ErrMediaNotFound        = errors.NotFound(v1.ErrorReason_MEDIA_NOT_FOUND.String(), "media image not found")
	ErrMediaStorageFailed   = errors.InternalServer(v1.ErrorReason_MEDIA_STORAGE_FAILED.String(), "media storage unavailable")
	mediaNamePattern        = regexp.MustCompile(`^[0-9a-f]{32}\.(jpg|png|webp)$`)
)

// ValidMediaURL reports whether value is empty or a server-generated,
// site-relative media URL. Article covers never accept an arbitrary external
// URL or scriptable scheme.
func ValidMediaURL(value string) bool {
	if value == "" {
		return true
	}
	if len(value) <= len(assetsMediaPrefix) || value[:len(assetsMediaPrefix)] != assetsMediaPrefix {
		return false
	}
	return mediaNamePattern.MatchString(value[len(assetsMediaPrefix):])
}

type MediaStore interface {
	Save(context.Context, string, []byte) (string, error)
	Load(context.Context, string) ([]byte, string, error)
}

type MediaUsecase struct{ store MediaStore }

func NewMediaUsecase(store MediaStore) *MediaUsecase { return &MediaUsecase{store: store} }

func (uc *MediaUsecase) SaveImage(ctx context.Context, data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaxMediaBytes {
		return "", ErrMediaInvalidArgument
	}
	ext, ok := sniffImageExt(data)
	if !ok {
		return "", ErrMediaInvalidArgument
	}
	name, err := uc.store.Save(ctx, ext, data)
	if err != nil {
		// Storage errors may contain absolute paths or operating-system detail.
		// Keep the client-facing error stable and deliberately omit the cause.
		return "", ErrMediaStorageFailed
	}
	return assetsMediaPrefix + name, nil
}

func (uc *MediaUsecase) GetImage(ctx context.Context, name string) ([]byte, string, error) {
	if !mediaNamePattern.MatchString(name) {
		return nil, "", ErrMediaNotFound
	}
	data, contentType, err := uc.store.Load(ctx, name)
	if err != nil {
		return nil, "", ErrMediaNotFound
	}
	return data, contentType, nil
}
