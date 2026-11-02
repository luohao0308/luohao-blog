package data

import (
	"database/sql"
	mysql "github.com/go-sql-driver/mysql"
	"os"
	"regexp"
	"testing"
	"time"
)

func timeBaselineDB(t *testing.T, zone string) *sql.DB {
	t.Helper()
	raw := os.Getenv("BLOG_TIME_MYSQL_DSN")
	if raw == "" {
		t.Skip("owned time baseline MySQL fixture required")
	}
	c, e := mysql.ParseDSN(raw)
	if e != nil || c.Net != "tcp" || c.Addr != "127.0.0.1:13329" || !regexp.MustCompile(`^time_contract_test_[a-f0-9]+$`).MatchString(c.DBName) {
		t.Fatal("refusing non-fixture time database")
	}
	c.ParseTime = true
	c.Loc = time.UTC
	if c.Params == nil {
		c.Params = map[string]string{}
	}
	c.Params["time_zone"] = "'" + zone + "'"
	db, e := sql.Open("mysql", c.FormatDSN())
	if e != nil {
		t.Fatal("fixture open failed")
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
func timeBaselineExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, e := db.Exec(q, args...); e != nil {
		t.Fatal("fixture statement failed")
	}
}
func TestTimeBaselineRoundtrip(t *testing.T) {
	old := timeBaselineDB(t, "+08:00")
	utc := timeBaselineDB(t, "+00:00")
	timeBaselineExec(t, old, "CREATE TABLE probe (id INT PRIMARY KEY, stamp TIMESTAMP NULL, expires TIMESTAMP NULL, due TIMESTAMP NULL, lease DATETIME(6) NULL, snapshot JSON NOT NULL)")
	now := time.Date(2026, 11, 2, 14, 50, 32, 0, time.UTC)
	lease := now.Add(789123 * time.Microsecond)
	snapshot := `{"published_at":"2026-11-02T14:50:32Z"}`
	timeBaselineExec(t, old, "INSERT INTO probe VALUES (1,?,?,?,?,?),(2,NULL,NULL,NULL,NULL,?)", now, now.Add(time.Hour), now.Add(-time.Second), lease, snapshot, snapshot)
	var got time.Time
	var epoch int64
	if e := old.QueryRow("SELECT stamp,UNIX_TIMESTAMP(stamp) FROM probe WHERE id=1").Scan(&got, &epoch); e != nil {
		t.Fatal(e)
	}
	if !got.Equal(now) || epoch != now.Unix()-28800 {
		t.Fatal("legacy cancellation not reproduced", got, epoch)
	}
	var beforeLease time.Time
	var beforeJSON string
	if e := utc.QueryRow("SELECT lease,CAST(snapshot AS CHAR) FROM probe WHERE id=1").Scan(&beforeLease, &beforeJSON); e != nil {
		t.Fatal(e)
	}
	// All predicates use one fixed reference instant and actual driver bindings.
	var oldExpired, oldDue int
	if e := old.QueryRow("SELECT expires<=?,due<=? FROM probe WHERE id=1", now, now).Scan(&oldExpired, &oldDue); e != nil {
		t.Fatal(e)
	}
	refs := []time.Time{now.Add(-2 * time.Second), now.Add(-time.Second), now, now.Add(time.Hour - time.Second), now.Add(time.Hour), now.Add(time.Hour + time.Second)}
	oldPredicates := make([][2]int, len(refs))
	for i, ref := range refs {
		if e := old.QueryRow("SELECT expires<=?,due<=? FROM probe WHERE id=1", ref, ref).Scan(&oldPredicates[i][0], &oldPredicates[i][1]); e != nil {
			t.Fatal(e)
		}
	}
	tx, e := utc.Begin()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec("UPDATE probe SET stamp=DATE_ADD(stamp,INTERVAL 8 HOUR),expires=DATE_ADD(expires,INTERVAL 8 HOUR),due=DATE_ADD(due,INTERVAL 8 HOUR)"); e != nil {
		_ = tx.Rollback()
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	if e = utc.QueryRow("SELECT stamp,UNIX_TIMESTAMP(stamp) FROM probe WHERE id=1").Scan(&got, &epoch); e != nil {
		t.Fatal(e)
	}
	if !got.Equal(now) || epoch != now.Unix() {
		t.Fatal("UTC instant mismatch")
	}
	var afterLease time.Time
	var afterJSON string
	var expired, due int
	if e = utc.QueryRow("SELECT lease,CAST(snapshot AS CHAR),expires<=?,due<=? FROM probe WHERE id=1", now, now).Scan(&afterLease, &afterJSON, &expired, &due); e != nil {
		t.Fatal(e)
	}
	if !beforeLease.Equal(lease) || !afterLease.Equal(lease) || beforeJSON != afterJSON || expired != oldExpired || due != oldDue || expired != 0 || due != 1 {
		t.Fatal("preservation failure")
	}
	for _, ref := range []time.Time{now.Add(time.Hour - time.Second), now.Add(time.Hour), now.Add(time.Hour + time.Second)} {
		var actual int
		if e = utc.QueryRow("SELECT expires<=? FROM probe WHERE id=1", ref).Scan(&actual); e != nil {
			t.Fatal(e)
		}
		expected := 0
		if !ref.Before(now.Add(time.Hour)) {
			expected = 1
		}
		if actual != expected {
			t.Fatal("expiry boundary")
		}
	}
	for i, ref := range refs {
		var values [2]int
		if e := utc.QueryRow("SELECT expires<=?,due<=? FROM probe WHERE id=1", ref, ref).Scan(&values[0], &values[1]); e != nil {
			t.Fatal(e)
		}
		if values != oldPredicates[i] {
			t.Fatal("deadline semantics changed")
		}
	}
	if afterLease.After(lease) || !afterLease.After(lease.Add(-time.Microsecond)) || afterLease.After(lease.Add(time.Microsecond)) {
		t.Fatal("lease equality boundary")
	}
	var nulls int
	if e = utc.QueryRow("SELECT stamp IS NULL AND expires IS NULL AND due IS NULL AND lease IS NULL FROM probe WHERE id=2").Scan(&nulls); e != nil || nulls != 1 {
		t.Fatal("NULL changed", e)
	}
	if lease.After(now) != afterLease.After(now) || lease.After(now.Add(time.Second)) != afterLease.After(now.Add(time.Second)) {
		t.Fatal("lease deadline changed")
	}
}
