package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/go-sql-driver/mysql"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func releaseFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../../content/releases/creator-content-20261028.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestManifestAndHash(t *testing.T) {
	raw := releaseFixture(t)
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	if _, err := loadBundle(raw, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBundle(raw, hash, true); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{"", "wrong"} {
		if _, err := loadBundle(raw, hash, true); err == nil {
			t.Fatal("accepted bad hash")
		}
	}
	var b bundle
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*bundle){
		"version":   func(b *bundle) { b.Version = 2 },
		"count":     func(b *bundle) { b.Operations = b.Operations[:12] },
		"duplicate": func(b *bundle) { b.Operations[1] = b.Operations[0] },
		"mask":      func(b *bundle) { b.Operations[0].Mask = append(b.Operations[0].Mask, "cover_url") },
		"media": func(b *bundle) {
			var f map[string]any
			_ = json.Unmarshal(b.Operations[0].Fields, &f)
			f["cover_url"] = "https://example.com/image"
			b.Operations[0].Fields, _ = json.Marshal(f)
		},
		"about_contacts": func(b *bundle) {
			var f map[string]any
			_ = json.Unmarshal(b.Operations[12].Fields, &f)
			f["contacts"] = []any{}
			b.Operations[12].Fields, _ = json.Marshal(f)
		},
		"timestamp": func(b *bundle) { b.Operations[0].Expected = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			var b bundle
			_ = json.Unmarshal(raw, &b)
			mutate(&b)
			changed, _ := json.Marshal(b)
			if _, err := loadBundle(changed, "", false); err == nil {
				t.Fatal("accepted unsafe manifest")
			}
		})
	}
}
func TestSchemaVersionGate(t *testing.T) {
	want, err := highestMigration()
	if err != nil || want != 16 {
		t.Fatalf("migration source: %d %v", want, err)
	}
	for _, tc := range []struct {
		v         uint64
		dirty, ok bool
	}{{16, false, true}, {15, false, false}, {17, false, false}, {16, true, false}, {0, false, false}} {
		if (checkVersion(tc.v, want, tc.dirty) == nil) != tc.ok {
			t.Fatal(tc)
		}
	}
}
func TestAboutPreservesMediaAndContacts(t *testing.T) {
	current := &v1.About{Id: "original", ProfileImage: "/media/profile", Contacts: []*v1.AboutContact{{Href: "https://example.com", Label: "site"}}, RepresentativeLinks: []*v1.AboutLink{{Href: "https://example.com/project"}}}
	b, err := loadBundle(releaseFixture(t), "", false)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := mergeAbout(current, b.Operations[12].Fields)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Id != current.Id || merged.ProfileImage != current.ProfileImage || !proto.Equal(merged.Contacts[0], current.Contacts[0]) || !proto.Equal(merged.RepresentativeLinks[0], current.RepresentativeLinks[0]) || merged.Name == "" {
		t.Fatal("lost preserved fields")
	}
}

type articleMemory struct {
	biz.ArticleRepo
	value  *biz.Article
	writes int
}

func (m *articleMemory) FindBySlug(context.Context, string) (*biz.Article, error) {
	return m.value, nil
}
func (m *articleMemory) UpdateArticle(_ context.Context, a *biz.Article) (*biz.Article, error) {
	m.writes++
	m.value = a
	return a, nil
}
func TestArticlePreflightRendersWithoutWriting(t *testing.T) {
	b, err := loadBundle(releaseFixture(t), "", false)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now()
	m := &articleMemory{value: &biz.Article{Slug: b.Operations[0].Slug, Title: "old", Status: biz.ArticleStatusPublished, PublishedAt: &stamp, CoverURL: "/v1/assets/media/0123456789abcdef0123456789abcdef.webp", Featured: true, CategorySlug: "engineering", ViewCount: 20, LikeCount: 5}}
	ar := &articles{ArticleRepo: m, dry: true, created: map[string]*biz.Article{}}
	r := &runner{a: service.NewArticleService(biz.NewArticleUsecase(ar, nil))}
	ctx := biz.NewAuthContext(context.Background(), &biz.Claims{Role: biz.UserRoleAdmin})
	if err = r.write(ctx, b.Operations[0]); err != nil {
		t.Fatal(err)
	}
	if m.writes != 0 {
		t.Fatal("preflight wrote")
	}
	ar.dry = false
	if err = r.write(ctx, b.Operations[0]); err != nil {
		t.Fatal(err)
	}
	if m.writes != 1 || m.value.ContentHTML == "" || m.value.CoverURL != "/v1/assets/media/0123456789abcdef0123456789abcdef.webp" || m.value.CategorySlug != "engineering" || !m.value.Featured || !m.value.PublishedAt.Equal(stamp) {
		t.Fatal("article lost preserved fields or rendering")
	}
}
func TestBackupPrivateAndNoOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := saveBackup(path, []json.RawMessage{json.RawMessage(`{"id":"public"}`)}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("backup permissions")
	}
	if saveBackup(path, nil) == nil {
		t.Fatal("overwrote backup")
	}
}

type projectMemory struct {
	biz.ProjectRepo
	value  *biz.Project
	writes int
}

func (m *projectMemory) FindBySlug(context.Context, string) (*biz.Project, error) {
	return m.value, nil
}
func (m *projectMemory) UpdateProject(_ context.Context, p *biz.Project) (*biz.Project, error) {
	m.writes++
	m.value = p
	return p, nil
}
func TestProjectPreservesMedia(t *testing.T) {
	b, err := loadBundle(releaseFixture(t), "", false)
	if err != nil {
		t.Fatal(err)
	}
	media := "/v1/assets/media/0123456789abcdef0123456789abcdef.webp"
	m := &projectMemory{value: &biz.Project{Slug: b.Operations[7].Slug, Name: "old", CoverURL: media, Screenshots: []string{media}, Published: true}}
	pr := &projects{ProjectRepo: m, dry: true}
	r := &runner{p: service.NewProjectService(biz.NewProjectUsecase(pr))}
	if err = r.write(context.Background(), b.Operations[7]); err != nil {
		t.Fatal(err)
	}
	if m.writes != 0 {
		t.Fatal("dry run wrote project")
	}
	pr.dry = false
	if err = r.write(context.Background(), b.Operations[7]); err != nil {
		t.Fatal(err)
	}
	if m.value.CoverURL != media || len(m.value.Screenshots) != 1 || m.value.Screenshots[0] != media {
		t.Fatal("lost project media")
	}
}

type failingIndex struct{ biz.ArticleSearchIndex }

func (failingIndex) IndexArticle(context.Context, *biz.Article) error { return errRelease }
func TestIndexFailureCaptured(t *testing.T) {
	i := &checkedIndex{ArticleSearchIndex: failingIndex{}}
	if i.IndexArticle(context.Background(), &biz.Article{}) == nil || i.err == nil {
		t.Fatal("lost index failure")
	}
}

func TestCreatedDraftDatabaseTimestampPrecision(t *testing.T) {
	created := &v1.Article{Id: "same-id", Slug: "new-article", Title: "A", ContentMd: "body", ContentHtml: "<p>body</p>", Tags: []string{"Go", "Ent"}, Status: v1.ArticleStatus_ARTICLE_STATUS_DRAFT, CoverUrl: proto.String(""), Featured: proto.Bool(false), UpdatedAt: timestamppb.New(time.Unix(100, 800000000))}
	for _, ts := range []time.Time{time.Unix(100, 0), time.Unix(101, 0)} {
		fresh := proto.Clone(created).(*v1.Article)
		fresh.UpdatedAt = timestamppb.New(ts)
		fresh.Tags = []string{"Ent", "Go"}
		if !sameCreatedDraft(created, fresh) {
			t.Fatal("database precision must not leave a valid new draft unpublished")
		}
	}
	for _, change := range []func(*v1.Article){func(a *v1.Article) { a.ContentMd = "other edit" }, func(a *v1.Article) { a.Status = v1.ArticleStatus_ARTICLE_STATUS_PUBLISHED }, func(a *v1.Article) { a.UpdatedAt = timestamppb.New(time.Unix(102, 0)) }, func(a *v1.Article) { a.Id = "other-id" }} {
		fresh := proto.Clone(created).(*v1.Article)
		change(fresh)
		if sameCreatedDraft(created, fresh) {
			t.Fatal("changed draft accepted")
		}
	}
}

func TestMySQLCreateReadPublish(t *testing.T) {
	dsn := os.Getenv("BLOG_CONTENT_RELEASE_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires isolated local creator_content_test MySQL")
	}
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil || parsed.DBName != "creator_content_test" || !strings.HasPrefix(parsed.Addr, "127.0.0.1:") {
		t.Fatal("only dedicated localhost test database is allowed")
	}
	store, cleanup, err := data.NewData(&conf.Data{Database: &conf.Data_Database{Driver: "mysql", Source: dsn}})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	svc := service.NewArticleService(biz.NewArticleUsecase(data.NewArticleRepo(store, nil), nil))
	ctx := biz.NewAuthContext(context.Background(), &biz.Claims{Role: biz.UserRoleAdmin})
	slug := "precision-" + time.Now().Format("150405000000")
	created, err := svc.CreateArticle(ctx, &v1.CreateArticleRequest{Article: &v1.Article{Slug: slug, Title: "precision check", ContentMd: "# test", Tags: []string{"test"}, Status: v1.ArticleStatus_ARTICLE_STATUS_PUBLISHED}})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := svc.GetArticle(ctx, &v1.GetArticleRequest{Slug: slug})
	if err != nil {
		t.Fatal(err)
	}
	if !sameCreatedDraft(created, fresh) {
		t.Fatalf("valid MySQL draft must pass real timestamp precision check: created=%v fresh=%v", created, fresh)
	}
	_, err = svc.UpdateArticle(ctx, &v1.UpdateArticleRequest{Article: &v1.Article{Slug: slug, Status: v1.ArticleStatus_ARTICLE_STATUS_PUBLISHED}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"status"}}})
	if err != nil {
		t.Fatal(err)
	}
	published, err := svc.GetArticle(ctx, &v1.GetArticleRequest{Slug: slug})
	if err != nil || published.Status != v1.ArticleStatus_ARTICLE_STATUS_PUBLISHED || published.PublishedAt == nil || published.ContentMd != "# test" {
		t.Fatal("real MySQL create/read/publish failed")
	}
}

func TestReaderIPUpdateBundle(t *testing.T) {
	raw, err := os.ReadFile("../../../content/releases/ip-reader-20261028.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = loadBundle(raw, "", false); err != nil {
		t.Fatal(err)
	}
}
