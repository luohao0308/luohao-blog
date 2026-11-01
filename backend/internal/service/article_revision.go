package service

import (
	"context"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/biz/article/render"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *ArticleService) ListArticleRevisions(ctx context.Context, req *v1.ListArticleRevisionsRequest) (*v1.ArticleRevisionSet, error) {
	token, e := pagination.ParsePageToken(req)
	if e != nil {
		return nil, invalidListArgument(e)
	}
	if !pageOffsetWithinWindow(token.Offset) {
		return nil, invalidListArgument(errPageOffsetOutOfRange)
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	rows, e := s.uc.ListArticleRevisions(ctx, req.Slug, int(req.PageSize), int(token.Offset))
	if e != nil {
		return nil, e
	}
	out := &v1.ArticleRevisionSet{Revisions: make([]*v1.ArticleRevision, 0, len(rows))}
	for _, row := range rows {
		out.Revisions = append(out.Revisions, revisionReply(row))
	}
	if len(rows) >= int(req.PageSize) {
		out.NextPageToken = token.Next(req).String()
	}
	return out, nil
}
func revisionReply(r *biz.ArticleRevision) *v1.ArticleRevision {
	out := &v1.ArticleRevision{ArticleSlug: r.ArticleSlug, Version: r.Version, Action: r.Action, CreatedAt: timestamppb.New(r.CreatedAt)}
	if r.ActorID != nil {
		out.ActorId = r.ActorID.String()
	}
	return out
}
func (s *ArticleService) GetArticleRevision(ctx context.Context, req *v1.GetArticleRevisionRequest) (*v1.ArticleRevision, error) {
	row, e := s.uc.GetArticleRevision(ctx, req.Slug, req.Version)
	if e != nil {
		return nil, e
	}
	if row.Snapshot == nil {
		return nil, biz.ErrArticleNotFound
	}
	out := revisionReply(row)
	v := row.Snapshot
	html, e := render.HTML(v.ContentMD)
	if e != nil {
		return nil, biz.ErrArticleInvalidArgument
	}
	a := &biz.Article{Slug: req.Slug, Version: row.Version, Title: v.Title, Summary: v.Summary, ContentMD: v.ContentMD, ContentHTML: html, Tags: v.Tags, CategorySlug: v.CategorySlug, CategoryName: v.CategoryName, CoverURL: v.CoverURL, Featured: v.Featured, Status: v.Status, PublishedAt: v.PublishedAt}
	out.Snapshot = convertArticleReply(a)
	out.CategoryId = v.CategoryID
	return out, nil
}

func (s *ArticleService) RestoreArticleRevision(ctx context.Context, req *v1.RestoreArticleRevisionRequest) (*v1.Article, error) {
	a, e := s.uc.RestoreArticleRevision(ctx, req.Slug, req.Version, req.ExpectedVersion)
	if e != nil {
		return nil, e
	}
	return convertArticleReply(a), nil
}
