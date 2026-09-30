package service

import (
	"context"
	"strings"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultPageSize = 20
)

// ArticleService is an article service.
type ArticleService struct {
	v1.UnimplementedArticleServiceServer

	uc *biz.ArticleUsecase
}

// NewArticleService new an article service.
func NewArticleService(uc *biz.ArticleUsecase) *ArticleService {
	return &ArticleService{uc: uc}
}

// CreateArticle creates an article.
// TODO(M1/S3): render content_md into content_html via goldmark.
func (s *ArticleService) CreateArticle(ctx context.Context, req *v1.CreateArticleRequest) (*v1.Article, error) {
	article, err := s.uc.CreateArticle(ctx, convertArticle(req.GetArticle()))
	if err != nil {
		return nil, err
	}
	return convertArticleReply(article), nil
}

// GetArticle returns an article by slug.
func (s *ArticleService) GetArticle(ctx context.Context, req *v1.GetArticleRequest) (*v1.Article, error) {
	var (
		article *biz.Article
		err     error
	)
	claims, authenticated := biz.AuthFromContext(ctx)
	if authenticated && claims.Role == biz.UserRoleAdmin {
		article, err = s.uc.GetArticle(ctx, req.GetSlug())
	} else {
		article, err = s.uc.GetPublicArticle(ctx, req.GetSlug())
	}
	if err != nil {
		return nil, err
	}
	return convertArticleReply(article), nil
}

// ListArticles lists articles.
func (s *ArticleService) ListArticles(ctx context.Context, req *v1.ListArticlesRequest) (*v1.ArticleSet, error) {
	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("slug", filtering.TypeString),
		filtering.DeclareIdent("title", filtering.TypeString),
		filtering.DeclareIdent("published_at", filtering.TypeTimestamp),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
		filtering.DeclareIdent("updated_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return nil, err
	}
	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, err
	}
	orderBy, err := ordering.ParseOrderBy(req)
	if err != nil {
		return nil, err
	}
	if err := orderBy.ValidateForPaths("slug", "title", "published_at", "created_at", "updated_at"); err != nil {
		return nil, err
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	listOptions := []biz.ListOption{
		biz.ListFilter(filter),
		biz.ListOrderBy(orderBy),
		biz.ListLimit(int(req.PageSize)),
		biz.ListOffset(int(pageToken.Offset)),
	}
	claims, authenticated := biz.AuthFromContext(ctx)
	if !authenticated || claims.Role != biz.UserRoleAdmin {
		listOptions = append(listOptions, biz.ListPublic())
	}
	articles, err := s.uc.ListArticles(ctx, listOptions...)
	if err != nil {
		return nil, err
	}
	set := &v1.ArticleSet{
		Articles: make([]*v1.Article, 0, len(articles)),
	}
	if len(articles) >= int(req.PageSize) {
		set.NextPageToken = pageToken.Next(req).String()
	}
	for _, article := range articles {
		set.Articles = append(set.Articles, convertArticleReply(article))
	}
	return set, nil
}

// UpdateArticle updates an article via field mask. The slug is immutable and
// only identifies the target record.
func (s *ArticleService) UpdateArticle(ctx context.Context, req *v1.UpdateArticleRequest) (*v1.Article, error) {
	if req.GetArticle().GetSlug() == "" || req.GetUpdateMask() == nil || len(req.GetUpdateMask().GetPaths()) == 0 {
		return nil, biz.ErrArticleInvalidArgument
	}
	current, err := s.GetArticle(ctx, &v1.GetArticleRequest{Slug: req.GetArticle().GetSlug()})
	if err != nil {
		return nil, err
	}
	fieldmask.Update(req.GetUpdateMask(), current, req.GetArticle())
	article, err := s.uc.UpdateArticle(ctx, convertArticle(current))
	if err != nil {
		return nil, err
	}
	return convertArticleReply(article), nil
}

// DeleteArticle soft-deletes an article.
func (s *ArticleService) DeleteArticle(ctx context.Context, req *v1.DeleteArticleRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteArticle(ctx, req.GetSlug()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// convertArticle parses an incoming proto into a DO. Server-assigned fields
// (id, content_html, timestamps) are omitted: only the mutable fields cross
// over. The published_at transition is owned by the usecase.
func convertArticle(in *v1.Article) *biz.Article {
	if in == nil {
		return nil
	}
	return &biz.Article{
		Slug:      strings.Clone(in.GetSlug()),
		Title:     in.GetTitle(),
		Summary:   in.GetSummary(),
		ContentMD: in.GetContentMd(),
		Tags:      in.GetTags(),
	}
}

func convertArticleReply(in *biz.Article) *v1.Article {
	if in == nil {
		return nil
	}
	out := &v1.Article{
		Id:          in.ID.String(),
		Slug:        in.Slug,
		Title:       in.Title,
		Summary:     in.Summary,
		ContentMd:   in.ContentMD,
		ContentHtml: in.ContentHTML,
		Tags:        in.Tags,
		Status:      convertArticleStatus(in.Status),
		CreatedAt:   timestamppb.New(in.CreatedAt),
		UpdatedAt:   timestamppb.New(in.UpdatedAt),
	}
	if in.PublishedAt != nil {
		out.PublishedAt = timestamppb.New(*in.PublishedAt)
	}
	return out
}

// convertArticleStatus maps a domain status onto the api enum. The two share
// their numeric values, but the mapping is written out so neither side can
// drift into the other silently.
func convertArticleStatus(in biz.ArticleStatus) v1.ArticleStatus {
	switch in {
	case biz.ArticleStatusDraft:
		return v1.ArticleStatus_ARTICLE_STATUS_DRAFT
	case biz.ArticleStatusPublished:
		return v1.ArticleStatus_ARTICLE_STATUS_PUBLISHED
	case biz.ArticleStatusDeleted:
		return v1.ArticleStatus_ARTICLE_STATUS_DELETED
	default:
		return v1.ArticleStatus_ARTICLE_STATUS_UNSPECIFIED
	}
}
