package main

import (
	"context"
	"encoding/json"
	mysql "github.com/go-sql-driver/mysql"
	tc "github.com/luohao0308/luohao-blog/backend/internal/timecontract"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

func TestContentReleaseGateMySQL(t *testing.T) {
	raw := os.Getenv("BLOG_UTC_MYSQL_DSN")
	if raw == "" {
		t.Skip("owned UTC fixture required")
	}
	c, e := mysql.ParseDSN(raw)
	if e != nil || c.Net != "tcp" || c.Addr != "127.0.0.1:13330" || !regexp.MustCompile(`^utc_contract_test_[a-f0-9]+$`).MatchString(c.DBName) {
		t.Fatal("refusing non-fixture")
	}
	ctx := context.Background()
	db, e := tc.OpenUTC(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = db.Close() }()
	name := c.DBName + "_d"
	if _, e = db.Exec("CREATE DATABASE `" + name + "`"); e != nil {
		t.Fatal(e)
	}
	defer func() { _, _ = db.Exec("DROP DATABASE `" + name + "`") }()
	c.DBName = name
	target, e := tc.OpenUTC(ctx, c.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = target.Close() }()
	if _, e = target.Exec("CREATE TABLE schema_migrations(version bigint NOT NULL PRIMARY KEY,dirty bool NOT NULL)"); e != nil {
		t.Fatal(e)
	}
	if _, e = target.Exec("INSERT INTO schema_migrations VALUES(21,0)"); e != nil {
		t.Fatal(e)
	}
	var requests atomic.Int32
	es := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(200) }))
	defer es.Close()
	dir := t.TempDir()
	confDir := filepath.Join(dir, "conf")
	if os.Mkdir(confDir, 0700) != nil {
		t.Fatal("fixture")
	}
	dsn, _ := json.Marshal(c.FormatDSN())
	url, _ := json.Marshal(es.URL)
	cfg := "data:\n  database:\n    source: " + string(dsn) + "\nes:\n  addresses: [" + string(url) + "]\n  index: fixture\n"
	if os.WriteFile(filepath.Join(confDir, "fixture.yaml"), []byte(cfg), 0600) != nil {
		t.Fatal("fixture")
	}
	bundle := filepath.Join(dir, "cases.json")
	if os.WriteFile(bundle, caseStudiesFixture(t), 0600) != nil {
		t.Fatal("fixture")
	}
	stale := filepath.Join(dir, "stale-control")
	targetIdentity, te := tc.GetTarget(ctx, target)
	if te != nil {
		t.Fatal(te)
	}
	h := tc.Hash("stale")
	if tc.WriteState(stale, tc.State{FormatVersion: 1, Target: targetIdentity, Status: "ready", OperationID: "stale", Kind: "fixture", ContractOperationID: "stale", ContractManifestSHA256: h, ReceiptSHA256: h}) != nil {
		t.Fatal("state fixture")
	}
	for _, control := range []string{filepath.Join(dir, "missing-control"), stale} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestCasePreviewProcess$", "--", "-case-studies", "-bundle", bundle, "-conf", confDir, "-state-dir", control)
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "KRATOS") && !strings.HasPrefix(v, "CONTENT_RELEASE_PREVIEW_HELPER=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "CONTENT_RELEASE_PREVIEW_HELPER=1")
		if cmd.Run() == nil {
			t.Fatal("missing contract allowed")
		}
	}
	if requests.Load() != 0 {
		t.Fatal("ES contacted before time gate")
	}
}
