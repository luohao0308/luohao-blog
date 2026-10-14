package data

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

func TestFSMediaStoreRoundTrip(t *testing.T) {
	uploads := t.TempDir()
	store, err := NewMediaStore(&conf.Data{UploadsDir: uploads})
	if err != nil {
		t.Fatalf("NewMediaStore() error = %v", err)
	}
	data := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	name, err := store.Save(context.Background(), "png", data)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, contentType, err := store.Load(context.Background(), name)
	if err != nil || contentType != "image/png" || string(got) != string(data) {
		t.Fatalf("Load() = (%q, %d, %v)", contentType, len(got), err)
	}
	if _, err := os.Stat(filepath.Join(uploads, mediaDirName, name)); err != nil {
		t.Fatalf("stored file: %v", err)
	}
	if _, _, err := store.Load(context.Background(), "../"+name); !os.IsNotExist(err) {
		t.Fatalf("traversal load error = %v, want not exist", err)
	}
}
