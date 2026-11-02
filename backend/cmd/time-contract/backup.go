package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	tc "github.com/luohao0308/luohao-blog/backend/internal/timecontract"
	"io"
	"os"
)

func backupArtifacts(archive, sidecar string) (string, string, error) {
	f, e := os.Open(archive)
	if e != nil {
		return "", "", tc.ErrContract
	}
	defer func() { _ = f.Close() }()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return "", "", tc.ErrContract
	}
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", "", tc.ErrContract
	}
	ah := fmt.Sprintf("%x", h.Sum(nil))
	if _, e = f.Seek(0, 0); e != nil {
		return "", "", tc.ErrContract
	}
	g, e := gzip.NewReader(f)
	if e != nil {
		return "", "", tc.ErrContract
	}
	_, e = io.Copy(io.Discard, g)
	ce := g.Close()
	if e != nil || ce != nil {
		return "", "", tc.ErrContract
	}
	raw, e := os.ReadFile(sidecar)
	if e != nil {
		return "", "", tc.ErrContract
	}
	var m struct {
		Source  int    `json:"source_schema_version"`
		Archive string `json:"archive_sha256"`
	}
	if json.Unmarshal(raw, &m) != nil || m.Source != 20 || m.Archive != ah {
		return "", "", tc.ErrContract
	}
	return ah, fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}
