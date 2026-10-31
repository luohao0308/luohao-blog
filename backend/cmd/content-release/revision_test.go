package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
	"os"
	"testing"
	"time"
)

func revisionFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../../docs/content/portfolio-revision-2026-10.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestRevisionParser(t *testing.T) {
	raw := revisionFixture(t)
	b, err := loadRevision(raw, "", false)
	if err != nil || len(b.Operations) != 6 {
		t.Fatalf("valid revision: %v", err)
	}
	if _, err = loadRevision(raw, "bad", true); err == nil {
		t.Fatal("bad hash accepted")
	}
	for _, mutation := range []string{"id", "before", "mask", "preserved", "count", "slug"} {
		t.Run(mutation, func(t *testing.T) {
			var doc map[string]any
			_ = json.Unmarshal(raw, &doc)
			entries := doc["projects"].([]any)
			e := entries[0].(map[string]any)
			switch mutation {
			case "id":
				e["id"] = ""
			case "before":
				delete(e["before"].(map[string]any), "name")
			case "mask":
				e["updateMask"] = append(e["updateMask"].([]any), "published")
				e["before"].(map[string]any)["published"] = true
				e["after"].(map[string]any)["published"] = false
			case "preserved":
				delete(e["preserved"].(map[string]any), "repo_url")
			case "count":
				doc["projects"] = entries[:3]
			case "slug":
				e["slug"] = "unknown"
			}
			changed, _ := json.Marshal(doc)
			if _, err := loadRevision(changed, "", false); err == nil {
				t.Fatal("unsafe revision accepted")
			}
		})
	}
}
func TestRevisionFieldComparison(t *testing.T) {
	current := &v1.Project{Id: "project", Slug: "aigc", Name: "before", Stack: []string{"Python"}}
	if err := compareRevisionFields(current, json.RawMessage(`{"name":"before","stack":["Python"],"repo_url":"","published":false}`)); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"name":"other"}`, `{"stack":["Go"]}`, `{"unknown":false}`} {
		if err := compareRevisionFields(current, json.RawMessage(raw)); err == nil {
			t.Fatal("changed/unknown field accepted")
		}
	}
}

func TestRevisionCheckRejectsDriftAtSameTimestamp(t *testing.T) {
	stamp := time.Unix(123, 0)
	memory := &projectMemory{value: &biz.Project{ID: uuid.MustParse("00000000-0000-7000-8000-000000000102"), Slug: "aigc", Name: "before", Published: true, UpdatedAt: stamp, RepoURL: "https://example.com/source"}}
	r := &runner{p: service.NewProjectService(biz.NewProjectUsecase(memory))}
	o := operation{Kind: "update_project", Slug: "aigc", Expected: stamp.Format(time.RFC3339), RevisionID: "00000000-0000-7000-8000-000000000102", Before: json.RawMessage(`{"name":"before"}`), Preserved: json.RawMessage(`{"published":true,"repo_url":"https://example.com/source"}`)}
	for _, change := range []string{"none", "id", "before", "preserved", "timestamp"} {
		t.Run(change, func(t *testing.T) {
			candidate := o
			switch change {
			case "id":
				candidate.RevisionID = "different"
			case "before":
				candidate.Before = json.RawMessage(`{"name":"other"}`)
			case "preserved":
				candidate.Preserved = json.RawMessage(`{"repo_url":"https://different.example"}`)
			case "timestamp":
				candidate.Expected = stamp.Add(time.Second).Format(time.RFC3339)
			}
			_, err := r.check(context.Background(), candidate)
			if (err == nil) != (change == "none") {
				t.Fatalf("unexpected precheck: %v", err)
			}
			if memory.writes != 0 {
				t.Fatal("check wrote content")
			}
		})
	}
}
func TestRevisionPostcheckAndLegacyIsolation(t *testing.T) {
	raw := revisionFixture(t)
	sum := sha256.Sum256(raw)
	if _, err := loadRevision(raw, hex.EncodeToString(sum[:]), true); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBundle(raw, "", false); err == nil {
		t.Fatal("revision accepted by legacy parser")
	}
	if _, err := loadRevision(releaseFixture(t), "", false); err == nil {
		t.Fatal("legacy bundle accepted by revision parser")
	}
	current := &v1.Project{Id: "id", Slug: "aigc", Name: "after", RepoUrl: "https://example.com"}
	op := operation{RevisionID: "id", Fields: json.RawMessage(`{"slug":"aigc","name":"after"}`), Preserved: json.RawMessage(`{"repo_url":"https://example.com"}`)}
	if err := checkRevisionResult(current, op); err != nil {
		t.Fatal(err)
	}
	current.RepoUrl = "https://changed.example"
	if err := checkRevisionResult(current, op); err == nil {
		t.Fatal("media/link drift accepted after write")
	}
	current.RepoUrl = "https://example.com"
	current.Name = "not-saved"
	if err := checkRevisionResult(current, op); err == nil {
		t.Fatal("unsaved patch accepted")
	}
}
