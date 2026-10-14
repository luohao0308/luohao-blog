package service

import (
	"context"
	"testing"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// The update flow merges the patch into the current record and converts it
// for the usecase; a dropped status would surface as UNSPECIFIED and get
// rejected by the biz layer (the bug that made every update a 400).
func TestConvertArticleCarriesStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status v1.ArticleStatus
		want   biz.ArticleStatus
	}{
		{"draft", v1.ArticleStatus_ARTICLE_STATUS_DRAFT, biz.ArticleStatusDraft},
		{"published", v1.ArticleStatus_ARTICLE_STATUS_PUBLISHED, biz.ArticleStatusPublished},
		{"unspecified stays zero", v1.ArticleStatus_ARTICLE_STATUS_UNSPECIFIED, biz.ArticleStatusUnspecified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := convertArticle(&v1.Article{Slug: "s", Title: "t", ContentMd: "md", Status: tc.status})
			if got.Status != tc.want {
				t.Fatalf("status = %d, want %d", got.Status, tc.want)
			}
		})
	}
}

// stubArticleRepo only records list options; the other repo methods are out
// of scope for the service-level list tests and fail loudly if reached.
type stubArticleRepo struct {
	listOpts []biz.ListOption
	current  *biz.Article
	updated  *biz.Article
}

func (s *stubArticleRepo) FindBySlug(context.Context, string) (*biz.Article, error) {
	if s.current != nil {
		copy := *s.current
		return &copy, nil
	}
	return nil, biz.ErrArticleNotFound
}
func (s *stubArticleRepo) ListArticles(_ context.Context, opts ...biz.ListOption) ([]*biz.Article, error) {
	s.listOpts = opts
	return nil, nil
}
func (s *stubArticleRepo) CreateArticle(context.Context, *biz.Article) (*biz.Article, error) {
	return nil, biz.ErrArticleInvalidArgument
}
func (s *stubArticleRepo) UpdateArticle(_ context.Context, article *biz.Article) (*biz.Article, error) {
	if s.current == nil {
		return nil, biz.ErrArticleInvalidArgument
	}
	s.updated = article
	return article, nil
}
func (s *stubArticleRepo) DeleteArticle(context.Context, string) error {
	return biz.ErrArticleInvalidArgument
}
func (s *stubArticleRepo) IncrementView(context.Context, string, string) (uint64, bool, error) {
	return 0, false, biz.ErrArticleInvalidArgument
}
func (s *stubArticleRepo) IncrementLike(context.Context, string, string) (uint64, bool, error) {
	return 0, false, biz.ErrArticleInvalidArgument
}

// The documented status filter (status:"PUBLISHED", the form the admin panel
// sends) used to fail the declaration check and surface as a 500: the AIP
// parse errors are plain errors that Kratos maps to Unknown. The contract
// promises INVALID_ARGUMENT for malformed list arguments instead.
func TestListArticlesFilterHandling(t *testing.T) {
	ctx := context.Background()
	repo := &stubArticleRepo{}
	svc := NewArticleService(biz.NewArticleUsecase(repo, nil))

	t.Run("documented status filter reaches the repo", func(t *testing.T) {
		repo.listOpts = nil
		if _, err := svc.ListArticles(ctx, &v1.ListArticlesRequest{Filter: `status:"PUBLISHED"`}); err != nil {
			t.Fatalf("ListArticles(status filter) error = %v, want success", err)
		}
		var opts biz.ListOptions
		for _, opt := range repo.listOpts {
			opt(&opts)
		}
		if opts.Filter.CheckedExpr == nil {
			t.Fatal("parsed filter did not reach the repo")
		}
	})
	t.Run("malformed filter is invalid argument", func(t *testing.T) {
		_, err := svc.ListArticles(ctx, &v1.ListArticlesRequest{Filter: `status:`})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("malformed filter error = %v, want bad request", err)
		}
	})
	t.Run("undeclared filter field is invalid argument", func(t *testing.T) {
		_, err := svc.ListArticles(ctx, &v1.ListArticlesRequest{Filter: `bogus="x"`})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("undeclared field error = %v, want bad request", err)
		}
	})
	t.Run("unsupported order_by field is invalid argument", func(t *testing.T) {
		_, err := svc.ListArticles(ctx, &v1.ListArticlesRequest{OrderBy: "content_md"})
		if !kratoserrors.IsBadRequest(err) {
			t.Fatalf("unsupported order_by error = %v, want bad request", err)
		}
	})
}

func TestUpdateArticleRejectsUnknownUpdateMaskPath(t *testing.T) {
	svc := NewArticleService(biz.NewArticleUsecase(&stubArticleRepo{}, nil))
	_, err := svc.UpdateArticle(context.Background(), &v1.UpdateArticleRequest{
		Article: &v1.Article{Slug: "demo"},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"unknown_field"},
		},
	})
	if !kratoserrors.IsBadRequest(err) {
		t.Fatalf("error = %v, want bad request", err)
	}
}

// The category mask path is accepted by the allowlist and its nested slug is
// carried to the usecase; unknown slugs are rejected in the data layer.
func TestConvertArticleCarriesCategorySlug(t *testing.T) {
	got := convertArticle(&v1.Article{
		Slug:      "s",
		Title:     "t",
		ContentMd: "md",
		Category:  &v1.CategoryBrief{Slug: "engineering", Name: "工程实践"},
	})
	if got.CategorySlug != "engineering" {
		t.Fatalf("category slug = %q, want engineering", got.CategorySlug)
	}
	if convertArticle(&v1.Article{Slug: "s"}).CategorySlug != "" {
		t.Fatal("uncategorized article must convert to an empty category slug")
	}
}

func TestArticleMetadataConversionAndLegacyMaskPreservation(t *testing.T) {
	cover := "/v1/assets/media/0123456789abcdef0123456789abcdef.png"
	featured := true
	domain := convertArticle(&v1.Article{
		Slug: "metadata", Title: "title", ContentMd: "body",
		CoverUrl: &cover, Featured: &featured,
	})
	if domain.CoverURL != cover || !domain.Featured {
		t.Fatalf("convertArticle metadata = (%q, %v)", domain.CoverURL, domain.Featured)
	}
	reply := convertArticleReply(domain)
	if reply.CoverUrl == nil || reply.GetCoverUrl() != cover || reply.Featured == nil || !reply.GetFeatured() {
		t.Fatalf("convertArticleReply metadata = (%v, %v)", reply.CoverUrl, reply.Featured)
	}

	// An older client updating only title omits both new optional fields. The
	// current record is merged first, so metadata survives unless named in
	// update_mask.
	repo := &stubArticleRepo{current: &biz.Article{
		Slug: "metadata", Title: "old", ContentMD: "body",
		Status: biz.ArticleStatusPublished, CoverURL: cover, Featured: true,
	}}
	svc := NewArticleService(biz.NewArticleUsecase(repo, nil))
	_, err := svc.UpdateArticle(context.Background(), &v1.UpdateArticleRequest{
		Article:    &v1.Article{Slug: "metadata", Title: "new"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}},
	})
	if err != nil {
		t.Fatalf("legacy UpdateArticle() error = %v", err)
	}
	if repo.updated == nil || repo.updated.CoverURL != cover || !repo.updated.Featured {
		t.Fatalf("legacy update cleared metadata: %+v", repo.updated)
	}

	// Explicit optional zero values remain distinguishable from omission and
	// clear the metadata when their mask paths are present.
	emptyCover := ""
	notFeatured := false
	repo.updated = nil
	_, err = svc.UpdateArticle(context.Background(), &v1.UpdateArticleRequest{
		Article: &v1.Article{
			Slug: "metadata", CoverUrl: &emptyCover, Featured: &notFeatured,
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"cover_url", "featured"}},
	})
	if err != nil {
		t.Fatalf("clear metadata UpdateArticle() error = %v", err)
	}
	if repo.updated == nil || repo.updated.CoverURL != "" || repo.updated.Featured {
		t.Fatalf("explicit zero metadata not applied: %+v", repo.updated)
	}
}

func TestListArticlesAcceptsFeaturedFilterAndOrder(t *testing.T) {
	repo := &stubArticleRepo{}
	svc := NewArticleService(biz.NewArticleUsecase(repo, nil))
	if _, err := svc.ListArticles(context.Background(), &v1.ListArticlesRequest{
		Filter:  "featured",
		OrderBy: "featured desc,published_at desc",
	}); err != nil {
		t.Fatalf("ListArticles(featured) error = %v", err)
	}
	var opts biz.ListOptions
	for _, opt := range repo.listOpts {
		opt(&opts)
	}
	if !opts.Public {
		t.Fatal("anonymous featured list did not keep public boundary")
	}
}
