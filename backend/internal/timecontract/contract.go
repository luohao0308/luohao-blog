// Package timecontract implements the fixed MySQL UTC data contract. It never
// starts application components or implicitly changes historical timestamps.
package timecontract

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

const DefaultStateDir = "/app/data/time-contract"

var ErrContract = errors.New("time contract rejected; operator verification required")
var ErrUnknown = errors.New("time contract commit outcome unknown; reconcile read-only")

type Target struct {
	ServerUUID string `json:"server_uuid"`
	Database   string `json:"database"`
}
type State struct {
	FormatVersion          int    `json:"format_version"`
	Target                 Target `json:"target"`
	Status                 string `json:"status"`
	OperationID            string `json:"operation_id"`
	Kind                   string `json:"kind"`
	ManifestSHA256         string `json:"manifest_sha256"`
	BackupSHA256           string `json:"backup_sha256"`
	ReceiptSHA256          string `json:"receipt_sha256"`
	ContractOperationID    string `json:"contract_operation_id"`
	ContractManifestSHA256 string `json:"contract_manifest_sha256"`
	AttemptNonce           string `json:"attempt_nonce,omitempty"`
}
type Marker struct {
	Version        int    `json:"contract_version"`
	Origin         string `json:"origin"`
	Offset         int    `json:"offset_seconds"`
	OperationID    string `json:"operation_id"`
	ManifestSHA256 string `json:"manifest_sha256"`
}
type Status struct {
	Target Target  `json:"target"`
	Schema uint64  `json:"schema"`
	Dirty  bool    `json:"dirty"`
	State  *State  `json:"state"`
	Marker *Marker `json:"marker"`
	Ready  bool    `json:"ready"`
}

func Hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

var silenceOnce sync.Once

type silentDriverLogger struct{}

func (silentDriverLogger) Print(...any) {}
func OpenUTC(ctx context.Context, dsn string) (*sql.DB, error) {
	silenceOnce.Do(func() { _ = mysql.SetLogger(silentDriverLogger{}) })
	c, e := mysql.ParseDSN(dsn)
	if e != nil || dsn == "" {
		return nil, ErrContract
	}
	c.ParseTime = true
	c.Loc = time.UTC
	if c.Params == nil {
		c.Params = map[string]string{}
	}
	c.Params["time_zone"] = "'+00:00'"
	connector, e := mysql.NewConnector(c)
	if e != nil {
		return nil, ErrContract
	}
	db := sql.OpenDB(connector)
	if e = db.PingContext(ctx); e != nil {
		_ = db.Close()
		return nil, ErrContract
	}
	return db, nil
}
func GetTarget(ctx context.Context, db *sql.DB) (Target, error) {
	var t Target
	e := db.QueryRowContext(ctx, "SELECT @@server_uuid,DATABASE()").Scan(&t.ServerUUID, &t.Database)
	if e != nil || t.ServerUUID == "" || !auditedDatabaseName(t.Database) {
		return t, ErrContract
	}
	return t, nil
}
func ReadState(dir string) (*State, error) {
	if dir == "" {
		return nil, ErrContract
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
		return nil, ErrContract
	}
	p := filepath.Join(dir, "state.json")
	info, e = os.Lstat(p)
	if e != nil || !info.Mode().IsRegular() || info.Size() > 16384 || info.Mode().Perm()&0077 != 0 {
		return nil, ErrContract
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return nil, ErrContract
	}
	var s State
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF || s.FormatVersion != 1 || s.OperationID == "" || (s.Status != "ready" && s.Status != "pending") {
		return nil, ErrContract
	}
	switch s.Kind {
	case "convert", "fresh-init", "fixture", "restore":
	default:
		return nil, ErrContract
	}
	if s.Kind == "restore" && !validHash(s.AttemptNonce) {
		return nil, ErrContract
	}
	if s.Target.ServerUUID == "" || s.Target.Database == "" || !validOperation(s.OperationID) {
		return nil, ErrContract
	}
	if s.Status == "ready" && (!validHash(s.ReceiptSHA256) || !validHash(s.ContractManifestSHA256) || s.ContractOperationID == "") {
		return nil, ErrContract
	}
	return &s, nil
}
func WriteState(dir string, s State) error {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return ErrContract
	}
	i, e := os.Lstat(dir)
	if e != nil || !i.IsDir() || i.Mode().Perm()&0022 != 0 {
		return ErrContract
	}
	b, e := json.Marshal(s)
	if e != nil {
		return ErrContract
	}
	f, e := os.CreateTemp(dir, ".state-")
	if e != nil {
		return ErrContract
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil || ce != nil {
		return ErrContract
	}
	if e = os.Rename(name, filepath.Join(dir, "state.json")); e != nil {
		return ErrContract
	}
	d, e := os.Open(dir)
	if e != nil {
		return ErrContract
	}
	defer func() { _ = d.Close() }()
	if d.Sync() != nil {
		return ErrContract
	}
	return nil
}
func Inspect(ctx context.Context, db *sql.DB, dir string) (Status, error) {
	var out Status
	var e error
	out.Target, e = GetTarget(ctx, db)
	if e != nil {
		return out, e
	}
	var n int
	if e = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='schema_migrations'").Scan(&n); e != nil {
		return out, ErrContract
	}
	if n == 0 {
		return out, nil
	}
	if e = db.QueryRowContext(ctx, "SELECT version,dirty FROM schema_migrations").Scan(&out.Schema, &out.Dirty); e != nil {
		return out, ErrContract
	}
	out.State, _ = ReadState(dir)
	if out.Schema == 21 && !out.Dirty {
		var m Marker
		e = db.QueryRowContext(ctx, "SELECT contract_version,origin,offset_seconds,operation_id,manifest_sha256 FROM time_contract WHERE id=1").Scan(&m.Version, &m.Origin, &m.Offset, &m.OperationID, &m.ManifestSHA256)
		if e == nil {
			out.Marker = &m
		} else if !errors.Is(e, sql.ErrNoRows) {
			return out, ErrContract
		}
	}
	if s, m := out.State, out.Marker; s != nil && m != nil {
		out.Ready = s.Target == out.Target && s.Status == "ready" && s.ContractManifestSHA256 == m.ManifestSHA256 && s.ContractOperationID == m.OperationID && validHash(s.ReceiptSHA256) && ValidMarker(m, out.Target)
	}
	return out, nil
}
func RequireReady(ctx context.Context, db *sql.DB, dir string) error {
	s, e := ReadState(dir)
	if e != nil || s.Status != "ready" {
		return ErrContract
	}
	out, e := Inspect(ctx, db, dir)
	if e != nil || !out.Ready || out.Schema != 21 || out.Dirty {
		return ErrContract
	}
	var zone string
	if db.QueryRowContext(ctx, "SELECT @@session.time_zone").Scan(&zone) != nil || zone != "+00:00" {
		return ErrContract
	}
	again, e := ReadState(dir)
	if e != nil || Hash(s) != Hash(again) {
		return ErrContract
	}
	return nil
}

// StateAbsent distinguishes a genuinely new control path from corrupt state.
func StateAbsent(dir string) bool {
	i, e := os.Lstat(dir)
	if os.IsNotExist(e) {
		return true
	}
	if e != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm()&0022 != 0 {
		return false
	}
	_, e = os.Lstat(filepath.Join(dir, "state.json"))
	return os.IsNotExist(e)
}

func fixtureDatabase(name string) bool {
	for _, p := range []string{"website_completion_", "case_articles_test_", "utc_contract_test_", "creator_content_test_"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func validHash(v string) bool {
	if len(v) != 64 {
		return false
	}
	_, e := hex.DecodeString(v)
	return e == nil
}

func validOperation(v string) bool {
	if len(v) == 0 || len(v) > 64 {
		return false
	}
	for _, c := range v {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.:", c) {
			continue
		}
		return false
	}
	return true
}

func ValidMarker(m *Marker, t Target) bool {
	if m == nil || m.Version != 1 || !validHash(m.ManifestSHA256) || !validOperation(m.OperationID) {
		return false
	}
	switch m.Origin {
	case "legacy-cst":
		return m.Offset == 28800
	case "fresh-utc":
		return m.Offset == 0
	case "fixture":
		return m.Offset == 0 && fixtureDatabase(t.Database)
	}
	return false
}
func ResolveStateDir(configured, override string) string {
	if override != "" {
		return override
	}
	if configured != "" {
		return configured
	}
	return DefaultStateDir
}

// This operator is intentionally limited to audited simple database names;
// InnoDB dictionary paths for escaped/special identifiers are out of scope.
func auditedDatabaseName(v string) bool {
	if len(v) == 0 || len(v) > 64 || v[0] < 'a' || v[0] > 'z' {
		return false
	}
	for _, c := range v {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' {
			continue
		}
		return false
	}
	return true
}
