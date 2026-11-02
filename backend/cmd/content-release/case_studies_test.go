package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func caseStudiesFixture(t *testing.T) []byte {
	t.Helper()
	b := bundle{Version: 1, Status: "prepared_not_applied", Target: "https://makerhao.cn"}
	for _, slug := range []string{"article-cas-revisions-case-study", "article-search-sync-case-study"} {
		fields, _ := json.Marshal(map[string]any{"slug": slug, "title": "深入案例", "summary": "可复现的实现边界", "content_md": "## 场景\n\n真实代码案例", "tags": []string{"Go"}, "status": 2})
		b.Operations = append(b.Operations, operation{Kind: "create_article", Slug: slug, CheckSlug: true, Fields: fields, Mask: append([]string(nil), articlePaths...)})
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestCaseStudiesModeBounds(t *testing.T) {
	raw := caseStudiesFixture(t)
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	if _, err := loadCaseStudies(raw, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCaseStudies(raw, hash, true); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "wrong"} {
		if _, err := loadCaseStudies(raw, bad, true); err == nil {
			t.Fatal("bad apply hash accepted")
		}
	}
	if _, err := loadBundle(raw, "", false); err == nil {
		t.Fatal("legacy loader accepted case mode")
	}
	if _, err := loadRevision(raw, "", false); err == nil {
		t.Fatal("revision loader accepted case mode")
	}
	for name, change := range map[string]func(*bundle){
		"target":          func(b *bundle) { b.Target = "https://other.example" },
		"version":         func(b *bundle) { b.Version = 2 },
		"extra operation": func(b *bundle) { b.Operations = append(b.Operations, b.Operations[0]) },
		"count":           func(b *bundle) { b.Operations = b.Operations[:1] },
		"duplicate":       func(b *bundle) { b.Operations[1] = b.Operations[0] },
		"update":          func(b *bundle) { b.Operations[0].Kind = "update_article" },
		"other slug":      func(b *bundle) { b.Operations[0].Slug = "existing" },
		"slug guard":      func(b *bundle) { b.Operations[0].CheckSlug = false },
		"timestamp":       func(b *bundle) { b.Operations[0].Expected = "2026-11-02T00:00:00Z" },
		"extra mask":      func(b *bundle) { b.Operations[0].Mask = append(b.Operations[0].Mask, "cover_url") },
		"preserve":        func(b *bundle) { b.Operations[0].Preserve = []string{"id"} },
		"fields": func(b *bundle) {
			b.Operations[0].Fields = json.RawMessage(`{"slug":"article-cas-revisions-case-study","title":"x","summary":"s","content_md":"body","tags":[],"status":1}`)
		},
		"empty body": func(b *bundle) {
			b.Operations[0].Fields = json.RawMessage(`{"slug":"article-cas-revisions-case-study","title":"x","summary":"s","content_md":"  ","tags":[],"status":2}`)
		},
		"extra field": func(b *bundle) {
			b.Operations[0].Fields = json.RawMessage(`{"slug":"article-cas-revisions-case-study","title":"x","summary":"s","content_md":"body","tags":[],"status":2,"cover_url":""}`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var b bundle
			_ = json.Unmarshal(raw, &b)
			change(&b)
			r, _ := json.Marshal(b)
			if _, err := loadCaseStudies(r, "", false); err == nil {
				t.Fatal("unsafe case manifest accepted")
			}
		})
	}
	unknown := append([]byte(`{"unexpected":true,`), raw[1:]...)
	if _, err := loadCaseStudies(unknown, "", false); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := loadCaseStudies(append(raw, []byte(` {}`)...), "", false); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}

func TestCaseStudiesModeSelection(t *testing.T) {
	raw := caseStudiesFixture(t)
	if _, err := loadReleaseBundle(raw, "", false, false, true); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][2]bool{{true, true}, {true, false}, {false, false}} {
		if _, err := loadReleaseBundle(raw, "", false, flags[0], flags[1]); err == nil {
			t.Fatal("case manifest accepted by wrong mode", flags)
		}
	}
	if _, err := loadReleaseBundle(releaseFixture(t), "", false, false, true); err == nil {
		t.Fatal("creator manifest entered case mode")
	}
}

func TestProposedCaseStudiesBundle(t *testing.T) {
	raw, err := os.ReadFile("../../../content/releases/blog-case-studies-20261102.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = loadCaseStudies(raw, "", false); err != nil {
		t.Fatal(err)
	}
}

func TestCasePreviewProcess(t *testing.T) {
	if os.Getenv("CONTENT_RELEASE_PREVIEW_HELPER") != "1" {
		return
	}
	index := 0
	for i, arg := range os.Args {
		if arg == "--" {
			index = i + 1
			break
		}
	}
	if index == 0 {
		os.Exit(4)
	}
	os.Args = append([]string{"content-release"}, os.Args[index:]...)
	flag.CommandLine = flag.NewFlagSet("content-release", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	if run() != nil {
		os.Exit(3)
	}
	os.Exit(0)
}
func TestCasePreviewSkipsConfigurationAndPreservesRendererSafety(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cases.json")
	var b bundle
	if err := json.Unmarshal(caseStudiesFixture(t), &b); err != nil {
		t.Fatal(err)
	}
	for i := range b.Operations {
		var fields map[string]any
		_ = json.Unmarshal(b.Operations[i].Fields, &fields)
		fields["content_md"] = "## 示例\n\n<script>alert(1)</script>\n\n[unsafe](javascript:alert(1))"
		b.Operations[i].Fields, _ = json.Marshal(fields)
	}
	raw, _ := json.Marshal(b)
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	invoke := func(extra ...string) ([]byte, error) {
		args := append([]string{"-test.run=^TestCasePreviewProcess$", "--", "-bundle", input, "-conf", filepath.Join(dir, "missing-config")}, extra...)
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "CONTENT_RELEASE_PREVIEW_HELPER=1")
		return cmd.Output()
	}
	output, err := invoke("-case-studies", "-render-preview")
	if err != nil {
		t.Fatal(err)
	}
	var html map[string]string
	if json.Unmarshal(output, &html) != nil || len(html) != 2 {
		t.Fatalf("bad preview JSON: %s", output)
	}
	for slug, body := range html {
		if !strings.Contains(body, "<h2") || strings.Contains(body, "<script") || strings.Contains(body, "javascript:") {
			t.Fatalf("unsafe preview %s: %s", slug, body)
		}
	}
	backup := filepath.Join(dir, "must-not-exist.json")
	for _, args := range [][]string{{"-render-preview"}, {"-case-studies", "-render-preview", "-revision"}, {"-case-studies", "-render-preview", "-backup", backup}, {"-case-studies", "-render-preview", "-apply", "-backup", backup}} {
		if _, err := invoke(args...); err == nil {
			t.Fatal("unsafe preview mode accepted", args)
		}
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatal("preview created backup")
	}
}
