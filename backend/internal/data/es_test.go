package data

import (
	"strings"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

func TestEsIndexSettings(t *testing.T) {
	withVec := esIndexSettings(1024)
	if !strings.Contains(withVec, "dense_vector") || !strings.Contains(withVec, "1024") {
		t.Fatal("expected dense_vector mapping")
	}
	withoutVec := esIndexSettings(0)
	if strings.Contains(withoutVec, "dense_vector") {
		t.Fatal("dense_vector should be absent when dims=0")
	}
}

func TestEmbeddingInput(t *testing.T) {
	a := &biz.Article{Title: "t", Summary: "s", ContentMD: "body"}
	got := embeddingInput(a)
	if !strings.Contains(got, "t") || !strings.Contains(got, "s") || !strings.Contains(got, "body") {
		t.Fatalf("unexpected embedding input: %q", got)
	}
}

func TestEsIndexDimensions(t *testing.T) {
	if esIndexDimensions(nil) != 0 {
		t.Fatal("nil embedder should return 0")
	}
}
