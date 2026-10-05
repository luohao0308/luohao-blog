package data

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

// defaultUploadsDir is used when the config leaves data.uploads_dir unset.
const defaultUploadsDir = "./data/uploads"

// avatarDirName is the subdirectory of the uploads dir that holds avatars.
const avatarDirName = "avatars"

// avatarContentType maps the stored filename extension onto the Content-Type
// served by GetAvatar. Only extensions the biz layer sniffed out of magic
// bytes can reach here.
var avatarContentType = map[string]string{
	"jpg":  "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
}

// fsAvatarStore keeps avatar images on the server's local disk. Production
// mounts a named Docker volume over the uploads dir so the files survive
// container replacement; losing it degrades to missing avatars, never to
// lost account data.
type fsAvatarStore struct {
	dir string
}

// NewAvatarStore builds the local-disk avatar store under the configured
// uploads directory, creating it on boot when missing.
func NewAvatarStore(c *conf.Data) (biz.AvatarStore, error) {
	dir := c.GetUploadsDir()
	if dir == "" {
		dir = defaultUploadsDir
	}
	avatars := filepath.Join(dir, avatarDirName)
	// 0755: the directory itself carries no secrets; files inside are
	// publicly served bytes anyway.
	if err := os.MkdirAll(avatars, 0o755); err != nil {
		return nil, err
	}
	return &fsAvatarStore{dir: avatars}, nil
}

// Save writes the image under a fresh UUIDv4 filename (32 hex chars, the
// shape the biz layer's URL pattern expects) and returns the bare filename.
func (s *fsAvatarStore) Save(_ context.Context, ext string, data []byte) (string, error) {
	name := strings.ReplaceAll(uuid.NewString(), "-", "") + "." + ext
	// 0644: avatar bytes are world-readable static assets by design.
	if err := os.WriteFile(filepath.Join(s.dir, name), data, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// Load reads a stored avatar back. The biz layer has already matched the
// name against its generator pattern; filepath.Base is defense in depth so a
// separator slipping through can never escape the avatars directory.
func (s *fsAvatarStore) Load(_ context.Context, name string) ([]byte, string, error) {
	clean := filepath.Base(name)
	if clean != name {
		return nil, "", os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(s.dir, clean))
	if err != nil {
		return nil, "", err
	}
	ext := strings.TrimPrefix(filepath.Ext(clean), ".")
	return data, avatarContentType[ext], nil
}
