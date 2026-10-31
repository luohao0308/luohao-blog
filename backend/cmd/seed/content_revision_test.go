package main

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz/article/render"
)

type contentRevision struct {
	Slug              string                     `json:"slug"`
	ID                string                     `json:"id"`
	ExpectedUpdatedAt string                     `json:"expectedUpdatedAt"`
	Before            map[string]json.RawMessage `json:"before"`
	After             map[string]json.RawMessage `json:"after"`
	UpdateMask        []string                   `json:"updateMask"`
}

// Check the review artifact without calling CMS or modifying publication state.
func TestContentRevisionProposal(t *testing.T) {
	data, err := os.ReadFile("../../../docs/content/portfolio-revision-2026-10.json")
	if err != nil {
		t.Fatal(err)
	}
	var proposal struct {
		BaseSHA  string            `json:"baseSha"`
		Projects []contentRevision `json:"projects"`
		Articles []contentRevision `json:"articles"`
	}
	if err := json.Unmarshal(data, &proposal); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(proposal.BaseSHA) {
		t.Fatal("missing immutable source baseline")
	}
	protected := map[string]bool{"id": true, "slug": true, "status": true, "published": true, "published_at": true, "cover_url": true, "screenshots": true, "repo_url": true, "live_url": true, "content_html": true}
	for _, entry := range append(proposal.Projects, proposal.Articles...) {
		if entry.ID == "" || entry.Slug == "" || entry.ExpectedUpdatedAt == "" {
			t.Fatal("missing CMS version precondition")
		}
		if len(entry.After) == 0 || len(entry.Before) != len(entry.After) || len(entry.After) != len(entry.UpdateMask) {
			t.Fatalf("%s: incomplete field mask", entry.Slug)
		}
		seen := map[string]bool{}
		for _, key := range entry.UpdateMask {
			before, beforeOK := entry.Before[key]
			after, afterOK := entry.After[key]
			if seen[key] || protected[key] || !beforeOK || !afterOK || reflect.DeepEqual(before, after) {
				t.Fatalf("%s: invalid update field %s", entry.Slug, key)
			}
			seen[key] = true
		}
	}
	rendered := map[string]string{}
	for _, entry := range proposal.Articles {
		var md string
		if err := json.Unmarshal(entry.After["content_md"], &md); err != nil {
			t.Fatal(err)
		}
		html, err := render.HTML(md)
		if err != nil {
			t.Fatal(err)
		}
		rendered[entry.Slug] = html
	}
	if destination := os.Getenv("BLOG_REVISION_RENDERED"); destination != "" {
		output, err := json.MarshalIndent(rendered, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, output, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
