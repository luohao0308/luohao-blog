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

const mediaDirName = "media"

type fsMediaStore struct{ dir string }

func NewMediaStore(c *conf.Data) (biz.MediaStore, error) {
	dir := c.GetUploadsDir()
	if dir == "" {
		dir = defaultUploadsDir
	}
	media := filepath.Join(dir, mediaDirName)
	if err := os.MkdirAll(media, 0o755); err != nil {
		return nil, err
	}
	return &fsMediaStore{dir: media}, nil
}

func (s *fsMediaStore) Save(_ context.Context, ext string, data []byte) (string, error) {
	name := strings.ReplaceAll(uuid.NewString(), "-", "") + "." + ext
	if err := os.WriteFile(filepath.Join(s.dir, name), data, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

func (s *fsMediaStore) Load(_ context.Context, name string) ([]byte, string, error) {
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
