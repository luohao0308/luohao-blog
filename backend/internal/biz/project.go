package biz

import (
	"context"
	"encoding/json"
	"net/mail"
	"net/url"
	"strings"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

const (
	maxProjectNameBytes  = 128
	maxProjectRoleBytes  = 128
	maxProjectURLBytes   = 512
	maxProjectShortBytes = 32
	maxProjectJSONBytes  = 8 << 10
	maxAboutJSONBytes    = 128 << 10
)

var (
	ErrProjectNotFound        = errors.NotFound(v1.ErrorReason_PROJECT_NOT_FOUND.String(), "project not found")
	ErrProjectInvalidArgument = errors.BadRequest(v1.ErrorReason_PROJECT_INVALID_ARGUMENT.String(), "invalid project argument")
	ErrProjectSlugConflict    = errors.Conflict(v1.ErrorReason_PROJECT_SLUG_CONFLICT.String(), "project slug conflict")
	ErrAboutNotFound          = errors.NotFound(v1.ErrorReason_ABOUT_NOT_FOUND.String(), "about page not found")
	ErrAboutInvalidArgument   = errors.BadRequest(v1.ErrorReason_ABOUT_INVALID_ARGUMENT.String(), "invalid about argument")
)

type Project struct {
	ID                                  uuid.UUID
	Slug, Name, Summary, Description    string
	Stack                               []string
	Role, RepoURL, LiveURL, Emoji, Year string
	Featured                            bool
	CoverURL                            string
	Screenshots                         []string
	Problem, Approach, Outcome          string
	Published                           bool
	Sort                                int32
	CreatedAt, UpdatedAt                time.Time
}

type ProjectListOptions struct {
	Offset, Limit int
	Published     *bool
}

type ProjectRepo interface {
	FindBySlug(context.Context, string) (*Project, error)
	ListProjects(context.Context, ProjectListOptions) ([]*Project, error)
	CreateProject(context.Context, *Project) (*Project, error)
	UpdateProject(context.Context, *Project) (*Project, error)
	ArchiveProject(context.Context, string) error
}

type ProjectUsecase struct{ repo ProjectRepo }

func NewProjectUsecase(repo ProjectRepo) *ProjectUsecase { return &ProjectUsecase{repo: repo} }

func (uc *ProjectUsecase) Create(ctx context.Context, p *Project) (*Project, error) {
	if err := validateProject(p); err != nil {
		return nil, err
	}
	return uc.repo.CreateProject(ctx, p)
}
func (uc *ProjectUsecase) Get(ctx context.Context, slug string) (*Project, error) {
	if !ValidSlug(slug) {
		return nil, ErrProjectInvalidArgument
	}
	return uc.repo.FindBySlug(ctx, slug)
}
func (uc *ProjectUsecase) GetPublic(ctx context.Context, slug string) (*Project, error) {
	p, err := uc.Get(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !p.Published {
		return nil, ErrProjectNotFound
	}
	return p, nil
}
func (uc *ProjectUsecase) List(ctx context.Context, options ProjectListOptions) ([]*Project, error) {
	if options.Offset < 0 || options.Limit <= 0 {
		return nil, ErrProjectInvalidArgument
	}
	return uc.repo.ListProjects(ctx, options)
}
func (uc *ProjectUsecase) Update(ctx context.Context, p *Project) (*Project, error) {
	if err := validateProject(p); err != nil {
		return nil, err
	}
	return uc.repo.UpdateProject(ctx, p)
}
func (uc *ProjectUsecase) Archive(ctx context.Context, slug string) error {
	if !ValidSlug(slug) {
		return ErrProjectInvalidArgument
	}
	return uc.repo.ArchiveProject(ctx, slug)
}

func validateProject(p *Project) error {
	if p == nil || !ValidSlug(p.Slug) || strings.TrimSpace(p.Name) == "" || len(p.Name) > maxProjectNameBytes || p.Sort < 0 {
		return ErrProjectInvalidArgument
	}
	if len(p.Role) > maxProjectRoleBytes || len(p.RepoURL) > maxProjectURLBytes || len(p.LiveURL) > maxProjectURLBytes || len(p.Emoji) > maxProjectShortBytes || len(p.Year) > maxProjectShortBytes {
		return ErrProjectInvalidArgument
	}
	if len([]rune(p.Summary)) > 2048 || len([]rune(p.Description)) > 50000 || len([]rune(p.Problem)) > 50000 || len([]rune(p.Approach)) > 50000 || len([]rune(p.Outcome)) > 50000 {
		return ErrProjectInvalidArgument
	}
	if !ValidMediaURL(p.CoverURL) {
		return ErrProjectInvalidArgument
	}
	for _, image := range p.Screenshots {
		if !ValidMediaURL(image) || image == "" {
			return ErrProjectInvalidArgument
		}
	}
	for _, link := range []string{p.RepoURL, p.LiveURL} {
		if link != "" && !validHTTPURL(link) {
			return ErrProjectInvalidArgument
		}
	}
	if len(p.Stack) > 64 || len(p.Screenshots) > 32 {
		return ErrProjectInvalidArgument
	}
	for _, item := range p.Stack {
		if strings.TrimSpace(item) == "" || len([]rune(item)) > 64 {
			return ErrProjectInvalidArgument
		}
	}
	if !jsonWithinLimit(struct {
		Stack       []string `json:"stack"`
		Screenshots []string `json:"screenshots"`
	}{p.Stack, p.Screenshots}, maxProjectJSONBytes) {
		return ErrProjectInvalidArgument
	}
	return nil
}

func validHTTPURL(raw string) bool {
	if hasUnsafeURLBytes(raw) {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}

func hasUnsafeURLBytes(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.ContainsAny(raw, "\r\n") || strings.Contains(lower, "%0d") || strings.Contains(lower, "%0a")
}

type AboutSkillGroup struct {
	Group string   `json:"group"`
	Items []string `json:"items"`
}
type AboutTimelineEntry struct {
	Year string `json:"year"`
	Text string `json:"text"`
}
type AboutContact struct {
	Label string `json:"label"`
	Href  string `json:"href"`
	Text  string `json:"text"`
}
type AboutLink struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

type About struct {
	ID                                                uuid.UUID
	Name, Headline, Intro, ProfileImage, CurrentFocus string
	Skills                                            []AboutSkillGroup
	Timeline                                          []AboutTimelineEntry
	Contacts                                          []AboutContact
	RepresentativeLinks                               []AboutLink
	UpdatedAt                                         time.Time
}

type AboutRepo interface {
	GetAbout(context.Context) (*About, error)
	UpdateAbout(context.Context, *About) (*About, error)
}
type AboutUsecase struct{ repo AboutRepo }

func NewAboutUsecase(repo AboutRepo) *AboutUsecase               { return &AboutUsecase{repo: repo} }
func (uc *AboutUsecase) Get(ctx context.Context) (*About, error) { return uc.repo.GetAbout(ctx) }
func (uc *AboutUsecase) Update(ctx context.Context, about *About) (*About, error) {
	if err := ValidateAbout(about); err != nil {
		return nil, err
	}
	return uc.repo.UpdateAbout(ctx, about)
}

func ValidateAbout(a *About) error {
	if a == nil || len(a.Name) > 128 || !ValidMediaURL(a.ProfileImage) || len(a.Skills) > 32 || len(a.Timeline) > 64 || len(a.Contacts) > 32 || len(a.RepresentativeLinks) > 32 {
		return ErrAboutInvalidArgument
	}
	if len([]rune(a.Headline)) > 1000 || len([]rune(a.Intro)) > 50000 || len([]rune(a.CurrentFocus)) > 50000 {
		return ErrAboutInvalidArgument
	}
	for _, group := range a.Skills {
		if strings.TrimSpace(group.Group) == "" || len([]rune(group.Group)) > 64 || len(group.Items) > 64 {
			return ErrAboutInvalidArgument
		}
		for _, item := range group.Items {
			if strings.TrimSpace(item) == "" || len([]rune(item)) > 64 {
				return ErrAboutInvalidArgument
			}
		}
	}
	for _, item := range a.Timeline {
		if strings.TrimSpace(item.Year) == "" || len([]rune(item.Year)) > 32 || strings.TrimSpace(item.Text) == "" || len([]rune(item.Text)) > 2000 {
			return ErrAboutInvalidArgument
		}
	}
	for _, contact := range a.Contacts {
		if strings.TrimSpace(contact.Label) == "" || len([]rune(contact.Label)) > 64 || strings.TrimSpace(contact.Text) == "" || len([]rune(contact.Text)) > 256 || len(contact.Href) > maxProjectURLBytes || !validContactURL(contact.Href) {
			return ErrAboutInvalidArgument
		}
	}
	for _, link := range a.RepresentativeLinks {
		if strings.TrimSpace(link.Label) == "" || len([]rune(link.Label)) > 128 || len(link.Href) > maxProjectURLBytes || !validHTTPURL(link.Href) {
			return ErrAboutInvalidArgument
		}
	}
	if !jsonWithinLimit(struct {
		Skills   []AboutSkillGroup    `json:"skills"`
		Timeline []AboutTimelineEntry `json:"timeline"`
		Contacts []AboutContact       `json:"contacts"`
		Links    []AboutLink          `json:"representative_links"`
	}{a.Skills, a.Timeline, a.Contacts, a.RepresentativeLinks}, maxAboutJSONBytes) {
		return ErrAboutInvalidArgument
	}
	return nil
}

func jsonWithinLimit(value any, limit int) bool {
	data, err := json.Marshal(value)
	return err == nil && len(data) <= limit
}

func validContactURL(raw string) bool {
	if validHTTPURL(raw) {
		return true
	}
	if hasUnsafeURLBytes(raw) {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "mailto" || u.Opaque == "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	address, err := mail.ParseAddress(u.Opaque)
	return err == nil && address.Address == u.Opaque
}
