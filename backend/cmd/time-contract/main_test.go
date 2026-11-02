package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupArtifactsRejectCorruption(t *testing.T) {
	dir := t.TempDir()
	var data bytes.Buffer
	g := gzip.NewWriter(&data)
	_, _ = g.Write([]byte("synthetic SQL"))
	_ = g.Close()
	archive := filepath.Join(dir, "backup.sql.gz")
	sidecar := filepath.Join(dir, "manifest.json")
	raw := data.Bytes()
	if os.WriteFile(archive, raw, 0600) != nil {
		t.Fatal("fixture")
	}
	m := map[string]any{"source_schema_version": 20, "archive_sha256": fmt.Sprintf("%x", sha256.Sum256(raw))}
	b, _ := json.Marshal(m)
	if os.WriteFile(sidecar, b, 0600) != nil {
		t.Fatal("fixture")
	}
	if _, _, e := backupArtifacts(archive, sidecar); e != nil {
		t.Fatal(e)
	}
	if os.WriteFile(archive, raw[:len(raw)-3], 0600) != nil {
		t.Fatal("fixture")
	}
	if _, _, e := backupArtifacts(archive, sidecar); e == nil {
		t.Fatal("truncated backup accepted")
	}
}
func TestReadRejectsTrailingAndUnknownJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "input.json")
	for _, input := range []string{`{"ok":true} {"ok":false}`, `{"unknown":true}`} {
		if os.WriteFile(p, []byte(input), 0600) != nil {
			t.Fatal("fixture")
		}
		var out struct {
			OK bool `json:"ok"`
		}
		if read(p, &out) == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
}
