package data

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/migrate"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/user"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatidentity"
)

func identityMySQL(t *testing.T) (*ent.Client, *userRepo) {
	t.Helper()
	dsn := os.Getenv("BLOG_WECHAT_IDENTITY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BLOG_WECHAT_IDENTITY_MYSQL_DSN not set; real fixture required for release acceptance")
	}
	cfg, err := gomysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid fixture DSN")
	}
	if cfg.Net != "tcp" || cfg.Addr != "127.0.0.1:13316" || !regexp.MustCompile(`^website_completion_[0-9a-f_]+$`).MatchString(cfg.DBName) {
		t.Fatal("refusing non-fixture target")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("open fixture failed")
	}
	for column, size := range map[string]int{"app_id": 32, "openid": 64} {
		var actual int
		var collation string
		if err := db.QueryRow("SELECT CHARACTER_MAXIMUM_LENGTH, COLLATION_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='wechat_identities' AND COLUMN_NAME=?", column).Scan(&actual, &collation); err != nil || actual != size || collation != "utf8mb4_bin" {
			t.Fatal("identity schema length/case-sensitive collation mismatch", column, actual, collation, err)
		}
	}
	var deleteRule string
	if err := db.QueryRow("SELECT DELETE_RULE FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='wechat_identities'").Scan(&deleteRule); err != nil || deleteRule != "CASCADE" {
		t.Fatal("identity FK delete rule mismatch", deleteRule, err)
	}
	if got := migrate.WechatIdentitiesTable.ForeignKeys[0].OnDelete; got != "CASCADE" {
		t.Fatal("generated Ent FK disagrees with versioned migration", got)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	t.Cleanup(func() { _ = client.Close() })
	return client, &userRepo{data: &Data{db: client}}
}

func identityUser(t *testing.T, client *ent.Client, legacy string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	create := client.User.Create().SetID(id).SetEmail(id.String() + "@example.invalid").SetPasswordHash("fixture-hash").SetRole(biz.UserRoleReader)
	if legacy != "" {
		create.SetWechatOpenid(legacy)
	}
	if _, err := create.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = client.User.Delete().Where(user.IDEQ(id)).Exec(context.Background()) })
	return id
}

func TestWechatIdentityMySQL(t *testing.T) {
	client, repo := identityMySQL(t)
	ctx := context.Background()
	a, b := identityUser(t, client, "legacy-"+uuid.NewString()), identityUser(t, client, "")
	app1, app2 := "wx1111111111111111", "wx2222222222222222"
	openid := uuid.NewString()
	if _, err := repo.FindByWechatIdentity(ctx, app1, openid); !errors.Is(err, biz.ErrUserNotFound) {
		t.Fatal("unexpected initial lookup")
	}
	if err := repo.BindWechatIdentity(ctx, a, 0, app1, openid); err != nil {
		t.Fatal(err)
	}
	if err := repo.BindWechatIdentity(ctx, a, 0, app1, openid); err != nil {
		t.Fatal("exact duplicate not idempotent", err)
	}
	if err := repo.BindWechatIdentity(ctx, b, 0, app1, openid); !errors.Is(err, biz.ErrUserWechatConflict) {
		t.Fatal("identity stolen", err)
	}
	if err := repo.BindWechatIdentity(ctx, a, 0, app1, openid+"other"); !errors.Is(err, biz.ErrUserWechatConflict) {
		t.Fatal("existing user identity replaced", err)
	}
	if err := repo.BindWechatIdentity(ctx, b, 0, app2, openid); err != nil {
		t.Fatal("cross-app OpenID collision", err)
	}
	for app, want := range map[string]uuid.UUID{app1: a, app2: b} {
		got, err := repo.FindByWechatIdentity(ctx, app, openid)
		if err != nil || got.ID != want {
			t.Fatal("lookup isolation failed")
		}
	}
	if err := repo.BindWechatIdentity(ctx, a, 0, app2, "Case"+openid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByWechatIdentity(ctx, app2, "case"+openid); !errors.Is(err, biz.ErrUserNotFound) {
		t.Fatal("OpenID comparison is not case sensitive")
	}
	// Application identifiers are also compared with the issuing case intact.
	if err := repo.BindWechatIdentity(ctx, b, 0, "WX2222222222222222", openid); err != nil {
		t.Fatal("AppID comparison is not case sensitive", err)
	}
	for _, value := range []struct{ app, openid string }{{"", openid}, {app1, ""}, {"wx" + string(make([]byte, 31)), openid}, {app1, string(make([]byte, 65))}} {
		if err := repo.BindWechatIdentity(ctx, b, 0, value.app, value.openid); !errors.Is(err, biz.ErrUserWechatConflict) {
			t.Fatal("invalid identity accepted", err)
		}
	}
	old, _ := client.User.Get(ctx, a)
	if old.WechatOpenid == "" {
		t.Fatal("legacy identity erased")
	}
	if _, updated, err := repo.UpdatePasswordState(ctx, a, 0, "next", false); err != nil || !updated {
		t.Fatal(err)
	}
	if err := repo.BindWechatIdentity(ctx, a, 0, app1, openid); !errors.Is(err, biz.ErrUserInvalidCredentials) {
		t.Fatal("stale credentials accepted", err)
	}
}

func TestWechatIdentityMySQLConcurrentUnique(t *testing.T) {
	client, repo := identityMySQL(t)
	ctx := context.Background()
	for _, sameUser := range []bool{false, true} {
		a, b := identityUser(t, client, ""), identityUser(t, client, "")
		ids := []uuid.UUID{a, b}
		names := []string{uuid.NewString(), ""}
		names[1] = names[0]
		if sameUser {
			ids[1] = a
			names[1] = uuid.NewString()
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		for i := range ids {
			go func(i int) {
				<-start
				results <- repo.BindWechatIdentity(ctx, ids[i], 0, "wx3333333333333333", names[i])
			}(i)
		}
		close(start)
		successes, conflicts := 0, 0
		for range 2 {
			err := <-results
			if err == nil {
				successes++
			} else if errors.Is(err, biz.ErrUserWechatConflict) {
				conflicts++
			} else {
				t.Fatal(err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("unique race results %d/%d", successes, conflicts)
		}
	}
}

func TestWechatIdentityMySQLPasswordLockOrdering(t *testing.T) {
	client, repo := identityMySQL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t.Run("reset commits first", func(t *testing.T) {
		id := identityUser(t, client, "")
		tx, err := client.Tx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err = tx.User.Query().Where(user.IDEQ(id)).ForUpdate().Only(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.User.UpdateOneID(id).SetCredentialVersion(1).Save(ctx); err != nil {
			t.Fatal(err)
		}
		result := make(chan error, 1)
		go func() { result <- repo.BindWechatIdentity(ctx, id, 0, "wx4444444444444444", uuid.NewString()) }()
		select {
		case <-result:
			t.Fatal("binding bypassed user lock")
		case <-time.After(100 * time.Millisecond):
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if err = <-result; !errors.Is(err, biz.ErrUserInvalidCredentials) {
			t.Fatal("reset-first stale version accepted", err)
		}
		if count, _ := client.WechatIdentity.Query().Where(wechatidentity.UserIDEQ(id)).Count(ctx); count != 0 {
			t.Fatal("stale identity inserted")
		}
	})
	t.Run("bind holds lock before reset", func(t *testing.T) {
		id := identityUser(t, client, "")
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		// This hook pauses the actual repository transaction after user locking,
		// immediately before the identity INSERT, without replacing SQL behavior.
		client.WechatIdentity.Use(func(next ent.Mutator) ent.Mutator {
			return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
				once.Do(func() { close(entered); <-release })
				return next.Mutate(ctx, m)
			})
		})
		bound := make(chan error, 1)
		go func() { bound <- repo.BindWechatIdentity(ctx, id, 0, "wx5555555555555555", uuid.NewString()) }()
		<-entered
		reset := make(chan error, 1)
		go func() {
			_, ok, e := repo.UpdatePasswordState(ctx, id, 0, "after", false)
			if e == nil && !ok {
				e = errors.New("reset CAS failed")
			}
			reset <- e
		}()
		select {
		case <-reset:
			close(release)
			t.Fatal("password update bypassed binding lock")
		case <-time.After(100 * time.Millisecond):
		}
		close(release)
		if err := <-bound; err != nil {
			t.Fatal(err)
		}
		if err := <-reset; err != nil {
			t.Fatal(err)
		}
		got, _ := client.User.Get(ctx, id)
		if got.CredentialVersion != 1 {
			t.Fatal("reset not applied")
		}
		if count, _ := client.WechatIdentity.Query().Where(wechatidentity.UserIDEQ(id)).Count(ctx); count != 1 {
			t.Fatal("bind-first identity absent")
		}
	})
}

func TestWechatIdentityMySQLBackfill(t *testing.T) {
	client, _ := identityMySQL(t)
	ctx := context.Background()
	app := "wx6666666666666666"
	id := identityUser(t, client, "backfill-"+uuid.NewString())
	old, _ := client.User.Get(ctx, id)
	if _, err := BackfillWechatIdentities(ctx, client, "", false, false); err == nil {
		t.Fatal("missing legacy app accepted")
	}
	if _, err := BackfillWechatIdentities(ctx, client, app, true, false); err == nil {
		t.Fatal("unpaused apply accepted")
	}
	report, err := BackfillWechatIdentities(ctx, client, app, false, false)
	if err != nil || report.Applied || report.Inserted < 1 {
		t.Fatal("dry run failed", report, err)
	}
	if count, _ := client.WechatIdentity.Query().Where(wechatidentity.UserIDEQ(id)).Count(ctx); count != 0 {
		t.Fatal("dry run committed")
	}
	first, err := BackfillWechatIdentities(ctx, client, app, true, true)
	if err != nil || !first.Applied {
		t.Fatal(err)
	}
	again, err := BackfillWechatIdentities(ctx, client, app, true, true)
	if err != nil || again.Inserted != 0 || again.Existing != first.Legacy {
		t.Fatal("backfill not idempotent", again, err)
	}
	current, _ := client.User.Get(ctx, id)
	if current.WechatOpenid != old.WechatOpenid {
		t.Fatal("old column changed")
	}
	conflictID := identityUser(t, client, "conflict-"+uuid.NewString())
	if _, err = client.WechatIdentity.Create().SetUserID(conflictID).SetAppID(app).SetOpenid("different-" + uuid.NewString()).Save(ctx); err != nil {
		t.Fatal(err)
	}
	pendingID := identityUser(t, client, "pending-"+uuid.NewString())
	if _, err = BackfillWechatIdentities(ctx, client, app, true, true); !errors.Is(err, biz.ErrUserWechatConflict) {
		t.Fatal("conflict did not abort", err)
	}
	if count, _ := client.WechatIdentity.Query().Where(wechatidentity.UserIDEQ(pendingID)).Count(ctx); count != 0 {
		t.Fatal("conflicting transaction partially committed")
	}
}

// commitAcknowledgmentLostDriver preserves real MySQL execution and commit;
// only the first successful commit acknowledgment is replaced by an error.
type commitAcknowledgmentLostDriver struct {
	dialect.Driver
	inject atomic.Bool
}

func (d *commitAcknowledgmentLostDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	tx, err := d.Driver.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return &commitAcknowledgmentLostTx{Tx: tx, inject: &d.inject}, nil
}

type commitAcknowledgmentLostTx struct {
	dialect.Tx
	inject *atomic.Bool
}

func (tx *commitAcknowledgmentLostTx) Commit() error {
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	if tx.inject.CompareAndSwap(true, false) {
		return errors.New("fixture lost successful COMMIT acknowledgment")
	}
	return nil
}
func TestWechatIdentityMySQLBackfillCommitUnknown(t *testing.T) {
	client, _ := identityMySQL(t)
	ctx := context.Background()
	app := "wx9999999999999999"
	id := identityUser(t, client, "commit-unknown-"+uuid.NewString())
	db, err := sql.Open("mysql", os.Getenv("BLOG_WECHAT_IDENTITY_MYSQL_DSN"))
	if err != nil {
		t.Fatal("open fixture failed")
	}
	driver := &commitAcknowledgmentLostDriver{Driver: entsql.OpenDB(dialect.MySQL, db)}
	driver.inject.Store(true)
	wrapped := ent.NewClient(ent.Driver(driver))
	defer func() { _ = wrapped.Close() }()
	report, err := BackfillWechatIdentities(ctx, wrapped, app, true, true)
	if !errors.Is(err, ErrWechatBackfillCommitUnknown) || report.Applied {
		t.Fatal("commit ambiguity misreported", report, err)
	}
	// Independent ordinary connection proves the commit happened despite failure.
	if count, e := client.WechatIdentity.Query().Where(wechatidentity.UserIDEQ(id), wechatidentity.AppIDEQ(app)).Count(ctx); e != nil || count != 1 {
		t.Fatal("lost-ack fixture did not commit", count, e)
	}
	check, err := BackfillWechatIdentities(ctx, client, app, false, true)
	if err != nil || check.Inserted != 0 || check.Existing < 1 || check.Applied {
		t.Fatal("safe explicit recheck failed", check, err)
	}
	again, err := BackfillWechatIdentities(ctx, client, app, true, true)
	if err != nil || again.Inserted != 0 || !again.Applied {
		t.Fatal("explicit idempotent retry failed", again, err)
	}
}
