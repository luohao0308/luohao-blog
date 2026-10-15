package data

import (
	"context"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/project"
)

type projectRepo struct{ data *Data }

func NewProjectRepo(data *Data) biz.ProjectRepo { return &projectRepo{data: data} }

func toBizProject(po *ent.Project) *biz.Project {
	if po == nil {
		return nil
	}
	return &biz.Project{
		ID: po.ID, Slug: po.Slug, Name: po.Name, Summary: po.Summary, Description: po.Description,
		Stack: append([]string(nil), po.Stack...), Role: po.Role, RepoURL: po.RepoURL, LiveURL: po.LiveURL,
		Emoji: po.Emoji, Year: po.Year, Featured: po.Featured, CoverURL: po.CoverURL,
		Screenshots: append([]string(nil), po.Screenshots...), Problem: po.Problem, Approach: po.Approach,
		Outcome: po.Outcome, Published: po.Published, Sort: po.Sort, CreatedAt: po.CreatedAt, UpdatedAt: po.UpdatedAt,
	}
}

func (r *projectRepo) FindBySlug(ctx context.Context, slug string) (*biz.Project, error) {
	po, err := r.data.db.Project.Query().Where(project.SlugEQ(slug)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrProjectNotFound
		}
		return nil, err
	}
	return toBizProject(po), nil
}

func (r *projectRepo) ListProjects(ctx context.Context, options biz.ProjectListOptions) ([]*biz.Project, error) {
	query := r.data.db.Project.Query().Order(project.BySort(), project.BySlug())
	if options.Published != nil {
		query = query.Where(project.PublishedEQ(*options.Published))
	}
	pos, err := query.Offset(options.Offset).Limit(options.Limit).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*biz.Project, 0, len(pos))
	for _, po := range pos {
		out = append(out, toBizProject(po))
	}
	return out, nil
}

func projectCreate(client *ent.Client, p *biz.Project) *ent.ProjectCreate {
	return client.Project.Create().
		SetSlug(p.Slug).SetName(p.Name).SetSummary(p.Summary).SetDescription(p.Description).
		SetStack(append([]string(nil), p.Stack...)).SetRole(p.Role).SetRepoURL(p.RepoURL).SetLiveURL(p.LiveURL).
		SetEmoji(p.Emoji).SetYear(p.Year).SetFeatured(p.Featured).SetCoverURL(p.CoverURL).
		SetScreenshots(append([]string(nil), p.Screenshots...)).SetProblem(p.Problem).SetApproach(p.Approach).
		SetOutcome(p.Outcome).SetPublished(p.Published).SetSort(p.Sort)
}

func (r *projectRepo) CreateProject(ctx context.Context, p *biz.Project) (*biz.Project, error) {
	po, err := projectCreate(r.data.db, p).Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, biz.ErrProjectSlugConflict
		}
		return nil, err
	}
	return toBizProject(po), nil
}

func (r *projectRepo) UpdateProject(ctx context.Context, p *biz.Project) (*biz.Project, error) {
	current, err := r.data.db.Project.Query().Where(project.SlugEQ(p.Slug)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrProjectNotFound
		}
		return nil, err
	}
	po, err := r.data.db.Project.UpdateOneID(current.ID).
		SetName(p.Name).SetSummary(p.Summary).SetDescription(p.Description).
		SetStack(append([]string(nil), p.Stack...)).SetRole(p.Role).SetRepoURL(p.RepoURL).SetLiveURL(p.LiveURL).
		SetEmoji(p.Emoji).SetYear(p.Year).SetFeatured(p.Featured).SetCoverURL(p.CoverURL).
		SetScreenshots(append([]string(nil), p.Screenshots...)).SetProblem(p.Problem).SetApproach(p.Approach).
		SetOutcome(p.Outcome).SetPublished(p.Published).SetSort(p.Sort).Save(ctx)
	if err != nil {
		return nil, err
	}
	return toBizProject(po), nil
}

func (r *projectRepo) ArchiveProject(ctx context.Context, slug string) error {
	affected, err := r.data.db.Project.Update().Where(project.SlugEQ(slug)).SetPublished(false).Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return biz.ErrProjectNotFound
	}
	return nil
}
