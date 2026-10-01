package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

// The demo articles live as plain markdown files next to this package and are
// compiled in, so the seed command stays self-contained. Each file carries a
// minimal frontmatter block followed by the content_md body; content_html is
// never stored here because the usecase re-renders it on every write.
//
//	---
//	slug: hello-world
//	title: Hello
//	summary:
//	tags: go, kratos
//	status: published
//	published_at: 2026-09-30T18:06:46+08:00
//	---
//	markdown body
//
//go:embed demoarticles/*.md
var demoArticleFS embed.FS

// demoArticle is one seeded article parsed from its markdown file.
type demoArticle struct {
	Slug        string
	Title       string
	Summary     string
	Tags        []string
	Status      biz.ArticleStatus
	PublishedAt *time.Time
	ContentMD   string
}

// loadDemoArticles reads every embedded markdown file in name order.
func loadDemoArticles() ([]*demoArticle, error) {
	entries, err := fs.ReadDir(demoArticleFS, "demoarticles")
	if err != nil {
		return nil, err
	}
	articles := make([]*demoArticle, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := demoArticleFS.ReadFile("demoarticles/" + entry.Name())
		if err != nil {
			return nil, err
		}
		a, err := parseDemoArticle(entry.Name(), data)
		if err != nil {
			return nil, err
		}
		articles = append(articles, a)
	}
	return articles, nil
}

// parseDemoArticle splits the frontmatter block from the body. Exactly one
// trailing newline is stripped from the body: the file format adds it, the
// stored content_md has none.
func parseDemoArticle(name string, data []byte) (*demoArticle, error) {
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return nil, fmt.Errorf("seed: %s: frontmatter must start with ---", name)
	}
	closing := strings.Index(text, "\n---\n")
	if closing < 0 {
		return nil, fmt.Errorf("seed: %s: frontmatter is not closed with ---", name)
	}
	a := &demoArticle{
		ContentMD: strings.TrimSuffix(text[closing+len("\n---\n"):], "\n"),
	}
	fields := map[string]string{}
	for i, line := range strings.Split(text[len("---\n"):closing], "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			return nil, fmt.Errorf("seed: %s: frontmatter line %d is not \"key: value\"", name, i+1)
		}
		fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	a.Slug = fields["slug"]
	a.Title = fields["title"]
	a.Summary = fields["summary"]
	for _, tag := range strings.Split(fields["tags"], ",") {
		if tag = strings.TrimSpace(tag); tag != "" {
			a.Tags = append(a.Tags, tag)
		}
	}
	switch fields["status"] {
	case "draft":
		a.Status = biz.ArticleStatusDraft
	case "published":
		a.Status = biz.ArticleStatusPublished
	default:
		return nil, fmt.Errorf("seed: %s: status must be draft or published, got %q", name, fields["status"])
	}
	if fields["published_at"] != "" {
		if a.Status != biz.ArticleStatusPublished {
			return nil, fmt.Errorf("seed: %s: published_at is only valid on published articles", name)
		}
		at, err := time.Parse(time.RFC3339, fields["published_at"])
		if err != nil {
			return nil, fmt.Errorf("seed: %s: parse published_at: %w", name, err)
		}
		a.PublishedAt = &at
	} else if a.Status == biz.ArticleStatusPublished {
		return nil, fmt.Errorf("seed: %s: published articles need published_at", name)
	}
	return a, nil
}

// toBizArticle maps the seed record onto the domain write model.
func (a *demoArticle) toBizArticle() *biz.Article {
	return &biz.Article{
		Slug:        a.Slug,
		Title:       a.Title,
		Summary:     a.Summary,
		ContentMD:   a.ContentMD,
		Tags:        a.Tags,
		Status:      a.Status,
		PublishedAt: a.PublishedAt,
	}
}

// seedDemoArticles brings the demo articles into the database. Missing
// articles are created; existing ones are skipped unless reset is set, so
// re-running the seed never clobbers edits made through the admin panel.
func seedDemoArticles(ctx context.Context, uc *biz.ArticleUsecase, reset bool) error {
	articles, err := loadDemoArticles()
	if err != nil {
		return err
	}
	for _, a := range articles {
		var verb string
		_, err := uc.GetArticle(ctx, a.Slug)
		switch {
		case err == nil && !reset:
			fmt.Printf("seed: article %s exists, skipped (use -reset to overwrite)\n", a.Slug)
			continue
		case err == nil:
			verb = "reset"
			_, err = uc.UpdateArticle(ctx, a.toBizArticle())
		case biz.ErrArticleNotFound.Is(err):
			verb = "created"
			err = createDemoArticle(ctx, uc, a)
		}
		if err != nil {
			return fmt.Errorf("seed: article %s: %w", a.Slug, err)
		}
		fmt.Printf("seed: %s article %s\n", verb, a.Slug)
	}
	return nil
}

// createDemoArticle inserts one article. CreateArticle always forces DRAFT,
// so a published article is created first and published in a second write
// that carries the canonical published_at stamp.
func createDemoArticle(ctx context.Context, uc *biz.ArticleUsecase, a *demoArticle) error {
	if _, err := uc.CreateArticle(ctx, &biz.Article{
		Slug:      a.Slug,
		Title:     a.Title,
		Summary:   a.Summary,
		ContentMD: a.ContentMD,
		Tags:      a.Tags,
	}); err != nil {
		return err
	}
	if a.Status == biz.ArticleStatusPublished {
		_, err := uc.UpdateArticle(ctx, a.toBizArticle())
		return err
	}
	return nil
}
