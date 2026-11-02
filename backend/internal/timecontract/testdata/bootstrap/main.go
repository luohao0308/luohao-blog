// Test harness only. go build ./... excludes testdata. The caller must first
// verify its local Docker context, owned label, and loopback publication.
package main

import (
	"context"
	"flag"
	mysql "github.com/go-sql-driver/mysql"
	tc "github.com/luohao0308/luohao-blog/backend/internal/timecontract"
	"os"
	"regexp"
	"time"
)

func main() {
	dir := flag.String("state-dir", "", "owned fixture state directory")
	key := flag.String("dsn-env", "MYSQL_MIGRATION_DSN", "fixture environment key")
	flag.Parse()
	raw := os.Getenv(*key)
	c, e := mysql.ParseDSN(raw)
	if e != nil || c.Net != "tcp" || !regexp.MustCompile(`^127\.0\.0\.1:(13316|13327|13328|13330)$`).MatchString(c.Addr) || !regexp.MustCompile(`^(website_completion_|case_articles_test_|utc_contract_test_|creator_content_test_)[a-f0-9_]+$`).MatchString(c.DBName) || *dir == "" {
		os.Exit(1)
	}
	ctx := context.Background()
	db, e := tc.OpenUTC(ctx, raw)
	if e != nil {
		os.Exit(2)
	}
	defer func() { _ = db.Close() }()
	var v int
	var dirty bool
	if db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&v, &dirty) != nil || v != 21 || dirty {
		os.Exit(3)
	}
	snap, e := tc.Capture(ctx, db)
	if e != nil {
		os.Exit(4)
	}
	h := tc.Hash(snap)
	op := "fixture-" + h[:24]
	_, e = db.Exec("INSERT INTO time_contract VALUES(1,1,'fixture',0,?,?,?)", op, h, time.Now().UTC())
	if e != nil {
		os.Exit(5)
	}
	t, e := tc.GetTarget(ctx, db)
	if e != nil {
		os.Exit(6)
	}
	s := tc.State{FormatVersion: 1, Target: t, Status: "ready", OperationID: op, Kind: "fixture", ManifestSHA256: h, ContractOperationID: op, ContractManifestSHA256: h, ReceiptSHA256: h}
	if tc.WriteState(*dir, s) != nil {
		os.Exit(7)
	}
}
