package timecontract_test

import (
	"context"
	"database/sql"
	"errors"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	mm "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	tc "github.com/luohao0308/luohao-blog/backend/internal/timecontract"
	"github.com/luohao0308/luohao-blog/backend/migrations"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func migration(t *testing.T, db *sql.DB, v uint) error {
	t.Helper()
	src, e := iofs.New(migrations.FS, ".")
	if e != nil {
		return e
	}
	d, e := mm.WithInstance(db, &mm.Config{})
	if e != nil {
		return e
	}
	m, e := migrate.NewWithInstance("iofs", src, "mysql", d)
	if e != nil {
		return e
	}
	e = m.Migrate(v)
	if e == migrate.ErrNoChange {
		return nil
	}
	return e
}
func fixture(t *testing.T) (context.Context, *sql.DB, *sql.DB) {
	t.Helper()
	raw := os.Getenv("BLOG_UTC_MYSQL_DSN")
	if raw == "" {
		t.Skip("owned UTC MySQL fixture required")
	}
	c, e := mysql.ParseDSN(raw)
	if e != nil || c.Net != "tcp" || c.Addr != "127.0.0.1:13330" || !regexp.MustCompile(`^utc_contract_test_[a-f0-9]+$`).MatchString(c.DBName) {
		t.Fatal("refusing non-fixture target")
	}
	c.ParseTime = true
	c.Loc = time.UTC
	if c.Params == nil {
		c.Params = map[string]string{}
	}
	c.Params["time_zone"] = "'+08:00'"
	old, e := sql.Open("mysql", c.FormatDSN())
	if e != nil {
		t.Fatal("fixture open")
	}
	ctx := context.Background()
	db, e := tc.OpenUTC(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = old.Close(); _ = db.Close() })
	return ctx, old, db
}
func TestMySQLContract(t *testing.T) {
	ctx, old, db := fixture(t)
	if e := migration(t, old, 20); e != nil {
		t.Fatal("migration20")
	}
	now := time.Date(2026, 11, 2, 14, 50, 32, 0, time.UTC)
	if _, e := old.Exec("INSERT INTO articles(id,created_at,updated_at,slug,title,status) VALUES('00000000-0000-7000-8000-000000000001',?,?, 'utc-case','fixture',1)", now, now); e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(t.TempDir(), "control")
	if tc.RequireReady(ctx, db, dir) == nil {
		t.Fatal("legacy allowed")
	}
	proof, e := tc.VerifyLegacyDSN(ctx, os.Getenv("BLOG_UTC_MYSQL_DSN"))
	if e != nil {
		t.Fatal("legacy proof", e)
	}
	m, e := tc.PlanLegacy(ctx, db, "test-conversion", proof)
	if e != nil {
		t.Fatal("plan", e)
	}
	a := tc.Attempt{StateDir: dir, BackupSHA256: tc.Hash("fixture-backup"), WritersPaused: true, EnsureMetadata: func() error { return migration(t, db, 21) }}
	br, be := tc.NewBackupReceipt(ctx, db, m, tc.Hash("fixture-backup"), tc.Hash("sidecar"))
	if be != nil {
		t.Fatal(be)
	}
	a.Backup = &br
	a.BeforeCommit = func() error { return errors.New("synthetic precommit failure") }
	if _, e = tc.ApplyLegacy(ctx, db, m, a); e == nil {
		t.Fatal("fault accepted")
	}
	r, e := tc.Reconcile(ctx, db, m, a)
	if e != nil || r.Outcome != "not_committed" {
		current, _ := tc.Capture(ctx, db)
		t.Log("snapshot schema hashes", current.SchemaDigest, m.Before.SchemaDigest)
		for i, table := range current.Tables {
			if i < len(m.Before.Tables) && tc.Hash(table) != tc.Hash(m.Before.Tables[i]) {
				t.Log("table hash differs", table.Name, tc.Hash(table), tc.Hash(m.Before.Tables[i]))
			}
		}
		t.Fatal("rollback not atomic", r, e)
	}
	if tc.RequireReady(ctx, db, dir) == nil {
		t.Fatal("unmarked21 allowed")
	}
	a.Resume = true
	a.BeforeCommit = nil
	a.Commit = func(tx *sql.Tx) error {
		if e := tx.Commit(); e != nil {
			return e
		}
		return errors.New("synthetic lost commit acknowledgement")
	}
	if _, e = tc.ApplyLegacy(ctx, db, m, a); !errors.Is(e, tc.ErrUnknown) {
		t.Fatal("unknown outcome not reported", e)
	}
	r, e = tc.Reconcile(ctx, db, m, a)
	if e != nil || r.Outcome != "committed_verified" {
		t.Fatal("unknown outcome reconcile", r, e)
	}
	if tc.RequireReady(ctx, db, dir) == nil {
		t.Fatal("pending activation allowed")
	}
	a.Commit = nil
	pendingState, _ := tc.ReadState(dir)
	originalPending := *pendingState
	pendingState.Kind = "restore"
	pendingState.AttemptNonce = tc.Hash("synthetic-restore")
	if tc.WriteState(dir, *pendingState) != nil {
		t.Fatal("fixture")
	}
	if _, err := tc.Activate(ctx, db, m, a); err == nil {
		t.Fatal("convert activation cleared restore")
	}
	wrongTarget := originalPending
	pendingState = &wrongTarget
	pendingState.Target.Database += "_wrong"
	if tc.WriteState(dir, *pendingState) != nil {
		t.Fatal("fixture")
	}
	if _, err := tc.Activate(ctx, db, m, a); err == nil {
		t.Fatal("wrong target activation")
	}
	if tc.WriteState(dir, originalPending) != nil {
		t.Fatal("fixture")
	}
	r, e = tc.Activate(ctx, db, m, a)
	if e != nil {
		t.Fatal("apply", r, e)
	}
	if tc.RequireReady(ctx, db, dir) != nil {
		t.Fatal("ready rejected")
	}
	db.SetMaxOpenConns(4)
	held := []*sql.Conn{}
	for i := 0; i < 3; i++ {
		c, e := db.Conn(ctx)
		if e != nil {
			t.Fatal(e)
		}
		held = append(held, c)
		var zone string
		if c.QueryRowContext(ctx, "SELECT @@session.time_zone").Scan(&zone) != nil || zone != "+00:00" {
			t.Fatal("pool timezone")
		}
	}
	for _, c := range held {
		_ = c.Close()
	}
	var stamp time.Time
	var epoch int64
	if db.QueryRow("SELECT created_at,UNIX_TIMESTAMP(created_at) FROM articles WHERE slug='utc-case'").Scan(&stamp, &epoch) != nil || !stamp.Equal(now) || epoch != now.Unix() {
		t.Fatal("epoch mismatch")
	}
	if _, e = db.Exec("UPDATE articles SET title='legitimate later edit' WHERE slug='utc-case'"); e != nil {
		t.Fatal(e)
	}
	r, e = tc.ApplyLegacy(ctx, db, m, a)
	if e != nil || r.Outcome != "already_complete" {
		t.Fatal("replay", r, e)
	}
	snap, e := tc.Capture(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	s, e := tc.Inspect(ctx, db, dir)
	if e != nil {
		t.Fatal(e)
	}
	bh, mh := tc.Hash("backup"), tc.Hash("backupmanifest")
	if tc.RestoreBegin(ctx, db, dir, "restore-1", bh, mh, true) != nil {
		t.Fatal("restore begin")
	}
	if tc.RequireReady(ctx, db, dir) == nil {
		t.Fatal("pending allowed")
	}
	restoreState, _ := tc.ReadState(dir)
	rr := tc.RestoreReceipt{AttemptNonce: restoreState.AttemptNonce, FormatVersion: 1, OperationID: "restore-1", Target: s.Target, BackupSHA256: bh, ManifestSHA256: mh, ImportSucceeded: true, RedumpVerified: true, SnapshotSHA256: tc.Hash(snap)}
	if _, e = db.Exec("UPDATE time_contract SET contract_version=2 WHERE id=1"); e != nil {
		t.Fatal(e)
	}
	if tc.RestoreComplete(ctx, db, dir, rr, true) == nil {
		t.Fatal("invalid marker restored to ready")
	}
	pending, _ := tc.ReadState(dir)
	if pending.Status != "pending" {
		t.Fatal("invalid restore cleared pending")
	}
	if _, e = db.Exec("UPDATE time_contract SET contract_version=1 WHERE id=1"); e != nil {
		t.Fatal(e)
	}
	if tc.RestoreComplete(ctx, db, dir, rr, true) != nil || tc.RequireReady(ctx, db, dir) != nil {
		t.Fatal("restore complete")
	}
	if tc.RestoreBegin(ctx, db, dir, "restore-1", bh, mh, false) != nil {
		t.Fatal("reused label begin")
	}
	newAttempt, _ := tc.ReadState(dir)
	if newAttempt.AttemptNonce == rr.AttemptNonce {
		t.Fatal("nonce reused")
	}
	if tc.RestoreComplete(ctx, db, dir, rr, true) == nil {
		t.Fatal("old receipt cleared new attempt")
	}
	rr.AttemptNonce = newAttempt.AttemptNonce
	if tc.RestoreComplete(ctx, db, dir, rr, true) != nil {
		t.Fatal("new nonce completion")
	}

}
func TestMySQLFreshInit(t *testing.T) {
	ctx, _, base := fixture(t)
	var name string
	if base.QueryRow("SELECT DATABASE()").Scan(&name) != nil {
		t.Fatal("database")
	}
	name += "_a"
	if _, e := base.Exec("CREATE DATABASE `" + name + "`"); e != nil {
		t.Fatal(e)
	}
	defer func() { _, _ = base.Exec("DROP DATABASE `" + name + "`") }()
	c, e := mysql.ParseDSN(os.Getenv("BLOG_UTC_MYSQL_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	c.DBName = name
	db, e := tc.OpenUTC(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = db.Close() }()
	dir := filepath.Join(t.TempDir(), "fresh")
	if tc.FreshInit(ctx, db, dir, "fresh-init", func() error { return migration(t, db, 21) }) != nil {
		t.Fatal("fresh init")
	}
	if tc.RequireReady(ctx, db, dir) != nil {
		t.Fatal("fresh ready")
	}
	var n int
	if db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&n) != nil || n == 0 {
		t.Fatal("seed missing")
	}
	if tc.FreshInit(ctx, db, dir, "again", func() error { return nil }) == nil {
		t.Fatal("nonempty re-init")
	}
}
func TestMySQLConcurrentOperator(t *testing.T) {
	ctx, _, base := fixture(t)
	var name string
	if base.QueryRow("SELECT DATABASE()").Scan(&name) != nil {
		t.Fatal("database")
	}
	name += "_b"
	if _, e := base.Exec("CREATE DATABASE `" + name + "`"); e != nil {
		t.Fatal(e)
	}
	defer func() { _, _ = base.Exec("DROP DATABASE `" + name + "`") }()
	c, e := mysql.ParseDSN(os.Getenv("BLOG_UTC_MYSQL_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	c.DBName = name
	db, e := tc.OpenUTC(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = db.Close() }()
	legacy := *c
	legacy.ParseTime = true
	legacy.Loc = time.UTC
	legacy.Params = map[string]string{"time_zone": "'+08:00'"}
	old, e := sql.Open("mysql", legacy.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = old.Close() }()
	if migration(t, old, 20) != nil {
		t.Fatal("migration")
	}
	proof, e := tc.VerifyLegacyDSN(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	m, e := tc.PlanLegacy(ctx, db, "concurrent", proof)
	if e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(t.TempDir(), "state")
	entered, release := make(chan struct{}), make(chan struct{})
	a := tc.Attempt{StateDir: dir, BackupSHA256: tc.Hash("backup"), WritersPaused: true, EnsureMetadata: func() error { return migration(t, db, 21) }, BeforeCommit: func() error { close(entered); <-release; return nil }}
	br, be := tc.NewBackupReceipt(ctx, db, m, tc.Hash("backup"), tc.Hash("sidecar"))
	if be != nil {
		t.Fatal(be)
	}
	a.Backup = &br
	done := make(chan error, 1)
	go func() { _, e := tc.ApplyLegacy(ctx, db, m, a); done <- e }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("operator never reached transaction")
	}
	other := a
	other.BeforeCommit = nil
	other.Resume = true
	if _, e = tc.ApplyLegacy(ctx, db, m, other); e == nil {
		close(release)
		t.Fatal("concurrent operator allowed")
	}
	close(release)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
}

func TestMySQLMixedLegacyRejected(t *testing.T) {
	ctx, _, base := fixture(t)
	var name string
	if base.QueryRow("SELECT DATABASE()").Scan(&name) != nil {
		t.Fatal("database")
	}
	name += "_e"
	if _, e := base.Exec("CREATE DATABASE `" + name + "`"); e != nil {
		t.Fatal(e)
	}
	defer func() { _, _ = base.Exec("DROP DATABASE `" + name + "`") }()
	c, e := mysql.ParseDSN(os.Getenv("BLOG_UTC_MYSQL_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	c.DBName = name
	db, e := tc.OpenUTC(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = db.Close() }()
	if migration(t, db, 20) != nil {
		t.Fatal("migration20")
	}
	proof, e := tc.VerifyLegacyDSN(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	m, e := tc.PlanLegacy(ctx, db, "mixed-case", proof)
	if e != nil {
		t.Fatal(e)
	}
	br, e := tc.NewBackupReceipt(ctx, db, m, tc.Hash("archive"), tc.Hash("sidecar"))
	if e != nil {
		t.Fatal(e)
	}
	before, e := tc.Capture(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE TABLE time_contract(id INT PRIMARY KEY)"); e != nil {
		t.Fatal(e)
	}
	if _, e = tc.PlanLegacy(ctx, db, "new-mixed-case", proof); e == nil {
		t.Fatal("mixed plan accepted")
	}
	dir := filepath.Join(t.TempDir(), "control")
	called := false
	a := tc.Attempt{StateDir: dir, Backup: &br, WritersPaused: true, EnsureMetadata: func() error { called = true; return migration(t, db, 21) }}
	if _, e = tc.ApplyLegacy(ctx, db, m, a); e == nil {
		t.Fatal("mixed apply accepted")
	}
	if called || !tc.StateAbsent(dir) {
		t.Fatal("mixed apply changed control or metadata")
	}
	var version int
	var dirty bool
	if db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty) != nil || version != 20 || dirty {
		t.Fatal("mixed schema mutated")
	}
	after, e := tc.Capture(ctx, db)
	if e != nil || tc.Hash(before) != tc.Hash(after) {
		t.Fatal("mixed business data changed", e)
	}
	if _, e = db.Exec("ALTER TABLE projects CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); e != nil {
		t.Fatal(e)
	}
	changed, e := tc.Capture(ctx, db)
	if e != nil || changed.SchemaDigest == before.SchemaDigest {
		t.Fatal("real collation drift omitted", e)
	}

}

func TestMySQLViewRejected(t *testing.T) {
	ctx, _, base := fixture(t)
	var name string
	if base.QueryRow("SELECT DATABASE()").Scan(&name) != nil {
		t.Fatal("database")
	}
	c, e := mysql.ParseDSN(os.Getenv("BLOG_UTC_MYSQL_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	name += "_f"
	outside := name + "a"
	for _, n := range []string{name, outside} {
		if _, e = base.Exec("CREATE DATABASE `" + n + "`"); e != nil {
			t.Fatal(e)
		}
	}
	defer func() {
		_, _ = base.Exec("DROP DATABASE `" + name + "`")
		_, _ = base.Exec("DROP DATABASE `" + outside + "`")
	}()
	c.DBName = name
	db, e := tc.OpenUTC(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = db.Close() }()
	if migration(t, db, 20) != nil {
		t.Fatal("migration20")
	}
	proof, e := tc.VerifyLegacyDSN(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	m, e := tc.PlanLegacy(ctx, db, "view-case", proof)
	if e != nil {
		t.Fatal(e)
	}
	br, e := tc.NewBackupReceipt(ctx, db, m, tc.Hash("archive"), tc.Hash("sidecar"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("RENAME TABLE projects TO `" + outside + "`.projects"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("ALTER TABLE `" + outside + "`.projects ENGINE=MyISAM"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE ALGORITHM=MERGE VIEW projects AS SELECT * FROM `" + outside + "`.projects"); e != nil {
		t.Fatal(e)
	}
	rows, e := db.Query("SELECT id,UNIX_TIMESTAMP(created_at),UNIX_TIMESTAMP(updated_at) FROM `" + outside + "`.projects ORDER BY id")
	if e != nil {
		t.Fatal(e)
	}
	observed := []string{}
	for rows.Next() {
		var id string
		var a, b int64
		if rows.Scan(&id, &a, &b) != nil {
			t.Fatal("scan")
		}
		observed = append(observed, id+":"+time.Unix(a, 0).String()+":"+time.Unix(b, 0).String())
	}
	_ = rows.Close()
	if _, e = tc.PlanLegacy(ctx, db, "view-other", proof); e == nil {
		t.Fatal("view plan allowed")
	}
	dir := filepath.Join(t.TempDir(), "state")
	called := false
	if _, e = tc.ApplyLegacy(ctx, db, m, tc.Attempt{StateDir: dir, Backup: &br, WritersPaused: true, EnsureMetadata: func() error { called = true; return migration(t, db, 21) }}); e == nil {
		t.Fatal("view apply allowed")
	}
	if called || !tc.StateAbsent(dir) {
		t.Fatal("view apply wrote metadata")
	}
	var v int
	var dirty bool
	if db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&v, &dirty) != nil || v != 20 || dirty {
		t.Fatal("view schema changed")
	}
	rows, e = db.Query("SELECT id,UNIX_TIMESTAMP(created_at),UNIX_TIMESTAMP(updated_at) FROM `" + outside + "`.projects ORDER BY id")
	if e != nil {
		t.Fatal(e)
	}
	after := []string{}
	for rows.Next() {
		var id string
		var a, b int64
		if rows.Scan(&id, &a, &b) != nil {
			t.Fatal("scan")
		}
		after = append(after, id+":"+time.Unix(a, 0).String()+":"+time.Unix(b, 0).String())
	}
	_ = rows.Close()
	if tc.Hash(observed) != tc.Hash(after) {
		t.Fatal("out-of-target base changed")
	}
}

func TestMySQLIncomingTimestampFKRejected(t *testing.T) {
	ctx, _, base := fixture(t)
	var prefix string
	if base.QueryRow("SELECT DATABASE()").Scan(&prefix) != nil {
		t.Fatal("database")
	}
	name, outside := prefix+"_ab", prefix+"_ac"
	for _, n := range []string{name, outside} {
		if _, e := base.Exec("CREATE DATABASE `" + n + "`"); e != nil {
			t.Fatal(e)
		}
	}
	defer func() {
		_, _ = base.Exec("DROP DATABASE `" + outside + "`")
		_, _ = base.Exec("DROP DATABASE `" + name + "`")
	}()
	c, e := mysql.ParseDSN(os.Getenv("BLOG_UTC_MYSQL_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	c.DBName = name
	db, e := tc.OpenUTC(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = db.Close() }()
	if migration(t, db, 20) != nil {
		t.Fatal("migration20")
	}
	if _, e = db.Exec("ALTER TABLE abouts ADD UNIQUE KEY fixture_time_unique(updated_at)"); e != nil {
		t.Fatal(e)
	}
	proof, e := tc.VerifyLegacyDSN(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	m, e := tc.PlanLegacy(ctx, db, "incoming-fk", proof)
	if e != nil {
		t.Fatal(e)
	}
	br, e := tc.NewBackupReceipt(ctx, db, m, tc.Hash("archive"), tc.Hash("sidecar"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE TABLE `" + outside + "`.child(id INT PRIMARY KEY,stamp TIMESTAMP NOT NULL,CONSTRAINT fixture_incoming FOREIGN KEY(stamp) REFERENCES `" + name + "`.abouts(updated_at) ON UPDATE CASCADE) ENGINE=InnoDB"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO `" + outside + "`.child SELECT 1,updated_at FROM abouts"); e != nil {
		t.Fatal(e)
	}
	var before int64
	if db.QueryRow("SELECT UNIX_TIMESTAMP(stamp) FROM `"+outside+"`.child WHERE id=1").Scan(&before) != nil {
		t.Fatal("before")
	}
	if _, e = tc.PlanLegacy(ctx, db, "incoming-other", proof); e == nil {
		t.Fatal("incoming timestamp FK plan allowed")
	}
	dir := filepath.Join(t.TempDir(), "state")
	called := false
	if _, e = tc.ApplyLegacy(ctx, db, m, tc.Attempt{StateDir: dir, Backup: &br, WritersPaused: true, EnsureMetadata: func() error { called = true; return migration(t, db, 21) }}); e == nil {
		t.Fatal("incoming timestamp FK apply allowed")
	}
	if called || !tc.StateAbsent(dir) {
		t.Fatal("incoming FK changed metadata")
	}
	var v int
	var dirty bool
	if db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&v, &dirty) != nil || v != 20 || dirty {
		t.Fatal("incoming FK schema changed")
	}
	var after int64
	if db.QueryRow("SELECT UNIX_TIMESTAMP(stamp) FROM `"+outside+"`.child WHERE id=1").Scan(&after) != nil || before != after {
		t.Fatal("foreign child changed")
	}
	var parent int64
	if db.QueryRow("SELECT UNIX_TIMESTAMP(updated_at) FROM abouts").Scan(&parent) != nil || parent != before {
		t.Fatal("parent changed")
	}
}

func TestMySQLOutgoingTimestampFKRejected(t *testing.T) {
	ctx, _, base := fixture(t)
	var prefix string
	if base.QueryRow("SELECT DATABASE()").Scan(&prefix) != nil {
		t.Fatal("database")
	}
	name, outside := prefix+"_ad", prefix+"_ae"
	for _, n := range []string{name, outside} {
		if _, e := base.Exec("CREATE DATABASE `" + n + "`"); e != nil {
			t.Fatal(e)
		}
	}
	defer func() {
		_, _ = base.Exec("DROP DATABASE `" + name + "`")
		_, _ = base.Exec("DROP DATABASE `" + outside + "`")
	}()
	c, e := mysql.ParseDSN(os.Getenv("BLOG_UTC_MYSQL_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	c.DBName = name
	db, e := tc.OpenUTC(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = db.Close() }()
	if migration(t, db, 20) != nil {
		t.Fatal("migration20")
	}
	proof, e := tc.VerifyLegacyDSN(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	m, e := tc.PlanLegacy(ctx, db, "outgoing-fk", proof)
	if e != nil {
		t.Fatal(e)
	}
	br, e := tc.NewBackupReceipt(ctx, db, m, tc.Hash("archive"), tc.Hash("sidecar"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE TABLE `" + outside + "`.parent(id INT PRIMARY KEY,stamp TIMESTAMP NOT NULL UNIQUE) ENGINE=InnoDB"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO `" + outside + "`.parent SELECT 1,updated_at FROM abouts"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO `" + outside + "`.parent SELECT 2,DATE_ADD(updated_at,INTERVAL 8 HOUR) FROM abouts"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("ALTER TABLE abouts ADD CONSTRAINT fixture_outgoing FOREIGN KEY(updated_at) REFERENCES `" + outside + "`.parent(stamp)"); e != nil {
		t.Fatal(e)
	}
	var before int64
	if db.QueryRow("SELECT UNIX_TIMESTAMP(updated_at) FROM abouts").Scan(&before) != nil {
		t.Fatal("before")
	}
	if _, e = tc.PlanLegacy(ctx, db, "outgoing-other", proof); e == nil {
		t.Fatal("outgoing TS FK plan allowed")
	}
	dir := filepath.Join(t.TempDir(), "state")
	called := false
	if _, e = tc.ApplyLegacy(ctx, db, m, tc.Attempt{StateDir: dir, Backup: &br, WritersPaused: true, EnsureMetadata: func() error { called = true; return migration(t, db, 21) }}); e == nil {
		t.Fatal("outgoing TS FK apply allowed")
	}
	if called || !tc.StateAbsent(dir) {
		t.Fatal("outgoing FK changed metadata")
	}
	var v int
	var dirty bool
	if db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&v, &dirty) != nil || v != 20 || dirty {
		t.Fatal("outgoing FK schema changed")
	}
	var after, parent int64
	if db.QueryRow("SELECT UNIX_TIMESTAMP(updated_at) FROM abouts").Scan(&after) != nil || before != after {
		t.Fatal("child relation changed")
	}
	if db.QueryRow("SELECT UNIX_TIMESTAMP(stamp) FROM `"+outside+"`.parent WHERE id=1").Scan(&parent) != nil || parent != before {
		t.Fatal("external parent changed")
	}
	if db.QueryRow("SELECT UNIX_TIMESTAMP(stamp) FROM `"+outside+"`.parent WHERE id=2").Scan(&parent) != nil || parent != before+28800 {
		t.Fatal("other parent changed")
	}
}
