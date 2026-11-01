package data

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
)

// Opt-in only: the caller supplies a task-owned disposable MySQL database.
func TestArticleCASMySQL(t *testing.T) {
	dsn := os.Getenv("BLOG_REVISION_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires isolated BLOG_REVISION_MYSQL_DSN")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(8)
	if err = runMigrations(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var old string
	var version uint64
	if err = db.QueryRowContext(ctx, "SELECT content_md,version FROM articles WHERE slug='legacy-article'").Scan(&old, &version); err != nil {
		t.Fatal(err)
	}
	if old != "legacy-before-revisions" || version != 1 {
		t.Fatalf("legacy content/version changed: %q v%d", old, version)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	repo := NewArticleRepo(&Data{db: client}, nil)
	slug := "cas-" + uuid.NewString()
	base, err := repo.CreateArticle(ctx, &biz.Article{Slug: slug, Title: "base", ContentMD: "original", ContentHTML: "<p>original</p>"})
	if err != nil {
		t.Fatal(err)
	}
	if base.Version != 1 {
		t.Fatalf("created version %d", base.Version)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, title := range []string{slug + "-a", slug + "-b"} {
		title := title
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			copy := *base
			copy.Title = title
			copy.Tags = []string{title}
			_, err := repo.UpdateArticle(ctx, &copy)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if biz.ErrArticleVersionConflict.Is(err) {
			conflict++
		} else {
			t.Fatalf("unexpected concurrent write error: %v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	current, err := repo.FindBySlug(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != 2 || len(current.Tags) != 1 || current.Tags[0] != current.Title {
		t.Fatalf("partial commit %+v", current)
	}
	loser := slug + "-a"
	if current.Title == loser {
		loser = slug + "-b"
	}
	var n int
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tags WHERE name=?", loser).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("losing transaction left %d tags", n)
	}
	if strings.TrimSpace(current.ContentMD) != "original" {
		t.Fatal("unmodified source changed")
	}
}

func TestArticleHistoryMySQL(t *testing.T) {
	dsn := os.Getenv("BLOG_REVISION_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires isolated BLOG_REVISION_MYSQL_DSN")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = runMigrations(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	repo := NewArticleRepo(&Data{db: client}, nil)
	slug := "history-" + uuid.NewString()
	a, err := repo.CreateArticle(ctx, &biz.Article{Slug: slug, Title: "initial", ContentMD: "initial-body", ContentHTML: "<p>initial-body</p>"})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM article_revisions WHERE article_id=?", a.ID.String()).Scan(&count); err != nil || count != 1 {
		t.Fatalf("initial snapshot count=%d err=%v", count, err)
	}
	a.Title = "next"
	a.ContentMD = "next-body"
	a.ContentHTML = "<p>next-body</p>"
	a, err = repo.UpdateArticle(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if a.Version != 2 {
		t.Fatalf("version=%d", a.Version)
	}
	var old string
	if err = db.QueryRowContext(ctx, "SELECT JSON_UNQUOTE(JSON_EXTRACT(snapshot,'$.content_md')) FROM article_revisions WHERE article_id=? AND version=1", a.ID.String()).Scan(&old); err != nil || old != "initial-body" {
		t.Fatalf("immutable old snapshot %q %v", old, err)
	}
	trigger := "revision_fail_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	marker := "fail-" + uuid.NewString()
	query := "CREATE TRIGGER " + trigger + " BEFORE INSERT ON article_revisions FOR EACH ROW BEGIN IF JSON_UNQUOTE(JSON_EXTRACT(NEW.snapshot,'$.content_md')) = '" + marker + "' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fixture snapshot failure'; END IF; END"
	if _, err = db.ExecContext(ctx, query); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(context.Background(), "DROP TRIGGER IF EXISTS "+trigger); err != nil {
			t.Error(err)
		}
	})
	a.ContentMD = marker
	a.ContentHTML = "<p>failure</p>"
	a.Tags = []string{marker}
	if _, err = repo.UpdateArticle(ctx, a); err == nil {
		t.Fatal("snapshot insertion failure must abort source update")
	}
	current, err := repo.FindBySlug(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != 2 || current.ContentMD != "next-body" || len(current.Tags) != 0 {
		t.Fatalf("partial source commit: %+v", current)
	}
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM article_revisions WHERE article_id=?", a.ID.String()).Scan(&count); err != nil || count != 2 {
		t.Fatalf("partial history commit count=%d err=%v", count, err)
	}
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tags WHERE name=?", marker).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial tags commit count=%d err=%v", count, err)
	}
	// A downstream outbox insert failure must also undo the already inserted revision.
	outboxTrigger := "outbox_fail_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	outboxHash := "outbox-" + uuid.NewString()
	outboxSQL := "CREATE TRIGGER " + outboxTrigger + " BEFORE INSERT ON wechat_sync_tasks FOR EACH ROW BEGIN IF NEW.content_hash = '" + outboxHash + "' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fixture outbox failure'; END IF; END"
	if _, err = db.ExecContext(ctx, outboxSQL); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(context.Background(), "DROP TRIGGER IF EXISTS "+outboxTrigger); err != nil {
			t.Error(err)
		}
	})
	publisher := repo.(interface {
		UpdateArticleAndEnqueue(context.Context, *biz.Article, *biz.WechatSyncTask) (*biz.Article, error)
	})
	outboxArticle := *current
	outboxArticle.ContentMD = "outbox-update"
	outboxArticle.ContentHTML = "<p>outbox-update</p>"
	task := testWechatTask(slug, outboxHash, "draft", "queued")
	if _, err = publisher.UpdateArticleAndEnqueue(ctx, &outboxArticle, task); err == nil {
		t.Fatal("outbox failure must abort source and revision")
	}
	current, err = repo.FindBySlug(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != 2 || current.ContentMD != "next-body" {
		t.Fatalf("partial outbox source commit %+v", current)
	}
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM article_revisions WHERE article_id=?", a.ID.String()).Scan(&count); err != nil || count != 2 {
		t.Fatalf("partial outbox revision count=%d err=%v", count, err)
	}
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM wechat_sync_tasks WHERE content_hash=?", outboxHash).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial outbox task count=%d err=%v", count, err)
	}
	// Simulate an existing pre-feature article without inventing earlier history.
	legacyID := uuid.NewString()
	legacySlug := "baseline-" + uuid.NewString()
	_, err = db.ExecContext(ctx, "INSERT INTO articles (id,created_at,updated_at,slug,title,content_md,content_html,status,version) VALUES (?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,?,'legacy','old-baseline','<p>old-baseline</p>',1,1)", legacyID, legacySlug)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := repo.FindBySlug(ctx, legacySlug)
	if err != nil {
		t.Fatal(err)
	}
	legacy.ContentMD = "after-baseline"
	legacy.ContentHTML = "<p>after-baseline</p>"
	if _, err = repo.UpdateArticle(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	var action string
	var actorNull bool
	if err = db.QueryRowContext(ctx, "SELECT action,actor_id IS NULL,JSON_UNQUOTE(JSON_EXTRACT(snapshot,'$.content_md')) FROM article_revisions WHERE article_id=? AND version=1", legacyID).Scan(&action, &actorNull, &old); err != nil || action != "baseline" || !actorNull || old != "old-baseline" {
		t.Fatalf("baseline action=%s actorNull=%v old=%q err=%v", action, actorNull, old, err)
	}
}

func TestArticleRestoreMySQL(t *testing.T) {
	dsn := os.Getenv("BLOG_REVISION_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires isolated BLOG_REVISION_MYSQL_DSN")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = runMigrations(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	store := &Data{db: client}
	repo := NewArticleRepo(store, nil)
	uc := biz.NewArticleUsecase(repo, nil)
	slug := "restore-" + uuid.NewString()
	a, err := uc.CreateArticle(ctx, &biz.Article{Slug: slug, Title: "initial restore", ContentMD: "restore-initial"})
	if err != nil {
		t.Fatal(err)
	}
	a.ContentMD = "published-new"
	a.Title = "published title"
	a.Status = biz.ArticleStatusPublished
	a, err = uc.UpdateArticle(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "UPDATE articles SET view_count=8,like_count=3 WHERE id=?", a.ID.String()); err != nil {
		t.Fatal(err)
	}
	before, err := repo.FindBySlug(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := uc.RestoreArticleRevision(ctx, slug, 1, before.Version)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Version != before.Version+1 || restored.ContentMD != "restore-initial" || restored.Status != before.Status || restored.ID != before.ID || restored.ViewCount != 8 || restored.LikeCount != 3 || restored.PublishedAt == nil || before.PublishedAt == nil || !restored.PublishedAt.Equal(*before.PublishedAt) || !restored.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("restored protected metadata changed: before=%+v after=%+v", before, restored)
	}
	if _, err = uc.RestoreArticleRevision(ctx, slug, 1, before.Version); !biz.ErrArticleVersionConflict.Is(err) {
		t.Fatalf("stale restore must conflict: %v", err)
	}
	var count int
	var action, old string
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM article_revisions WHERE article_id=?", a.ID.String()).Scan(&count); err != nil || count != 3 {
		t.Fatalf("restore history count=%d err=%v", count, err)
	}
	if err = db.QueryRowContext(ctx, "SELECT action FROM article_revisions WHERE article_id=? AND version=3", a.ID.String()).Scan(&action); err != nil || action != "restore" {
		t.Fatalf("action=%s err=%v", action, err)
	}
	if err = db.QueryRowContext(ctx, "SELECT JSON_UNQUOTE(JSON_EXTRACT(snapshot,'$.content_md')) FROM article_revisions WHERE article_id=? AND version=1", a.ID.String()).Scan(&old); err != nil || old != "restore-initial" {
		t.Fatalf("old history changed: %q %v", old, err)
	}
	// Restore of a now-invalid category must not silently bind the same slug to a new ID.
	catSlug := "old-cat-" + uuid.NewString()
	cats := NewCategoryRepo(store)
	if _, err = cats.CreateCategory(ctx, &biz.Category{Slug: catSlug, Name: "old category"}); err != nil {
		t.Fatal(err)
	}
	categorized, err := uc.CreateArticle(ctx, &biz.Article{Slug: "cat-" + uuid.NewString(), Title: "with category", ContentMD: "body", CategorySlug: catSlug})
	if err != nil {
		t.Fatal(err)
	}
	if err = cats.DeleteCategory(ctx, catSlug); err != nil {
		t.Fatal(err)
	}
	if _, err = cats.CreateCategory(ctx, &biz.Category{Slug: catSlug, Name: "different category"}); err != nil {
		t.Fatal(err)
	}
	detached, err := repo.FindBySlug(ctx, categorized.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = uc.RestoreArticleRevision(ctx, categorized.Slug, 1, detached.Version); err == nil {
		t.Fatal("deleted historical category must not bind a recreated slug")
	}
	after, err := repo.FindBySlug(ctx, categorized.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if after.Version != detached.Version || after.CategorySlug != "" {
		t.Fatalf("invalid category partially restored %+v", after)
	}
}

func TestCategoryCreateDeleteMySQL(t *testing.T) {
	dsn := os.Getenv("BLOG_REVISION_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires isolated BLOG_REVISION_MYSQL_DSN")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(8)
	if err = runMigrations(db); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store := &Data{db: ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))}
	cats := NewCategoryRepo(store)
	repo := NewArticleRepo(store, nil)
	catSlug := "race-cat-" + uuid.NewString()
	if _, err = cats.CreateCategory(ctx, &biz.Category{Slug: catSlug, Name: "race category"}); err != nil {
		t.Fatal(err)
	}
	original, err := repo.CreateArticle(ctx, &biz.Article{Slug: "race-" + uuid.NewString(), Title: "already linked", ContentMD: "body", ContentHTML: "<p>body</p>", CategorySlug: catSlug})
	if err != nil {
		t.Fatal(err)
	}
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	gate := "gate_" + suffix
	signal := "signal_" + suffix
	trigger := "pause_" + suffix
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	var acquired int
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", gate).Scan(&acquired); err != nil || acquired != 1 {
		t.Fatalf("gate: %d %v", acquired, err)
	}
	t.Cleanup(func() { _, _ = conn.ExecContext(context.Background(), "DO RELEASE_LOCK(?)", gate) })
	query := "CREATE TRIGGER " + trigger + " BEFORE UPDATE ON articles FOR EACH ROW BEGIN IF OLD.id = '" + original.ID.String() + "' AND NEW.version > OLD.version THEN SET @revision_signal = GET_LOCK('" + signal + "',0); SET @revision_gate = GET_LOCK('" + gate + "',15); DO RELEASE_LOCK('" + gate + "'); DO RELEASE_LOCK('" + signal + "'); END IF; END"
	if _, err = db.ExecContext(ctx, query); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(), "DO RELEASE_LOCK(?)", gate)
		if _, err := db.ExecContext(context.Background(), "DROP TRIGGER IF EXISTS "+trigger); err != nil {
			t.Error(err)
		}
	})
	deleted := make(chan error, 1)
	go func() { deleted <- cats.DeleteCategory(ctx, catSlug) }()
	reached := false
	for i := 0; i < 100; i++ {
		var used sql.NullInt64
		if err = db.QueryRowContext(ctx, "SELECT IS_USED_LOCK(?)", signal).Scan(&used); err != nil {
			t.Fatal(err)
		}
		if used.Valid {
			reached = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !reached {
		t.Fatal("delete did not reach controlled article update after enumeration")
	}
	lateSlug := "late-" + uuid.NewString()
	created := make(chan error, 1)
	go func() {
		_, err := repo.CreateArticle(ctx, &biz.Article{Slug: lateSlug, Title: "concurrent new article", ContentMD: "late", ContentHTML: "<p>late</p>", CategorySlug: catSlug})
		created <- err
	}()
	// Give the concurrent INSERT an opportunity to acquire its FK lock while delete is paused.
	var createErr error
	finished := false
	select {
	case createErr = <-created:
		finished = true
	case <-time.After(250 * time.Millisecond):
	}
	if _, err = conn.ExecContext(ctx, "DO RELEASE_LOCK(?)", gate); err != nil {
		t.Fatal(err)
	}
	if err = <-deleted; err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !finished {
		createErr = <-created
	}
	if createErr != nil {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM articles WHERE slug=?", lateSlug).Scan(&n); err != nil || n != 0 {
			t.Fatalf("failed concurrent create left a row: n=%d err=%v", n, err)
		}
		return
	} // A blocked insert must fail atomically after its category disappears.
	current, err := repo.FindBySlug(ctx, lateSlug)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := repo.(biz.ArticleRevisionRepo).GetArticleRevision(ctx, lateSlug, current.Version)
	if err != nil {
		t.Fatal(err)
	}
	if revision.Snapshot.CategorySlug != current.CategorySlug {
		t.Fatalf("FK cleanup bypassed version/history: current v%d category=%q historical=%q", current.Version, current.CategorySlug, revision.Snapshot.CategorySlug)
	}
}

func TestRestoreCategoryDeleteMySQL(t *testing.T) {
	dsn := os.Getenv("BLOG_REVISION_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires isolated BLOG_REVISION_MYSQL_DSN")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(8)
	if err = runMigrations(db); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store := &Data{db: ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))}
	cats := NewCategoryRepo(store)
	repo := NewArticleRepo(store, nil)
	catSlug := "restore-race-" + uuid.NewString()
	if _, err = cats.CreateCategory(ctx, &biz.Category{Slug: catSlug, Name: "original category"}); err != nil {
		t.Fatal(err)
	}
	a, err := repo.CreateArticle(ctx, &biz.Article{Slug: "restore-race-" + uuid.NewString(), Title: "article", ContentMD: "initial", ContentHTML: "<p>initial</p>", CategorySlug: catSlug})
	if err != nil {
		t.Fatal(err)
	}
	a.CategorySlug = ""
	a.ContentMD = "unbound"
	a.ContentHTML = "<p>unbound</p>"
	a, err = repo.UpdateArticle(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	prepared := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	type result struct {
		article *biz.Article
		err     error
	}
	restored := make(chan result, 1)
	go func() {
		value, err := repo.(biz.ArticleRevisionRestorer).RestoreArticleRevision(ctx, a.Slug, 1, a.Version, func(ctx context.Context, value *biz.Article) error {
			close(prepared)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			value.ContentHTML = "<p>initial</p>"
			return nil
		})
		restored <- result{value, err}
	}()
	select {
	case <-prepared:
	case value := <-restored:
		t.Fatalf("restore failed before category lock: %v", value.err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	deleted := make(chan error, 1)
	go func() { deleted <- cats.DeleteCategory(ctx, catSlug) }()
	early := false
	var deleteErr error
	select {
	case deleteErr = <-deleted:
		early = true
	case <-time.After(250 * time.Millisecond):
	}
	unblock()
	value := <-restored
	if early {
		t.Fatalf("category deletion bypassed in-flight restore lock, delete=%v restore=%v", deleteErr, value.err)
	}
	if value.err != nil || value.article.Version != 3 {
		t.Fatalf("restore not serialized: %+v %v", value.article, value.err)
	}
	if deleteErr = <-deleted; deleteErr != nil {
		t.Fatal(deleteErr)
	}
	current, err := repo.FindBySlug(ctx, a.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != 4 || current.CategorySlug != "" || current.ContentMD != "initial" {
		t.Fatalf("category detach did not version restored state %+v", current)
	}
	revision, err := repo.(biz.ArticleRevisionRepo).GetArticleRevision(ctx, a.Slug, 3)
	if err != nil {
		t.Fatal(err)
	}
	if revision.Snapshot.CategorySlug != catSlug {
		t.Fatal("restore snapshot was overwritten by later category deletion")
	}
}
