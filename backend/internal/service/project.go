package service

import (
	"context"
	"errors"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var allowedProjectUpdateMaskPaths = map[string]struct{}{
	"name": {}, "summary": {}, "description": {}, "stack": {}, "role": {}, "repo_url": {},
	"live_url": {}, "emoji": {}, "year": {}, "featured": {}, "cover_url": {}, "screenshots": {},
	"problem": {}, "approach": {}, "outcome": {}, "published": {}, "sort": {},
}

type ProjectService struct {
	v1.UnimplementedProjectServiceServer
	uc *biz.ProjectUsecase
}

func NewProjectService(uc *biz.ProjectUsecase) *ProjectService { return &ProjectService{uc: uc} }

func (s *ProjectService) ListProjects(ctx context.Context, req *v1.ListProjectsRequest) (*v1.ProjectSet, error) {
	token, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, invalidListArgument(err)
	}
	if !pageOffsetWithinWindow(token.Offset) {
		return nil, invalidListArgument(errPageOffsetOutOfRange)
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	clampPageSize(&req.PageSize)
	options := biz.ProjectListOptions{Offset: int(token.Offset), Limit: int(req.PageSize)}
	claims, admin := biz.AuthFromContext(ctx)
	admin = admin && claims.Role == biz.UserRoleAdmin
	if admin {
		if req.Published != nil {
			value := req.GetPublished()
			options.Published = &value
		}
	} else {
		value := true
		options.Published = &value
	}
	items, err := s.uc.List(ctx, options)
	if err != nil {
		return nil, err
	}
	out := &v1.ProjectSet{Projects: make([]*v1.Project, 0, len(items))}
	for _, item := range items {
		out.Projects = append(out.Projects, convertProjectReply(item))
	}
	if len(items) >= int(req.PageSize) {
		out.NextPageToken = token.Next(req).String()
	}
	return out, nil
}

func (s *ProjectService) GetProject(ctx context.Context, req *v1.GetProjectRequest) (*v1.Project, error) {
	claims, admin := biz.AuthFromContext(ctx)
	var p *biz.Project
	var err error
	if admin && claims.Role == biz.UserRoleAdmin {
		p, err = s.uc.Get(ctx, req.GetSlug())
	} else {
		p, err = s.uc.GetPublic(ctx, req.GetSlug())
	}
	if err != nil {
		return nil, err
	}
	return convertProjectReply(p), nil
}

func (s *ProjectService) CreateProject(ctx context.Context, req *v1.CreateProjectRequest) (*v1.Project, error) {
	p, err := s.uc.Create(ctx, convertProject(req.GetProject()))
	if err != nil {
		return nil, err
	}
	return convertProjectReply(p), nil
}

func (s *ProjectService) UpdateProject(ctx context.Context, req *v1.UpdateProjectRequest) (*v1.Project, error) {
	if req.GetProject().GetSlug() == "" || req.GetUpdateMask() == nil || len(req.GetUpdateMask().GetPaths()) == 0 {
		return nil, biz.ErrProjectInvalidArgument
	}
	for _, path := range req.GetUpdateMask().GetPaths() {
		if _, ok := allowedProjectUpdateMaskPaths[path]; !ok {
			return nil, biz.ErrProjectInvalidArgument.WithCause(errors.New("unknown update_mask path: " + path))
		}
	}
	current, err := s.uc.Get(ctx, req.GetProject().GetSlug())
	if err != nil {
		return nil, err
	}
	currentDTO := convertProjectReply(current)
	fieldmask.Update(req.GetUpdateMask(), currentDTO, req.GetProject())
	updated, err := s.uc.Update(ctx, convertProject(currentDTO))
	if err != nil {
		return nil, err
	}
	return convertProjectReply(updated), nil
}

func (s *ProjectService) ArchiveProject(ctx context.Context, req *v1.ArchiveProjectRequest) (*emptypb.Empty, error) {
	if err := s.uc.Archive(ctx, req.GetSlug()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func convertProject(in *v1.Project) *biz.Project {
	if in == nil {
		return nil
	}
	return &biz.Project{Slug: in.GetSlug(), Name: in.GetName(), Summary: in.GetSummary(), Description: in.GetDescription(), Stack: append([]string(nil), in.GetStack()...), Role: in.GetRole(), RepoURL: in.GetRepoUrl(), LiveURL: in.GetLiveUrl(), Emoji: in.GetEmoji(), Year: in.GetYear(), Featured: in.GetFeatured(), CoverURL: in.GetCoverUrl(), Screenshots: append([]string(nil), in.GetScreenshots()...), Problem: in.GetProblem(), Approach: in.GetApproach(), Outcome: in.GetOutcome(), Published: in.GetPublished(), Sort: in.GetSort()}
}
func convertProjectReply(in *biz.Project) *v1.Project {
	if in == nil {
		return nil
	}
	return &v1.Project{Id: in.ID.String(), Slug: in.Slug, Name: in.Name, Summary: in.Summary, Description: in.Description, Stack: in.Stack, Role: in.Role, RepoUrl: in.RepoURL, LiveUrl: in.LiveURL, Emoji: in.Emoji, Year: in.Year, Featured: in.Featured, CoverUrl: in.CoverURL, Screenshots: in.Screenshots, Problem: in.Problem, Approach: in.Approach, Outcome: in.Outcome, Published: in.Published, Sort: in.Sort, CreatedAt: timestamppb.New(in.CreatedAt), UpdatedAt: timestamppb.New(in.UpdatedAt)}
}
