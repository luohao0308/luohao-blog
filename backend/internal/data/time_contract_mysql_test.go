package data

import (
	"context"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	tc "github.com/luohao0308/luohao-blog/backend/internal/timecontract"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestDataTimeContractMySQL(t *testing.T) {
	raw := os.Getenv("BLOG_UTC_MYSQL_DSN")
	if raw == "" {
		t.Skip("owned UTC consumer fixture required")
	}
	c, e := mysql.ParseDSN(raw)
	if e != nil || c.Net != "tcp" || c.Addr != "127.0.0.1:13330" || !regexp.MustCompile(`^utc_contract_test_[a-f0-9]+$`).MatchString(c.DBName) {
		t.Fatal("refusing non-fixture")
	}
	ctx := context.Background()
	base, e := tc.OpenUTC(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = base.Close() }()
	c.DBName += "_c"
	if _, e = base.Exec("CREATE DATABASE `" + c.DBName + "`"); e != nil {
		t.Fatal(e)
	}
	defer func() { _, _ = base.Exec("DROP DATABASE `" + c.DBName + "`") }()
	db, e := tc.OpenUTC(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = db.Close() }()
	dir := filepath.Join(t.TempDir(), "state")
	cfg := &conf.Data{Database: &conf.Data_Database{Source: c.FormatDSN(), TimeContractStateDir: dir}}
	if _, _, e = NewDataWithStateDir(cfg, dir); e == nil {
		t.Fatal("missing state allowed")
	}
	var n int
	if db.QueryRow("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()").Scan(&n) != nil || n != 0 {
		t.Fatal("gate performed migration")
	}
	if tc.FreshInit(ctx, db, dir, "consumer", func() error { return runMigrations(db) }) != nil {
		t.Fatal("init")
	}
	store, cleanup, e := NewData(cfg)
	if e != nil {
		t.Fatal(e)
	}
	repo := NewArticleRepo(store, nil)
	article, e := repo.CreateArticle(ctx, &biz.Article{Slug: "consumer-time", Title: "fixture", ContentMD: "body", Version: 1})
	if e != nil {
		cleanup()
		t.Fatal(e)
	}
	persisted, pe := repo.FindBySlug(ctx, article.Slug)
	if pe != nil {
		cleanup()
		t.Fatal(pe)
	}
	var epoch int64
	if db.QueryRow("SELECT UNIX_TIMESTAMP(created_at) FROM articles WHERE slug='consumer-time'").Scan(&epoch) != nil || epoch != persisted.CreatedAt.Unix() || epoch-article.CreatedAt.Unix() < -1 || epoch-article.CreatedAt.Unix() > 1 {
		cleanup()
		t.Fatal("new write epoch")
	}
	cleanup()
	s, e := tc.ReadState(dir)
	if e != nil {
		t.Fatal(e)
	}
	s.Status = "pending"
	if tc.WriteState(dir, *s) != nil {
		t.Fatal("pending")
	}
	if _, _, e = NewDataWithStateDir(cfg, dir); e == nil {
		t.Fatal("pending allowed")
	}
}
