package data

import (
	"context"
	"database/sql"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/collection"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/user"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

func TestCollectionRepoMySQL(t *testing.T) {
	dsn := os.Getenv("BLOG_COLLECTION_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BLOG_COLLECTION_MYSQL_DSN is not set")
	}
	cfg, err := gomysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid fixture DSN")
	}
	if cfg.Net != "tcp" || cfg.Addr != "127.0.0.1:13316" || !regexp.MustCompile(`^website_completion_[0-9a-f_]+$`).MatchString(cfg.DBName) {
		t.Fatal("refusing non-fixture MySQL target")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	defer func() { _ = client.Close() }()
	ctx := context.Background()
	data := &Data{db: client}
	repo := NewCollectionRepo(data)
	uid1, uid2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	prefix := "collection-fixture-" + uid1.String()[:8]
	hash, _ := biz.HashPassword("longenough1")
	for _, u := range []struct {
		id    uuid.UUID
		email string
	}{{uid1, prefix + "-1@example.com"}, {uid2, prefix + "-2@example.com"}} {
		if _, err = client.User.Create().SetID(u.id).SetEmail(u.email).SetPasswordHash(hash).SetDisplayName("Fixture").SetRole(biz.UserRoleReader).Save(ctx); err != nil {
			t.Fatal(err)
		}
	}
	articleIDs := []uuid.UUID{}
	for _, suffix := range []string{"a", "b", "hidden", "race"} {
		id := uuid.Must(uuid.NewV7())
		articleIDs = append(articleIDs, id)
		slug := prefix + "-" + suffix
		if _, err = client.Article.Create().SetID(id).SetSlug(slug).SetTitle(slug).SetContentMd("body").SetContentHTML("<p>body</p>").SetStatus(biz.ArticleStatusPublished).Save(ctx); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		_, _ = client.Collection.Delete().Where(collection.UserIDIn(uid1, uid2)).Exec(ctx)
		_, _ = client.Article.Delete().Where(article.IDIn(articleIDs...)).Exec(ctx)
		_, _ = client.User.Delete().Where(user.IDIn(uid1, uid2)).Exec(ctx)
	}()
	raceSlug := prefix + "-race"
	const workers = 8
	results := make(chan *biz.Collection, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item, saveErr := repo.Save(ctx, uid1, raceSlug)
			results <- item
			errs <- saveErr
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	var savedAt time.Time
	for saveErr := range errs {
		if saveErr != nil {
			t.Fatalf("concurrent save=%v", saveErr)
		}
	}
	for item := range results {
		if item == nil {
			t.Fatal("nil concurrent result")
		}
		if savedAt.IsZero() {
			savedAt = item.SavedAt
		} else if !item.SavedAt.Equal(savedAt) {
			t.Fatal("duplicate save refreshed saved_at")
		}
	}
	if count, _ := client.Collection.Query().Where(collection.UserIDEQ(uid1), collection.ArticleIDEQ(articleIDs[3])).Count(ctx); count != 1 {
		t.Fatalf("race rows=%d", count)
	}
	tie := time.Now().UTC().Truncate(time.Second)
	for i, suffix := range []string{"a", "b", "hidden"} {
		if _, err = client.Collection.Create().SetSavedAt(tie).SetUserID(uid1).SetArticleID(articleIDs[i]).SetArticleSlug(prefix + "-" + suffix).Save(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = client.Article.Update().Where(article.IDEQ(articleIDs[2])).SetStatus(biz.ArticleStatusDraft).Save(ctx); err != nil {
		t.Fatal(err)
	}
	page1, err := repo.List(ctx, uid1, 1, 0)
	if err != nil || len(page1) != 1 || page1[0].Article.Slug != prefix+"-a" {
		t.Fatalf("page1=%+v %v", page1, err)
	}
	page2, err := repo.List(ctx, uid1, 1, 1)
	if err != nil || len(page2) != 1 || page2[0].Article.Slug != prefix+"-b" {
		t.Fatalf("page2=%+v %v", page2, err)
	}
	if _, err = repo.Save(ctx, uid2, prefix+"-a"); err != nil {
		t.Fatal(err)
	}
	other, err := repo.List(ctx, uid2, 10, 0)
	if err != nil || len(other) != 1 {
		t.Fatalf("other=%+v %v", other, err)
	}
}
