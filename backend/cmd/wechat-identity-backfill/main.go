// wechat-identity-backfill copies explicitly identified legacy bindings only.
// It does not migrate schemas, contact WeChat, or start the application.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/env"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
)

func run() error {
	configDir := flag.String("conf", "./configs", "private server configuration directory")
	appID := flag.String("legacy-app-id", "", "verified application that issued legacy OpenIDs (required)")
	apply := flag.Bool("apply", false, "commit backfill; default is a rolled-back dry run")
	paused := flag.Bool("writers-paused", false, "acknowledge all old and new identity writers have been stopped/disabled")
	flag.Parse()
	if *appID == "" || (*apply && !*paused) {
		return fmt.Errorf("explicit legacy AppID and, for apply, writers-paused acknowledgment required")
	}
	// Configuration/storage errors can contain DSNs; only count-only success or
	// fixed failure messages are exposed by this command.
	log.SetDefault(log.NewLogger(slog.NewTextHandler(io.Discard, nil)))
	c := config.New(config.WithSource(file.NewSource(*configDir), env.NewSource("KRATOS")))
	defer func() { _ = c.Close() }()
	if err := c.Load(); err != nil {
		return fmt.Errorf("configuration load failed")
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		return fmt.Errorf("configuration scan failed")
	}
	db, err := sql.Open("mysql", bc.GetData().GetDatabase().GetSource())
	if err != nil {
		return fmt.Errorf("database connection failed")
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	defer func() { _ = client.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var version uint
	var dirty bool
	if err := db.QueryRowContext(ctx, "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil || version != 16 || dirty {
		return fmt.Errorf("clean schema version 16 required; this tool does not run migrations")
	}
	report, err := data.BackfillWechatIdentities(ctx, client, *appID, *apply, *paused)
	if err != nil {
		return safeBackfillError(err)
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Do not expose DSNs, account identifiers, or a false rollback claim after a
// lost COMMIT acknowledgment. Only explicit outcome categories leave the CLI.
func safeBackfillError(err error) error {
	if errors.Is(err, data.ErrWechatBackfillCommitUnknown) {
		return errors.New("backfill_commit_outcome_unknown: keep identity writers paused and recheck database state before retrying")
	}
	if errors.Is(err, biz.ErrUserWechatConflict) {
		return errors.New("backfill_identity_conflict: inspect ownership before retrying")
	}
	return errors.New("backfill_storage_or_validation_failure: recheck database state before retrying")
}
