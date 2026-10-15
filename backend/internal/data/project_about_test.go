package data

import (
	"context"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

func TestProjectRepoCRUDOrderAndPublishedBoundary(t *testing.T) {
	ctx := context.Background()
	_, _, data := newTestRepo(t)
	repo := NewProjectRepo(data)
	for _, p := range []*biz.Project{
		{Slug: "later", Name: "Later", Stack: []string{}, Screenshots: []string{}, Published: true, Sort: 20},
		{Slug: "first", Name: "First", Stack: []string{"Go"}, Screenshots: []string{}, Published: true, Sort: 10},
		{Slug: "draft", Name: "Draft", Stack: []string{}, Screenshots: []string{}, Published: false, Sort: 1},
	} {
		if _, err := repo.CreateProject(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.CreateProject(ctx, &biz.Project{Slug: "first", Name: "Duplicate", Stack: []string{}, Screenshots: []string{}}); err != biz.ErrProjectSlugConflict {
		t.Fatalf("duplicate create = %v, want conflict", err)
	}
	published := true
	items, err := repo.ListProjects(ctx, biz.ProjectListOptions{Limit: 10, Published: &published})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Slug != "first" || items[1].Slug != "later" {
		t.Fatalf("published order = %+v", items)
	}
	first, err := repo.FindBySlug(ctx, "first")
	if err != nil || len(first.Stack) != 1 {
		t.Fatalf("find = %+v, %v", first, err)
	}
	first.Name, first.Published = "Updated", false
	if _, err = repo.UpdateProject(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err = repo.ArchiveProject(ctx, "later"); err != nil {
		t.Fatal(err)
	}
	if err = repo.ArchiveProject(ctx, "missing"); err != biz.ErrProjectNotFound {
		t.Fatalf("archive missing = %v", err)
	}
	items, err = repo.ListProjects(ctx, biz.ProjectListOptions{Limit: 10, Published: &published})
	if err != nil || len(items) != 0 {
		t.Fatalf("after archive = %+v, %v", items, err)
	}
}

func TestAboutRepoSingletonRoundTrip(t *testing.T) {
	ctx := context.Background()
	_, _, data := newTestRepo(t)
	repo := NewAboutRepo(data)
	if _, err := repo.GetAbout(ctx); err != biz.ErrAboutNotFound {
		t.Fatalf("empty get = %v", err)
	}
	want := &biz.About{
		Headline: "headline", Skills: []biz.AboutSkillGroup{{Group: "后端", Items: []string{"Go"}}},
		Timeline:            []biz.AboutTimelineEntry{{Year: "2026", Text: "text"}},
		Contacts:            []biz.AboutContact{{Label: "GitHub", Href: "https://github.com/example", Text: "example"}},
		RepresentativeLinks: []biz.AboutLink{{Label: "Repo", Href: "https://github.com/example/repo"}},
	}
	created, err := repo.UpdateAbout(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	want.ID = created.ID
	want.Headline = "updated"
	updated, err := repo.UpdateAbout(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.Headline != "updated" || len(updated.Skills) != 1 || len(updated.Contacts) != 1 {
		t.Fatalf("updated = %+v", updated)
	}
}
