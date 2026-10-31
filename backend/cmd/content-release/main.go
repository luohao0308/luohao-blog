// Command content-release applies the approved creator bundle once. It never
// provisions identities, sessions or search infrastructure. See README.md.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	mysql "github.com/go-sql-driver/mysql"
	"io"
	stdlog "log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/env"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/log"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
	"github.com/luohao0308/luohao-blog/backend/migrations"
	"go.einride.tech/aip/fieldmask"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var errRelease = errors.New("content release rejected")

type operation struct {
	RevisionID   string          `json:"-"`
	Before       json.RawMessage `json:"-"`
	Preserved    json.RawMessage `json:"-"`
	Kind         string          `json:"kind"`
	Slug         string          `json:"slug"`
	Expected     string          `json:"expected_updated_at"`
	CheckSlug    bool            `json:"requires_admin_slug_check"`
	Fields       json.RawMessage `json:"fields"`
	Mask         []string        `json:"update_mask"`
	Preserve     []string        `json:"preserve_fields"`
	CoverDisplay string          `json:"cover_display"`
	ContentHash  string          `json:"expected_content_sha256"`
}
type bundle struct {
	Version    int         `json:"schemaVersion"`
	Status     string      `json:"status"`
	Target     string      `json:"target"`
	Base       string      `json:"base"`
	Requires   []string    `json:"requires"`
	Operations []operation `json:"operations"`
}

var articlePaths = []string{"title", "summary", "content_md", "tags"}
var projectPaths = []string{"name", "summary", "description", "stack", "role", "repo_url", "live_url", "emoji", "year", "featured", "published", "sort"}
var aboutPaths = []string{"name", "headline", "intro", "current_focus", "skills", "timeline"}

func same(a, b []string) bool {
	a = append([]string(nil), a...)
	b = append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, "\x00") == strings.Join(b, "\x00")
}
func loadBundle(raw []byte, hash string, apply bool) (*bundle, error) {
	sum := sha256.Sum256(raw)
	if (apply || hash != "") && hash != hex.EncodeToString(sum[:]) {
		return nil, errRelease
	}
	var b bundle
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&b) != nil || d.Decode(new(any)) != io.EOF || b.Version != 1 || b.Status != "prepared_not_applied" || b.Target != "https://makerhao.cn" || len(b.Operations) != 13 {
		return nil, errRelease
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	for _, o := range b.Operations {
		key := strings.TrimPrefix(strings.TrimPrefix(o.Kind, "update_"), "create_") + "/" + o.Slug
		if seen[key] {
			return nil, errRelease
		}
		seen[key] = true
		counts[o.Kind]++
		paths := articlePaths
		switch o.Kind {
		case "update_article", "create_article":
		case "update_project", "create_project":
			paths = projectPaths
		case "update_about":
			paths = aboutPaths
		default:
			return nil, errRelease
		}
		if o.Kind != "update_about" && (!biz.ValidSlug(o.Slug) || !same(paths, o.Mask)) {
			return nil, errRelease
		}
		if o.Kind == "update_about" && (o.Slug != "" || len(o.Mask) != 0 || !same(o.Preserve, []string{"id", "profile_image", "contacts", "representative_links"})) {
			return nil, errRelease
		}
		if strings.HasPrefix(o.Kind, "create_") {
			if !o.CheckSlug || o.Expected != "" {
				return nil, errRelease
			}
		} else {
			if _, err := time.Parse(time.RFC3339Nano, o.Expected); err != nil {
				return nil, errRelease
			}
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(o.Fields, &fields) != nil || len(fields) == 0 {
			return nil, errRelease
		}
		allowed := map[string]bool{}
		for _, p := range paths {
			allowed[p] = true
		}
		if o.Kind != "update_about" {
			allowed["slug"] = true
			var slug string
			if json.Unmarshal(fields["slug"], &slug) != nil || slug != o.Slug {
				return nil, errRelease
			}
		}
		if strings.HasSuffix(o.Kind, "article") {
			allowed["status"] = true
			var status int
			if json.Unmarshal(fields["status"], &status) != nil || status != 2 {
				return nil, errRelease
			}
		}
		for k, v := range fields {
			if !allowed[k] || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				return nil, errRelease
			}
		}
		for _, p := range paths {
			if _, ok := fields[p]; !ok {
				return nil, errRelease
			}
		}
		var msg proto.Message = &v1.Article{}
		if strings.HasSuffix(o.Kind, "project") {
			msg = &v1.Project{}
		}
		if o.Kind == "update_about" {
			msg = &v1.About{}
		}
		if protojson.Unmarshal(o.Fields, msg) != nil {
			return nil, errRelease
		}
	}
	initialRelease := counts["update_article"] == 4 && counts["create_article"] == 3 && counts["update_project"] == 3 && counts["create_project"] == 2
	readerUpdate := counts["update_article"] == 7 && counts["create_article"] == 0 && counts["update_project"] == 5 && counts["create_project"] == 0
	if !initialRelease && !readerUpdate || counts["update_about"] != 1 {
		return nil, errRelease
	}
	return &b, nil
}

// Derive the gate from the same embedded source used by data.NewData.
func highestMigration() (uint64, error) {
	files, err := migrations.FS.ReadDir(".")
	if err != nil {
		return 0, err
	}
	var highest uint64
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".up.sql") {
			n, e := strconv.ParseUint(strings.SplitN(f.Name(), "_", 2)[0], 10, 64)
			if e != nil {
				return 0, e
			}
			if n > highest {
				highest = n
			}
		}
	}
	if highest == 0 {
		return 0, errRelease
	}
	return highest, nil
}
func checkVersion(version, want uint64, dirty bool) error {
	if dirty || version != want {
		return errRelease
	}
	return nil
}
func schemaGate(ctx context.Context, db *sql.DB) error {
	want, err := highestMigration()
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, "SELECT version, dirty FROM schema_migrations")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return errRelease
	}
	var v uint64
	var dirty bool
	if err = rows.Scan(&v, &dirty); err != nil {
		return err
	}
	if rows.Next() {
		return errRelease
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return checkVersion(v, want, dirty)
}

// Dry repositories use real read methods but intercept every write exercised
// by the services. This runs existing validation/rendering without DB writes.
type articles struct {
	biz.ArticleRepo
	dry     bool
	created map[string]*biz.Article
}

func (r *articles) FindBySlug(c context.Context, s string) (*biz.Article, error) {
	if a := r.created[s]; r.dry && a != nil {
		return a, nil
	}
	return r.ArticleRepo.FindBySlug(c, s)
}
func (r *articles) CreateArticle(c context.Context, a *biz.Article) (*biz.Article, error) {
	if r.dry {
		r.created[a.Slug] = a
		return a, nil
	}
	return r.ArticleRepo.CreateArticle(c, a)
}
func (r *articles) UpdateArticle(c context.Context, a *biz.Article) (*biz.Article, error) {
	if r.dry {
		return a, nil
	}
	return r.ArticleRepo.UpdateArticle(c, a)
}

type projects struct {
	biz.ProjectRepo
	dry bool
}

func (r *projects) CreateProject(c context.Context, p *biz.Project) (*biz.Project, error) {
	if r.dry {
		return p, nil
	}
	return r.ProjectRepo.CreateProject(c, p)
}
func (r *projects) UpdateProject(c context.Context, p *biz.Project) (*biz.Project, error) {
	if r.dry {
		return p, nil
	}
	return r.ProjectRepo.UpdateProject(c, p)
}

type about struct {
	biz.AboutRepo
	dry bool
}

func (r *about) UpdateAbout(c context.Context, a *biz.About) (*biz.About, error) {
	if r.dry {
		return a, nil
	}
	return r.AboutRepo.UpdateAbout(c, a)
}

type runner struct {
	db *sql.DB
	a  *service.ArticleService
	p  *service.ProjectService
	b  *service.AboutService
}

func timestampMatches(t *timestamppb.Timestamp, expected string) bool {
	e, err := time.Parse(time.RFC3339Nano, expected)
	return err == nil && t != nil && t.IsValid() && t.AsTime().Equal(e)
}
func (r *runner) check(ctx context.Context, o operation) (proto.Message, error) {
	if strings.HasPrefix(o.Kind, "create_") {
		table := "articles"
		if o.Kind == "create_project" {
			table = "projects"
		}
		var n int
		err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE slug = ?", o.Slug).Scan(&n)
		if err != nil || n != 0 {
			return nil, errRelease
		}
		return nil, nil
	}
	var m proto.Message
	var t *timestamppb.Timestamp
	switch o.Kind {
	case "update_article":
		a, e := r.a.GetArticle(ctx, &v1.GetArticleRequest{Slug: o.Slug})
		if e != nil {
			return nil, e
		}
		m = a
		t = a.UpdatedAt
	case "update_project":
		p, e := r.p.GetProject(ctx, &v1.GetProjectRequest{Slug: o.Slug})
		if e != nil {
			return nil, e
		}
		m = p
		t = p.UpdatedAt
	case "update_about":
		b, e := r.b.GetAbout(ctx, &v1.GetAboutRequest{})
		if e != nil {
			return nil, e
		}
		m = b
		t = b.UpdatedAt
	default:
		return nil, errRelease
	}
	if !timestampMatches(t, o.Expected) {
		return nil, errRelease
	}
	if o.RevisionID != "" {
		if err := revisionIdentity(m, o); err != nil {
			return nil, err
		}
		if err := compareRevisionFields(m, o.Before); err != nil {
			return nil, err
		}
		if err := compareRevisionFields(m, o.Preserved); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// MySQL TIMESTAMP columns use whole seconds, while a fresh Ent create can
// retain nanoseconds. Match the persisted draft identity and content, allowing
// only the database's rounding/truncation interval for its update timestamp.
func sameCreatedDraft(created, fresh *v1.Article) bool {
	if created == nil || fresh == nil || created.UpdatedAt == nil || fresh.UpdatedAt == nil || !created.UpdatedAt.IsValid() || !fresh.UpdatedAt.IsValid() {
		return false
	}
	delta := fresh.UpdatedAt.AsTime().Sub(created.UpdatedAt.AsTime())
	if delta < -time.Second || delta > time.Second {
		return false
	}
	return created.Id == fresh.Id && created.Slug == fresh.Slug && created.Title == fresh.Title && created.Summary == fresh.Summary && created.ContentMd == fresh.ContentMd && created.ContentHtml == fresh.ContentHtml && same(created.Tags, fresh.Tags) && fresh.Status == v1.ArticleStatus_ARTICLE_STATUS_DRAFT && fresh.PublishedAt == nil && fresh.GetCoverUrl() == created.GetCoverUrl() && fresh.GetFeatured() == created.GetFeatured() && proto.Equal(fresh.Category, created.Category)
}
func mergeAbout(current *v1.About, raw []byte) (*v1.About, error) {
	patch := &v1.About{}
	if err := protojson.Unmarshal(raw, patch); err != nil {
		return nil, err
	}
	out := proto.Clone(current).(*v1.About)
	fieldmask.Update(&fieldmaskpb.FieldMask{Paths: aboutPaths}, out, patch)
	return out, nil
}
func (r *runner) write(ctx context.Context, o operation) error {
	switch o.Kind {
	case "create_article", "update_article":
		a := &v1.Article{}
		if err := protojson.Unmarshal(o.Fields, a); err != nil {
			return err
		}
		if o.Kind == "create_article" {
			created, err := r.a.CreateArticle(ctx, &v1.CreateArticleRequest{Article: a})
			if err != nil {
				return err
			}
			fresh, err := r.a.GetArticle(ctx, &v1.GetArticleRequest{Slug: created.Slug})
			if err != nil || !sameCreatedDraft(created, fresh) {
				return errRelease
			}
			a = &v1.Article{Slug: created.Slug, Status: v1.ArticleStatus_ARTICLE_STATUS_PUBLISHED}
			_, err = r.a.UpdateArticle(ctx, &v1.UpdateArticleRequest{Article: a, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"status"}}})
			return err
		}
		_, err := r.a.UpdateArticle(ctx, &v1.UpdateArticleRequest{Article: a, UpdateMask: &fieldmaskpb.FieldMask{Paths: o.Mask}})
		return err
	case "create_project", "update_project":
		p := &v1.Project{}
		if err := protojson.Unmarshal(o.Fields, p); err != nil {
			return err
		}
		if o.Kind == "create_project" {
			_, err := r.p.CreateProject(ctx, &v1.CreateProjectRequest{Project: p})
			return err
		}
		_, err := r.p.UpdateProject(ctx, &v1.UpdateProjectRequest{Project: p, UpdateMask: &fieldmaskpb.FieldMask{Paths: o.Mask}})
		return err
	case "update_about":
		current, err := r.b.GetAbout(ctx, &v1.GetAboutRequest{})
		if err != nil {
			return err
		}
		merged, err := mergeAbout(current, o.Fields)
		if err != nil {
			return err
		}
		_, err = r.b.UpdateAbout(ctx, &v1.UpdateAboutRequest{About: merged})
		return err
	}
	return errRelease
}
func saveBackup(path string, snapshots []json.RawMessage) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err = json.NewEncoder(f).Encode(snapshots); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	return f.Close()
}
func run() error {
	confPath := flag.String("conf", "./configs", "Kratos config path")
	bundlePath := flag.String("bundle", "../content/releases/creator-content-20261028.json", "release bundle")
	hash := flag.String("bundle-sha256", "", "exact SHA256 of bundle bytes")
	apply := flag.Bool("apply", false, "apply after full preflight")
	revision := flag.Bool("revision", false, "use the fixed six-item portfolio revision schema")
	backup := flag.String("backup", "", "new snapshot file (required for apply)")
	flag.Parse()
	if flag.NArg() != 0 || (*apply && *backup == "") {
		return errRelease
	}
	raw, err := os.ReadFile(*bundlePath)
	if err != nil {
		return err
	}
	var b *bundle
	if *revision {
		b, err = loadRevision(raw, *hash, *apply)
	} else {
		b, err = loadBundle(raw, *hash, *apply)
	}
	if err != nil {
		return err
	}
	// Suppress framework/driver diagnostics: they can contain config or SQL.
	log.SetDefault(log.NewLogger(log.NewHandler(log.WithWriter(io.Discard))))
	stdlog.SetOutput(io.Discard)
	_ = mysql.SetLogger(silentLogger{})
	c := config.New(config.WithSource(file.NewSource(*confPath), env.NewSource("KRATOS")))
	defer func() { _ = c.Close() }()
	if err = c.Load(); err != nil {
		return err
	}
	var bc conf.Bootstrap
	if err = c.Scan(&bc); err != nil {
		return err
	}
	if bc.GetData().GetDatabase() == nil {
		return errRelease
	}
	bc.Data.GetDatabase().Debug = false
	db, err := sql.Open("mysql", bc.GetData().GetDatabase().GetSource())
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err = schemaGate(ctx, db); err != nil {
		return err
	}
	if err = probeES(ctx, &bc); err != nil {
		return err
	}
	store, cleanup, err := data.NewData(bc.Data)
	if err != nil {
		return err
	}
	defer cleanup()
	ar := &articles{ArticleRepo: data.NewArticleRepo(store, nil), dry: true, created: map[string]*biz.Article{}}
	pr := &projects{ProjectRepo: data.NewProjectRepo(store), dry: true}
	ab := &about{AboutRepo: data.NewAboutRepo(store), dry: true}
	r := &runner{db: db, a: service.NewArticleService(biz.NewArticleUsecase(ar, nil)), p: service.NewProjectService(biz.NewProjectUsecase(pr)), b: service.NewAboutService(biz.NewAboutUsecase(ab))}
	// An in-process role selects management reads; no identity/session/token is created.
	ctx = biz.NewAuthContext(ctx, &biz.Claims{Role: biz.UserRoleAdmin})
	snapshots := []json.RawMessage{}
	for _, o := range b.Operations {
		m, e := r.check(ctx, o)
		if e != nil {
			return e
		}
		if m != nil {
			raw, e := protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)
			if e != nil {
				return e
			}
			snapshots = append(snapshots, raw)
		}
		if e = r.write(ctx, o); e != nil {
			return e
		}
		fmt.Printf("kind=%s slug=%s status=preflight count=1\n", o.Kind, o.Slug)
	}
	if !*apply {
		return nil
	}
	if err = saveBackup(*backup, snapshots); err != nil {
		return err
	}
	idx, err := data.NewEsIndexer(&bc)
	if err != nil || idx == nil {
		return errRelease
	}
	strict := &checkedIndex{ArticleSearchIndex: idx}
	r.a = service.NewArticleService(biz.NewArticleUsecase(ar, strict))
	ar.dry = false
	pr.dry = false
	ab.dry = false
	for _, o := range b.Operations {
		if err = schemaGate(ctx, db); err != nil {
			return err
		}
		if _, err = r.check(ctx, o); err != nil {
			return err
		}
		if err = r.write(ctx, o); err != nil {
			return err
		}
		if strict.err != nil {
			return errRelease
		}
		if isRevisionOperation(o) {
			var fresh proto.Message
			if o.Kind == "update_article" {
				fresh, err = r.a.GetArticle(ctx, &v1.GetArticleRequest{Slug: o.Slug})
			} else {
				fresh, err = r.p.GetProject(ctx, &v1.GetProjectRequest{Slug: o.Slug})
			}
			if err != nil {
				return err
			}
			if err = checkRevisionResult(fresh, o); err != nil {
				return err
			}
		}
		fmt.Printf("kind=%s slug=%s status=applied count=1\n", o.Kind, o.Slug)
	}
	if refresh, ok := idx.(interface{ Refresh(context.Context) error }); ok {
		if err = refresh.Refresh(ctx); err != nil {
			return err
		}
	}
	return nil
}
func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "content-release: operation failed; stopped")
		os.Exit(1)
	}
}

// Capture the otherwise best-effort business-layer indexing error.
type checkedIndex struct {
	biz.ArticleSearchIndex
	err error
}

func (i *checkedIndex) IndexArticle(c context.Context, a *biz.Article) error {
	i.err = i.ArticleSearchIndex.IndexArticle(c, a)
	return i.err
}

type silentLogger struct{}

func (silentLogger) Print(...any) {}

// HEAD does not create an index (NewEsIndexer can). Require an existing index
// before invoking that constructor, including during read-only preflight.
func probeES(ctx context.Context, c *conf.Bootstrap) error {
	es := c.GetEs()
	if es == nil || len(es.GetAddresses()) == 0 {
		return errRelease
	}
	index := es.GetIndex()
	if index == "" {
		index = "articles"
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, address := range es.GetAddresses() {
		u, err := url.Parse(address)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return errRelease
		}
		u.Path = strings.TrimRight(u.Path, "/") + "/" + url.PathEscape(index)
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
		if err != nil {
			return errRelease
		}
		res, err := client.Do(req)
		if err != nil {
			return errRelease
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return errRelease
		}
	}
	return nil
}
