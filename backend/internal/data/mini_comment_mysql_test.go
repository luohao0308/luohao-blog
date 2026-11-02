package data

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/comment"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatcommentcheck"
)

// This opens only the migrated, disposable runner database. It never creates a
// schema and intentionally does not print driver errors, which can contain DSNs.
func miniMySQL(t *testing.T) (*ent.Client, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("BLOG_MINI_CONTENT_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BLOG_MINI_CONTENT_MYSQL_DSN not set; isolated MySQL acceptance fixture required")
	}
	cfg, err := gomysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid mini fixture DSN")
	}
	if cfg.Net != "tcp" || cfg.Addr != "127.0.0.1:13316" || !regexp.MustCompile(`^website_completion_[0-9a-f_]+$`).MatchString(cfg.DBName) {
		t.Fatal("refusing non-fixture mini MySQL target")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("open mini fixture failed")
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var version int
	var dirty bool
	if err := db.QueryRowContext(ctx, "SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty); err != nil || version != 21 || dirty {
		t.Fatal("mini fixture must have clean migration 21")
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	return client, db
}

func TestMiniCommentMySQLHTTP(t *testing.T) {
	client, _ := miniMySQL(t)
	miniCommentHTTPSafetyLifecycle(t, client)
}

func TestMiniCommentMySQLSchemaAndNullableTimes(t *testing.T) {
	_, sqlDB := miniMySQL(t)
	ctx := context.Background()
	for _, column := range []string{"lease_until", "checked_at"} {
		var nullable, typ string
		var precision int
		if err := sqlDB.QueryRowContext(ctx, `SELECT IS_NULLABLE,DATA_TYPE,DATETIME_PRECISION FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='wechat_comment_checks' AND COLUMN_NAME=?`, column).Scan(&nullable, &typ, &precision); err != nil || nullable != "YES" || typ != "datetime" || precision != 6 {
			t.Fatal("mini nullable timestamp schema mismatch", column)
		}
	}
	rows, err := sqlDB.QueryContext(ctx, `SELECT REFERENCED_TABLE_NAME,DELETE_RULE FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='wechat_comment_checks'`)
	if err != nil {
		t.Fatal("mini FK metadata unavailable")
	}
	rules := map[string]string{}
	for rows.Next() {
		var table, rule string
		if err := rows.Scan(&table, &rule); err != nil {
			t.Fatal("mini FK metadata unreadable")
		}
		rules[table] = rule
	}
	if err := rows.Err(); err != nil {
		t.Fatal("mini FK metadata iteration failed")
	}
	_ = rows.Close()
	if len(rules) != 2 || rules["users"] != "CASCADE" || rules["wechat_identities"] != "SET NULL" {
		t.Fatal("mini FK rules mismatch")
	}
	r, db, a := miniFixture(t)
	c := miniCreate(t, r, a, "mysql-null", "hello")
	if c.LeaseUntil.IsZero() {
		t.Fatal("initial lease absent")
	}
	c = miniPass(t, r, c, true)
	check, err := db.WechatCommentCheck.Query().Where(wechatcommentcheck.CommentIDEQ(c.Comment.ID)).Only(ctx)
	if err != nil || check.LeaseUntil != nil || check.CheckedAt == nil {
		t.Fatal("completed timestamp persistence mismatch")
	}
	if err := NewCommentRepo(&Data{db: db}).Delete(ctx, c.Comment.ID); err != nil {
		t.Fatal("strict-mode tombstone deletion failed")
	}
	check, err = db.WechatCommentCheck.Query().Where(wechatcommentcheck.CommentIDEQ(c.Comment.ID)).Only(ctx)
	if err != nil || check.LeaseUntil != nil || check.CheckedAt != nil || check.IdentityID != nil || check.InputHash != "" || check.State != string(biz.MiniCommentDeleted) {
		t.Fatal("nullable tombstone persistence mismatch")
	}
}

func TestMiniCommentMySQLConcurrentAtMostOne(t *testing.T) {
	miniMySQL(t) // Explicit skip without opt-in; miniFixture otherwise uses SQLite.
	r, db, a := miniFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	type result struct {
		row     *biz.MiniComment
		created bool
		err     error
	}
	results := make(chan result, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			row, created, err := r.CreatePending(ctx, &biz.Comment{ArticleSlug: "post", Content: "same"}, a, "mysql-concurrent", biz.MiniCommentRequestHash("post", "same"), time.Minute)
			results <- result{row, created, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	count := 0
	var winner *biz.MiniComment
	for got := range results {
		if got.err != nil || got.row == nil {
			t.Fatal("concurrent fixture create failed", got.err)
		}
		if got.created {
			count++
		}
		if winner == nil {
			winner = got.row
		} else if winner.Comment.ID != got.row.Comment.ID {
			t.Fatal("multiple request winners")
		}
	}
	n, err := db.Comment.Query().Where(comment.UserIDEQ(a.UserID)).Count(ctx)
	if err != nil || count != 1 || n != 1 {
		t.Fatal("request race did not preserve at-most-one", count, n)
	}
}

// Poll server evidence rather than assuming scheduler timing or lock queue FIFO.
// No lock metadata permission/visibility is an acceptance failure, not a skip.
func waitMiniMySQLLock(t *testing.T, db *sql.DB, done <-chan error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var n int
		err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks l ON l.ENGINE=w.ENGINE AND l.ENGINE_LOCK_ID=w.REQUESTING_ENGINE_LOCK_ID WHERE l.OBJECT_SCHEMA=DATABASE() AND l.OBJECT_NAME='comments'`).Scan(&n)
		if err != nil {
			t.Fatal("performance_schema lock-wait evidence unavailable")
		}
		if n > 0 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("repository never entered observed comment lock wait")
		case <-done:
			t.Fatal("repository completed before observed lock wait")
		case <-ticker.C:
		}
	}
}

func TestMiniCommentMySQLLatePassLockOrdering(t *testing.T) {
	_, observer := miniMySQL(t)
	for _, deletion := range []bool{false, true} {
		name := "generation"
		if deletion {
			name = "deletion"
		}
		t.Run(name, func(t *testing.T) {
			r, db, a := miniFixture(t)
			c := miniCreate(t, r, a, "mysql-lock", "hello")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tx, err := db.Tx(ctx)
			if err != nil {
				t.Fatal("fixture transaction failed")
			}
			defer func() { _ = tx.Rollback() }()
			if _, err = tx.Comment.Query().Where(comment.IDEQ(c.Comment.ID)).ForUpdate().Only(ctx); err != nil {
				t.Fatal("fixture comment lock failed")
			}
			done := make(chan error, 1)
			go func() {
				_, err := r.CompleteCheck(ctx, c, biz.MiniCommentVerdict{State: biz.MiniCommentPass, NameTraceID: "name", ContentTraceID: "content"}, true)
				done <- err
			}()
			waitMiniMySQLLock(t, observer, done)
			// Follow repository lock order: comment first, then its safety evidence.
			check, err := tx.WechatCommentCheck.Query().Where(wechatcommentcheck.CommentIDEQ(c.Comment.ID)).ForUpdate().Only(ctx)
			if err != nil {
				t.Fatal("fixture check lock failed")
			}
			if deletion {
				if _, err = tx.WechatCommentCheck.UpdateOneID(check.ID).SetState(string(biz.MiniCommentDeleted)).AddGeneration(1).ClearIdentityID().SetInputHash("").ClearLeaseUntil().Save(ctx); err != nil {
					t.Fatal("fixture tombstone failed")
				}
				if err = tx.Comment.DeleteOneID(c.Comment.ID).Exec(ctx); err != nil {
					t.Fatal("fixture deletion failed")
				}
			} else {
				if _, err = tx.WechatCommentCheck.UpdateOneID(check.ID).AddGeneration(1).Save(ctx); err != nil {
					t.Fatal("fixture generation update failed")
				}
			}
			if err = tx.Commit(); err != nil {
				t.Fatal("fixture commit failed")
			}
			err = <-done
			if deletion {
				if !errors.Is(err, biz.ErrMiniCommentDeleted) && !errors.Is(err, biz.ErrCommentNotFound) {
					t.Fatal("late pass did not reject deleted comment", err)
				}
				if exists, e := db.Comment.Query().Where(comment.IDEQ(c.Comment.ID)).Exist(ctx); e != nil || exists {
					t.Fatal("late pass resurrected comment")
				}
			} else {
				if !errors.Is(err, biz.ErrMiniCommentConflict) {
					t.Fatal("late pass did not reject obsolete generation", err)
				}
				current, e := db.Comment.Get(ctx, c.Comment.ID)
				if e != nil || current.Status != biz.CommentStatusPending {
					t.Fatal("obsolete pass changed global approval")
				}
			}
			stored, e := db.WechatCommentCheck.Get(ctx, check.ID)
			if e != nil || stored.Generation != 2 || stored.NameTraceID != "" || stored.ContentTraceID != "" {
				t.Fatal("obsolete pass overwrote current evidence")
			}
		})
	}
}
