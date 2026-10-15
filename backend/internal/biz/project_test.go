package biz

import (
	"context"
	"strings"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

type fakeProjectRepo struct {
	items    map[string]*Project
	archived string
}

func newFakeProjectRepo() *fakeProjectRepo { return &fakeProjectRepo{items: map[string]*Project{}} }
func (r *fakeProjectRepo) FindBySlug(_ context.Context, slug string) (*Project, error) {
	p, ok := r.items[slug]
	if !ok {
		return nil, ErrProjectNotFound
	}
	copy := *p
	return &copy, nil
}
func (r *fakeProjectRepo) ListProjects(_ context.Context, options ProjectListOptions) ([]*Project, error) {
	out := []*Project{}
	for _, p := range r.items {
		if options.Published != nil && p.Published != *options.Published {
			continue
		}
		copy := *p
		out = append(out, &copy)
	}
	return out, nil
}
func (r *fakeProjectRepo) CreateProject(_ context.Context, p *Project) (*Project, error) {
	r.items[p.Slug] = p
	return p, nil
}
func (r *fakeProjectRepo) UpdateProject(_ context.Context, p *Project) (*Project, error) {
	r.items[p.Slug] = p
	return p, nil
}
func (r *fakeProjectRepo) ArchiveProject(_ context.Context, slug string) error {
	r.archived = slug
	r.items[slug].Published = false
	return nil
}

func validProject() *Project {
	return &Project{Slug: "demo", Name: "Demo", Summary: "summary", Description: "description", RepoURL: "https://github.com/example/demo", CoverURL: "/v1/assets/media/0123456789abcdef0123456789abcdef.png", Screenshots: []string{"/v1/assets/media/abcdef0123456789abcdef0123456789.webp"}}
}

func TestProjectUsecaseValidationAndPublicBoundary(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProjectRepo()
	uc := NewProjectUsecase(repo)
	for name, mutate := range map[string]func(*Project){
		"bad slug":       func(p *Project) { p.Slug = "BAD" },
		"external cover": func(p *Project) { p.CoverURL = "https://example.com/a.png" },
		"bad screenshot": func(p *Project) { p.Screenshots = []string{"javascript:alert(1)"} },
		"bad repo":       func(p *Project) { p.RepoURL = "javascript:alert(1)" },
		"bad live":       func(p *Project) { p.LiveURL = "data:text/html,x" },
	} {
		t.Run(name, func(t *testing.T) {
			p := validProject()
			mutate(p)
			if _, err := uc.Create(ctx, p); !kratoserrors.IsBadRequest(err) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	draft := validProject()
	if _, err := uc.Create(ctx, draft); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.GetPublic(ctx, draft.Slug); !kratoserrors.IsNotFound(err) {
		t.Fatalf("draft public get = %v", err)
	}
	draft.Published = true
	if _, err := uc.Update(ctx, draft); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.GetPublic(ctx, draft.Slug); err != nil {
		t.Fatal(err)
	}
	if err := uc.Archive(ctx, draft.Slug); err != nil || repo.archived != draft.Slug {
		t.Fatalf("archive = %q, %v", repo.archived, err)
	}
}

func TestProjectUsecaseRejectsStorageAndJSONLimits(t *testing.T) {
	uc := NewProjectUsecase(newFakeProjectRepo())
	cases := map[string]func(*Project){
		"multibyte name bytes": func(p *Project) { p.Name = strings.Repeat("界", 43) },
		"role bytes":           func(p *Project) { p.Role = strings.Repeat("界", 43) },
		"repo url bytes":       func(p *Project) { p.RepoURL = "https://example.com/" + strings.Repeat("a", 500) },
		"emoji bytes":          func(p *Project) { p.Emoji = strings.Repeat("界", 11) },
		"year bytes":           func(p *Project) { p.Year = strings.Repeat("y", 33) },
		"stack item":           func(p *Project) { p.Stack = []string{strings.Repeat("x", 65)} },
		"stack json total": func(p *Project) {
			for range 64 {
				p.Stack = append(p.Stack, strings.Repeat("界", 64))
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := validProject()
			mutate(p)
			if _, err := uc.Create(context.Background(), p); !kratoserrors.IsBadRequest(err) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestAboutUsecaseValidation(t *testing.T) {
	valid := &About{
		Headline: "全栈工程师", ProfileImage: "/v1/assets/media/0123456789abcdef0123456789abcdef.jpg",
		Contacts:            []AboutContact{{Label: "邮箱", Href: "mailto:test@example.com", Text: "test@example.com"}},
		RepresentativeLinks: []AboutLink{{Label: "GitHub", Href: "https://github.com/example"}},
	}
	for name, mutate := range map[string]func(*About){
		"bad image":          func(a *About) { a.ProfileImage = "https://example.com/me.jpg" },
		"javascript contact": func(a *About) { a.Contacts[0].Href = "javascript:alert(1)" },
		"crlf mail":          func(a *About) { a.Contacts[0].Href = "mailto:test@example.com%0d%0aBcc:x@y.com" },
		"invalid mail":       func(a *About) { a.Contacts[0].Href = "mailto:not-an-email" },
		"data link":          func(a *About) { a.RepresentativeLinks[0].Href = "data:text/html,x" },
	} {
		t.Run(name, func(t *testing.T) {
			a := *valid
			a.Contacts = append([]AboutContact(nil), valid.Contacts...)
			a.RepresentativeLinks = append([]AboutLink(nil), valid.RepresentativeLinks...)
			mutate(&a)
			if err := ValidateAbout(&a); !kratoserrors.IsBadRequest(err) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	if err := ValidateAbout(valid); err != nil {
		t.Fatalf("valid about: %v", err)
	}
}

func TestAboutUsecaseRejectsStorageAndJSONLimits(t *testing.T) {
	base := &About{Contacts: []AboutContact{{Label: "GitHub", Href: "https://github.com/example", Text: "example"}}}
	cases := map[string]func(*About){
		"multibyte name bytes": func(a *About) { a.Name = strings.Repeat("界", 43) },
		"skill item": func(a *About) {
			a.Skills = []AboutSkillGroup{{Group: "backend", Items: []string{strings.Repeat("x", 65)}}}
		},
		"timeline item": func(a *About) { a.Timeline = []AboutTimelineEntry{{Year: "2026", Text: strings.Repeat("x", 2001)}} },
		"contact text":  func(a *About) { a.Contacts[0].Text = strings.Repeat("x", 257) },
		"link label": func(a *About) {
			a.RepresentativeLinks = []AboutLink{{Label: strings.Repeat("x", 129), Href: "https://example.com"}}
		},
		"json total": func(a *About) {
			for range 64 {
				a.Timeline = append(a.Timeline, AboutTimelineEntry{Year: "2026", Text: strings.Repeat("界", 2000)})
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			a := *base
			a.Contacts = append([]AboutContact(nil), base.Contacts...)
			mutate(&a)
			if err := ValidateAbout(&a); !kratoserrors.IsBadRequest(err) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
