package timecontract

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type Attempt struct {
	StateDir       string
	BackupSHA256   string
	WritersPaused  bool
	Resume         bool
	EnsureMetadata func() error
	BeforeCommit   func() error
	Commit         func(*sql.Tx) error
	Backup         *BackupReceipt
}
type Result struct {
	Outcome        string `json:"outcome"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

func expected(s Snapshot) Snapshot {
	b, _ := json.Marshal(s)
	var out Snapshot
	_ = json.Unmarshal(b, &out)
	for i := range out.Tables {
		for j := range out.Tables[i].Rows {
			r := &out.Tables[i].Rows[j]
			r.Time = r.After
			r.After = r.Time
		}
	}
	return out
}
func Reconcile(ctx context.Context, db *sql.DB, m Manifest, a Attempt) (Result, error) {
	h := Hash(m)
	out := Result{Outcome: "unknown", ManifestSHA256: h}
	status, e := Inspect(ctx, db, a.StateDir)
	if e != nil {
		return out, e
	}
	if status.Target != m.Target {
		return out, ErrContract
	}
	if status.Ready && status.Marker.ManifestSHA256 == h {
		out.Outcome = "already_complete"
		return out, nil
	}
	now, e := capture(ctx, db, 0, false)
	if e != nil {
		return out, e
	}
	if status.Marker == nil {
		raw, _ := json.Marshal(m.Before)
		var before Snapshot
		_ = json.Unmarshal(raw, &before)
		for i := range before.Tables {
			for j := range before.Tables[i].Rows {
				before.Tables[i].Rows[j].After = before.Tables[i].Rows[j].Time
			}
		}
		if Hash(now) == Hash(before) {
			out.Outcome = "not_committed"
		}
		return out, nil
	}
	if status.Marker.OperationID == m.OperationID && status.Marker.ManifestSHA256 == h && Hash(now) == Hash(expected(m.Before)) {
		out.Outcome = "committed_verified"
	}
	return out, nil
}
func Activate(ctx context.Context, db *sql.DB, m Manifest, a Attempt) (Result, error) {
	unlock, e := operationLock(ctx, db)
	if e != nil {
		return Result{}, e
	}
	defer unlock()
	return activateLocked(ctx, db, m, a)
}
func activateLocked(ctx context.Context, db *sql.DB, m Manifest, a Attempt) (Result, error) {
	r, e := Reconcile(ctx, db, m, a)
	if e != nil || r.Outcome != "committed_verified" || !a.WritersPaused {
		return r, ErrContract
	}
	s, e := ReadState(a.StateDir)
	if e != nil || s.Status != "pending" || s.OperationID != m.OperationID || s.ManifestSHA256 != Hash(m) || s.Kind != "convert" || s.Target != m.Target {
		return r, ErrContract
	}
	backup := a.BackupSHA256
	if a.Backup != nil {
		backup = a.Backup.ArchiveSHA256
	}
	if !validHash(backup) || backup != s.BackupSHA256 {
		return r, ErrContract
	}
	s.Status = "ready"
	s.ContractOperationID = m.OperationID
	s.ContractManifestSHA256 = Hash(m)
	s.ReceiptSHA256 = Hash(r)
	if WriteState(a.StateDir, *s) != nil {
		return r, ErrContract
	}
	return r, nil
}
func ApplyLegacy(ctx context.Context, db *sql.DB, m Manifest, a Attempt) (Result, error) {
	h := Hash(m)
	out := Result{ManifestSHA256: h}
	if m.Evidence.Target != m.Target || m.Evidence.SessionOffset != 28800 || m.Evidence.Samples < 1 || m.Version != 1 || m.Offset != 28800 || !validOperation(m.OperationID) || !a.WritersPaused || !validBackup(m, a.Backup) {
		return out, ErrContract
	}
	a.BackupSHA256 = a.Backup.ArchiveSHA256
	t, e := GetTarget(ctx, db)
	if e != nil || t != m.Target {
		return out, ErrContract
	}
	st, e := Inspect(ctx, db, a.StateDir)
	if e != nil {
		return out, e
	}
	if st.Ready && st.Marker.ManifestSHA256 == h {
		return Result{"already_complete", h}, nil
	}
	if st.Dirty || st.Schema != 20 && st.Schema != 21 {
		return out, ErrContract
	}
	if st.Schema == 20 && !legacyMetadataAbsent(ctx, db) {
		return out, ErrContract
	}
	conn, e := db.Conn(ctx)
	if e != nil {
		return out, ErrContract
	}
	defer func() { _ = conn.Close() }()
	var locked int
	if conn.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT(DATABASE(),':utc-contract'),0)").Scan(&locked) != nil || locked != 1 {
		return out, ErrContract
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(CONCAT(DATABASE(),':utc-contract'))")
	}()
	var currentVersion uint64
	var currentDirty bool
	if conn.QueryRowContext(ctx, "SELECT version,dirty FROM schema_migrations").Scan(&currentVersion, &currentDirty) != nil || currentDirty || currentVersion != st.Schema {
		return out, ErrContract
	}
	if currentVersion == 21 {
		var markers int
		if conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM time_contract").Scan(&markers) != nil || markers != 0 {
			return out, ErrContract
		}
	}
	if st.Schema == 20 && !legacyMetadataAbsent(ctx, conn) {
		return out, ErrContract
	}
	s, se := ReadState(a.StateDir)
	if se == nil {
		if !a.Resume || s.Status != "pending" || s.Kind != "convert" || s.OperationID != m.OperationID || s.ManifestSHA256 != h || s.Target != t || s.BackupSHA256 != a.BackupSHA256 {
			return out, ErrContract
		}
	} else if st.Schema != 20 || !StateAbsent(a.StateDir) {
		return out, ErrContract
	}
	if st.Marker != nil {
		return out, ErrContract
	}
	preflight, pe := capture(ctx, conn, 28800, false)
	if pe != nil || Hash(preflight) != Hash(m.Before) {
		return out, ErrContract
	}
	state := State{FormatVersion: 1, Target: t, Status: "pending", OperationID: m.OperationID, Kind: "convert", ManifestSHA256: h, BackupSHA256: a.BackupSHA256}
	if WriteState(a.StateDir, state) != nil {
		return out, ErrContract
	}
	if st.Schema == 20 {
		if a.EnsureMetadata == nil || a.EnsureMetadata() != nil {
			return out, ErrContract
		}
	}
	tx, e := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return out, ErrContract
	}
	defer func() { _ = tx.Rollback() }()
	before, e := capture(ctx, tx, 28800, true)
	if e != nil || Hash(before) != Hash(m.Before) {
		return out, ErrContract
	}
	for _, table := range before.Tables {
		cs := timestampColumns[table.Name]
		if len(cs) == 0 {
			continue
		}
		sets := []string{}
		for _, c := range cs {
			sets = append(sets, "`"+c+"`=DATE_ADD(`"+c+"`,INTERVAL 8 HOUR)")
		}
		if _, e = tx.ExecContext(ctx, "UPDATE `"+table.Name+"` SET "+join(sets)); e != nil {
			return out, ErrContract
		}
	}
	after, e := capture(ctx, tx, 0, false)
	if e != nil || Hash(after) != Hash(expected(m.Before)) {
		return out, ErrContract
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO time_contract(id,contract_version,origin,offset_seconds,operation_id,manifest_sha256,completed_at) VALUES(1,1,'legacy-cst',28800,?,?,?)", m.OperationID, h, time.Now().UTC())
	if e != nil {
		return out, ErrContract
	}
	if a.BeforeCommit != nil {
		if a.BeforeCommit() != nil {
			return out, ErrContract
		}
	}
	commit := tx.Commit
	if a.Commit != nil {
		commit = func() error { return a.Commit(tx) }
	}
	if commit() != nil {
		return Result{"outcome_unknown", h}, ErrUnknown
	}
	return activateLocked(ctx, db, m, a)
}
func join(v []string) string {
	out := ""
	for i, s := range v {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}
