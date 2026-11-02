// Command time-contract is the explicit UTC maintenance operator. No application
// components are initialized here; all diagnostics intentionally omit SQL/DSNs.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/env"
	"github.com/go-kratos/kratos/v3/config/file"
	klog "github.com/go-kratos/kratos/v3/log"
	"github.com/golang-migrate/migrate/v4"
	mm "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	tc "github.com/luohao0308/luohao-blog/backend/internal/timecontract"
	"github.com/luohao0308/luohao-blog/backend/migrations"
	"io"
	"log"
	"os"
	"time"
)

func migrateMetadata(db *sql.DB) error {
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
	e = m.Migrate(21)
	if e == migrate.ErrNoChange {
		return nil
	}
	return e
}
func read(path string, v any) error {
	f, e := os.Open(path)
	if e != nil {
		return tc.ErrContract
	}
	defer func() { _ = f.Close() }()
	d := json.NewDecoder(io.LimitReader(f, 8<<20))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		return tc.ErrContract
	}
	return nil
}
func run() error {
	backupFile := flag.String("backup-file", "", "verified gzip backup archive")
	backupManifest := flag.String("backup-manifest", "", "exact dump sidecar")
	mode := flag.String("mode", "plan", "fixed operator mode")
	confDir := flag.String("conf", "./configs", "config directory")
	stateDir := flag.String("state-dir", "", "persistent independent state directory")
	manifestPath := flag.String("manifest", "", "exact manifest")
	manifestHash := flag.String("manifest-sha256", "", "manifest canonical SHA256")
	backup := flag.String("backup-sha256", "", "exact archive SHA256")
	backupReceipt := flag.String("backup-receipt", "", "backup receipt")
	receiptPath := flag.String("verification-receipt", "", "verified restore receipt")
	operation := flag.String("operation-id", "", "operation identifier")
	paused := flag.Bool("writers-paused", false, "operator confirms all writers paused")
	resume := flag.Bool("resume", false, "explicit original attempt recovery")
	flag.Parse()
	if flag.NArg() != 0 {
		return tc.ErrContract
	}
	switch *mode {
	case "status", "snapshot", "plan", "fresh-init", "restore-begin", "restore-complete", "apply", "reconcile", "activate", "backup-receipt":
	default:
		return tc.ErrContract
	}
	log.SetOutput(io.Discard)
	klog.SetDefault(klog.NewLogger(klog.NewHandler(klog.WithWriter(io.Discard))))
	c := config.New(config.WithSource(file.NewSource(*confDir), env.NewSource("KRATOS")))
	defer func() { _ = c.Close() }()
	if c.Load() != nil {
		return tc.ErrContract
	}
	var bc conf.Bootstrap
	if c.Scan(&bc) != nil {
		return tc.ErrContract
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	*stateDir = tc.ResolveStateDir(bc.GetData().GetDatabase().GetTimeContractStateDir(), *stateDir)
	db, e := tc.OpenUTC(ctx, bc.GetData().GetDatabase().GetSource())
	if e != nil {
		return e
	}
	defer func() { _ = db.Close() }()
	emit := func(v any) error { return json.NewEncoder(os.Stdout).Encode(v) }
	switch *mode {
	case "status":
		s, e := tc.Inspect(ctx, db, *stateDir)
		if e != nil {
			return e
		}
		return emit(s)
	case "snapshot":
		s, e := tc.Capture(ctx, db)
		if e != nil {
			return e
		}
		t, e := tc.GetTarget(ctx, db)
		if e != nil {
			return e
		}
		return emit(map[string]any{"target": t, "snapshot": s, "snapshot_sha256": tc.Hash(s)})
	case "plan":
		proof, e := tc.VerifyLegacyDSN(ctx, bc.GetData().GetDatabase().GetSource())
		if e != nil {
			return e
		}
		m, e := tc.PlanLegacy(ctx, db, *operation, proof)
		if e != nil {
			return e
		}
		raw, e := json.Marshal(m)
		if e != nil {
			return tc.ErrContract
		}
		_, e = os.Stdout.Write(raw)
		return e
	case "fresh-init":
		if !*paused {
			return tc.ErrContract
		}
		if e = tc.FreshInit(ctx, db, *stateDir, *operation, func() error { return migrateMetadata(db) }); e != nil {
			return e
		}
		return emit(map[string]bool{"ready": true})
	case "restore-begin":
		if e = tc.RestoreBegin(ctx, db, *stateDir, *operation, *backup, *manifestHash, *paused); e != nil {
			return e
		}
		t, e := tc.GetTarget(ctx, db)
		if e != nil {
			return e
		}
		pending, e := tc.ReadState(*stateDir)
		if e != nil {
			return e
		}
		return emit(map[string]any{"target": t, "ready": false, "attempt_nonce": pending.AttemptNonce})
	case "restore-complete":
		var r tc.RestoreReceipt
		if read(*receiptPath, &r) != nil {
			return tc.ErrContract
		}
		if e = tc.RestoreComplete(ctx, db, *stateDir, r, *paused); e != nil {
			return e
		}
		return emit(map[string]bool{"ready": true})
	case "apply", "reconcile", "activate", "backup-receipt":
		var m tc.Manifest
		rawManifest, re := os.ReadFile(*manifestPath)
		var canonical []byte
		if re != nil || read(*manifestPath, &m) != nil || tc.Hash(m) != *manifestHash {
			return tc.ErrContract
		}

		canonical, _ = json.Marshal(m)
		if !bytes.Equal(rawManifest, canonical) {
			return tc.ErrContract
		}
		if *mode == "backup-receipt" {
			if !*paused {
				return tc.ErrContract
			}
			ah, mh, e := backupArtifacts(*backupFile, *backupManifest)
			if e != nil {
				return e
			}
			r, e := tc.NewBackupReceipt(ctx, db, m, ah, mh)
			if e != nil {
				return e
			}
			return emit(r)
		}
		var verifiedBackup *tc.BackupReceipt
		if *mode == "apply" || (*mode == "activate" && *backupReceipt != "") {
			var r tc.BackupReceipt
			if read(*backupReceipt, &r) != nil {
				return tc.ErrContract
			}
			ah, mh, e := backupArtifacts(*backupFile, *backupManifest)
			if e != nil || ah != r.ArchiveSHA256 || mh != r.BackupManifestSHA256 {
				return tc.ErrContract
			}
			verifiedBackup = &r
		}
		a := tc.Attempt{Backup: verifiedBackup, StateDir: *stateDir, BackupSHA256: *backup, WritersPaused: *paused, Resume: *resume, EnsureMetadata: func() error { return migrateMetadata(db) }}
		var r tc.Result
		switch *mode {
		case "apply":
			r, e = tc.ApplyLegacy(ctx, db, m, a)
		case "activate":
			r, e = tc.Activate(ctx, db, m, a)
		default:
			r, e = tc.Reconcile(ctx, db, m, a)
		}
		if e != nil {
			return e
		}
		return emit(r)
	}
	return tc.ErrContract
}
func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "time-contract: rejected or outcome unknown; stopped, reconcile read-only")
		os.Exit(1)
	}
}
