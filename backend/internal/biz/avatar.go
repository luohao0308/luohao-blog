package biz

import (
	"bytes"
	"context"
	"regexp"

	"github.com/google/uuid"
)

// MaxAvatarBytes bounds the decoded avatar image. Avatars render at tens of
// pixels in the header and comments, so 2 MiB is generous headroom even for
// high-DPI square crops.
const MaxAvatarBytes = 2 << 20

// assetsAvatarPrefix is the public, site-relative path space of stored
// avatars: GetAvatar serves GET /v1/assets/avatars/{name}. Stored paths are
// site-relative so the database never bakes in a scheme or host.
const assetsAvatarPrefix = "/v1/assets/avatars/"

// avatarNamePattern matches exactly the filenames this module generates: 32
// lowercase hex characters (a UUIDv4 without dashes) plus a known image
// extension. Anything else was never written by us, so it can only be a
// probing attempt and is answered with NOT_FOUND before the disk is touched.
var avatarNamePattern = regexp.MustCompile(`^[0-9a-f]{32}\.(jpg|png|webp)$`)

// AvatarStore persists avatar images on the server's local disk. The
// production topology has no object store; a named Docker volume keeps the
// files across container replacements.
type AvatarStore interface {
	// Save writes the image under a fresh random filename with the given
	// extension and returns the bare filename (no path, no URL prefix).
	Save(ctx context.Context, ext string, data []byte) (string, error)
	// Load returns the bytes and the HTTP content type for a stored filename.
	Load(ctx context.Context, name string) ([]byte, string, error)
}

// AvatarUsecase validates and stores avatars, and wires them onto accounts.
// Uploads are already authenticated at the service boundary; this layer owns
// the content rules (size, magic bytes) and the URL shape.
type AvatarUsecase struct {
	store AvatarStore
	users *UserUsecase
}

// NewAvatarUsecase new an Avatar usecase.
func NewAvatarUsecase(store AvatarStore, users *UserUsecase) *AvatarUsecase {
	return &AvatarUsecase{store: store, users: users}
}

// SaveAvatar validates the uploaded image, stores it, and points the
// account's avatar_url at it. Invalid size or content is the same generic
// invalid-argument error: never leak which rule tripped.
func (uc *AvatarUsecase) SaveAvatar(ctx context.Context, userID uuid.UUID, data []byte) (*User, error) {
	if len(data) == 0 || len(data) > MaxAvatarBytes {
		return nil, ErrUserInvalidArgument
	}
	ext, ok := sniffImageExt(data)
	if !ok {
		return nil, ErrUserInvalidArgument
	}
	name, err := uc.store.Save(ctx, ext, data)
	if err != nil {
		return nil, err
	}
	return uc.users.UpdateAvatar(ctx, userID, assetsAvatarPrefix+name)
}

// GetAvatar loads the bytes for a stored avatar filename. Unknown or
// malformed names and files that vanished from disk are all the same
// not-found: the name space is random, enumeration buys nothing.
func (uc *AvatarUsecase) GetAvatar(ctx context.Context, name string) ([]byte, string, error) {
	if !avatarNamePattern.MatchString(name) {
		return nil, "", ErrUserNotFound
	}
	data, contentType, err := uc.store.Load(ctx, name)
	if err != nil {
		return nil, "", ErrUserNotFound
	}
	return data, contentType, nil
}

// sniffImageExt identifies the accepted image formats by magic bytes: jpeg,
// png, and webp. The declared content type of the upload is ignored — the
// first bytes are the only truth about what was actually sent.
func sniffImageExt(data []byte) (string, bool) {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "jpg", true
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "png", true
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "webp", true
	default:
		return "", false
	}
}
