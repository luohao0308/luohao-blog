package service

import (
	"context"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type serviceProjectRepo struct {
	items map[string]*biz.Project
	last  biz.ProjectListOptions
}

func newServiceProjectRepo() *serviceProjectRepo {
	return &serviceProjectRepo{items: map[string]*biz.Project{
		"published": {Slug: "published", Name: "Published", Published: true},
		"draft":     {Slug: "draft", Name: "Draft", Published: false},
	}}
}
func (r *serviceProjectRepo) FindBySlug(_ context.Context, slug string) (*biz.Project, error) {
	p, ok := r.items[slug]
	if !ok {
		return nil, biz.ErrProjectNotFound
	}
	copy := *p
	return &copy, nil
}
func (r *serviceProjectRepo) ListProjects(_ context.Context, options biz.ProjectListOptions) ([]*biz.Project, error) {
	r.last = options
	out := []*biz.Project{}
	for _, p := range r.items {
		if options.Published == nil || p.Published == *options.Published {
			copy := *p
			out = append(out, &copy)
		}
	}
	return out, nil
}
func (r *serviceProjectRepo) CreateProject(_ context.Context, p *biz.Project) (*biz.Project, error) {
	r.items[p.Slug] = p
	return p, nil
}
func (r *serviceProjectRepo) UpdateProject(_ context.Context, p *biz.Project) (*biz.Project, error) {
	r.items[p.Slug] = p
	return p, nil
}
func (r *serviceProjectRepo) ArchiveProject(_ context.Context, slug string) error {
	r.items[slug].Published = false
	return nil
}

func TestProjectServicePublicAndAdminVisibility(t *testing.T) {
	repo := newServiceProjectRepo()
	svc := NewProjectService(biz.NewProjectUsecase(repo))
	if _, err := svc.GetProject(context.Background(), &v1.GetProjectRequest{Slug: "draft"}); !kratoserrors.IsNotFound(err) {
		t.Fatalf("public draft get = %v", err)
	}
	if _, err := svc.ListProjects(context.Background(), &v1.ListProjectsRequest{}); err != nil {
		t.Fatal(err)
	}
	if repo.last.Published == nil || !*repo.last.Published {
		t.Fatalf("public list options = %+v", repo.last)
	}
	admin := biz.NewAuthContext(context.Background(), &biz.Claims{UserID: uuid.New(), Role: biz.UserRoleAdmin})
	if got, err := svc.GetProject(admin, &v1.GetProjectRequest{Slug: "draft"}); err != nil || got.GetSlug() != "draft" {
		t.Fatalf("admin draft get = %+v, %v", got, err)
	}
	wantDraft := false
	if _, err := svc.ListProjects(admin, &v1.ListProjectsRequest{Published: &wantDraft}); err != nil {
		t.Fatal(err)
	}
	if repo.last.Published == nil || *repo.last.Published {
		t.Fatalf("admin filter options = %+v", repo.last)
	}
}

func TestProjectServiceFieldMaskPreservesAndClears(t *testing.T) {
	repo := newServiceProjectRepo()
	repo.items["published"].CoverURL = "/v1/assets/media/0123456789abcdef0123456789abcdef.png"
	repo.items["published"].Stack = []string{"Go"}
	repo.items["published"].Screenshots = []string{"/v1/assets/media/abcdef0123456789abcdef0123456789.webp"}
	svc := NewProjectService(biz.NewProjectUsecase(repo))
	_, err := svc.UpdateProject(context.Background(), &v1.UpdateProjectRequest{Project: &v1.Project{Slug: "published", Name: "Renamed"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name"}}})
	if err != nil {
		t.Fatal(err)
	}
	if repo.items["published"].CoverURL == "" || len(repo.items["published"].Stack) != 1 {
		t.Fatalf("mask cleared fields: %+v", repo.items["published"])
	}
	_, err = svc.UpdateProject(context.Background(), &v1.UpdateProjectRequest{Project: &v1.Project{Slug: "published", CoverUrl: ""}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"cover_url"}}})
	if err != nil || repo.items["published"].CoverURL != "" {
		t.Fatalf("clear cover = %+v, %v", repo.items["published"], err)
	}
	replacement := "/v1/assets/media/11111111111111111111111111111111.jpg"
	_, err = svc.UpdateProject(context.Background(), &v1.UpdateProjectRequest{Project: &v1.Project{Slug: "published", Stack: []string{"Rust"}, Screenshots: []string{replacement}}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"stack", "screenshots"}}})
	if err != nil || len(repo.items["published"].Stack) != 1 || repo.items["published"].Stack[0] != "Rust" || len(repo.items["published"].Screenshots) != 1 || repo.items["published"].Screenshots[0] != replacement {
		t.Fatalf("replace collections = %+v, %v", repo.items["published"], err)
	}
	_, err = svc.UpdateProject(context.Background(), &v1.UpdateProjectRequest{Project: &v1.Project{Slug: "published", Stack: []string{}, Screenshots: []string{}}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"stack", "screenshots"}}})
	if err != nil || len(repo.items["published"].Stack) != 0 || len(repo.items["published"].Screenshots) != 0 {
		t.Fatalf("clear collections = %+v, %v", repo.items["published"], err)
	}
}

func TestProjectServiceRejectsSlugUpdateMask(t *testing.T) {
	svc := NewProjectService(biz.NewProjectUsecase(newServiceProjectRepo()))
	_, err := svc.UpdateProject(context.Background(), &v1.UpdateProjectRequest{Project: &v1.Project{Slug: "published", Name: "x"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"slug"}}})
	if !kratoserrors.IsBadRequest(err) {
		t.Fatalf("slug mask error = %v", err)
	}
}

type serviceAboutRepo struct{ about *biz.About }

func (r *serviceAboutRepo) GetAbout(context.Context) (*biz.About, error) {
	if r.about == nil {
		return nil, biz.ErrAboutNotFound
	}
	return r.about, nil
}
func (r *serviceAboutRepo) UpdateAbout(_ context.Context, a *biz.About) (*biz.About, error) {
	r.about = a
	return a, nil
}

func TestAboutServiceRoundTrip(t *testing.T) {
	repo := &serviceAboutRepo{}
	svc := NewAboutService(biz.NewAboutUsecase(repo))
	in := &v1.About{Headline: "headline", Skills: []*v1.AboutSkillGroup{{Group: "后端", Items: []string{"Go"}}}, Contacts: []*v1.AboutContact{{Label: "GitHub", Href: "https://github.com/example", Text: "example"}}}
	out, err := svc.UpdateAbout(context.Background(), &v1.UpdateAboutRequest{About: in})
	if err != nil || out.GetHeadline() != "headline" || len(out.GetSkills()) != 1 || len(out.GetContacts()) != 1 {
		t.Fatalf("UpdateAbout = %+v, %v", out, err)
	}
}
