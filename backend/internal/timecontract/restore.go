package timecontract

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
)

type RestoreReceipt struct {
	FormatVersion   int    `json:"format_version"`
	OperationID     string `json:"operation_id"`
	Target          Target `json:"target"`
	BackupSHA256    string `json:"backup_sha256"`
	ManifestSHA256  string `json:"manifest_sha256"`
	ImportSucceeded bool   `json:"import_succeeded"`
	RedumpVerified  bool   `json:"redump_verified"`
	SnapshotSHA256  string `json:"snapshot_sha256"`
	AttemptNonce    string `json:"attempt_nonce"`
}

func RestoreBegin(ctx context.Context, db *sql.DB, dir, operation, backup, manifest string, paused bool) error {
	unlock, e := operationLock(ctx, db)
	if e != nil {
		return e
	}
	defer unlock()
	t, e := GetTarget(ctx, db)
	if e != nil || !validOperation(operation) || !validHash(backup) || !validHash(manifest) {
		return ErrContract
	}
	if s, se := ReadState(dir); se == nil {
		if s.Status != "ready" || s.Target != t {
			return ErrContract
		}
	} else if !StateAbsent(dir) {
		return ErrContract
	}
	nonce := make([]byte, 32)
	if _, e = rand.Read(nonce); e != nil {
		return ErrContract
	}
	return WriteState(dir, State{AttemptNonce: hex.EncodeToString(nonce), FormatVersion: 1, Target: t, Status: "pending", OperationID: operation, Kind: "restore", BackupSHA256: backup, ManifestSHA256: manifest})
}
func RestoreComplete(ctx context.Context, db *sql.DB, dir string, r RestoreReceipt, paused bool) error {
	unlock, e := operationLock(ctx, db)
	if e != nil {
		return e
	}
	defer unlock()
	s, e := ReadState(dir)
	if e != nil || !paused || r.FormatVersion != 1 || s.Status != "pending" || s.Kind != "restore" || s.OperationID != r.OperationID || !validHash(r.AttemptNonce) || r.AttemptNonce != s.AttemptNonce || s.Target != r.Target || s.BackupSHA256 != r.BackupSHA256 || s.ManifestSHA256 != r.ManifestSHA256 || !r.ImportSucceeded || !r.RedumpVerified {
		return ErrContract
	}
	target, e := GetTarget(ctx, db)
	if e != nil || target != s.Target {
		return ErrContract
	}
	snap, e := Capture(ctx, db)
	if e != nil || Hash(snap) != r.SnapshotSHA256 {
		return ErrContract
	}
	status, e := Inspect(ctx, db, dir)
	if e != nil || status.Schema != 21 || status.Dirty || !ValidMarker(status.Marker, status.Target) {
		return ErrContract
	}
	s.Status = "ready"
	s.ContractOperationID = status.Marker.OperationID
	s.ContractManifestSHA256 = status.Marker.ManifestSHA256
	s.ReceiptSHA256 = Hash(r)
	return WriteState(dir, *s)
}
