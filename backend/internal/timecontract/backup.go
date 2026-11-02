package timecontract

import (
	"context"
	"database/sql"
	"encoding/json"
)

type BackupReceipt struct {
	FormatVersion        int    `json:"format_version"`
	Target               Target `json:"target"`
	OperationID          string `json:"operation_id"`
	ManifestSHA256       string `json:"manifest_sha256"`
	ArchiveSHA256        string `json:"archive_sha256"`
	BackupManifestSHA256 string `json:"backup_manifest_sha256"`
	BeforeSnapshotSHA256 string `json:"before_snapshot_sha256"`
}

func BeforeSnapshot(s Snapshot) Snapshot {
	b, _ := json.Marshal(s)
	var out Snapshot
	_ = json.Unmarshal(b, &out)
	for i := range out.Tables {
		for j := range out.Tables[i].Rows {
			out.Tables[i].Rows[j].After = out.Tables[i].Rows[j].Time
		}
	}
	return out
}
func NewBackupReceipt(ctx context.Context, db *sql.DB, m Manifest, archive, sidecar string) (BackupReceipt, error) {
	r := BackupReceipt{FormatVersion: 1, Target: m.Target, OperationID: m.OperationID, ManifestSHA256: Hash(m), ArchiveSHA256: archive, BackupManifestSHA256: sidecar, BeforeSnapshotSHA256: Hash(BeforeSnapshot(m.Before))}
	current, e := Capture(ctx, db)
	if e != nil || Hash(current) != r.BeforeSnapshotSHA256 || !validHash(archive) || !validHash(sidecar) {
		return r, ErrContract
	}
	return r, nil
}
func validBackup(m Manifest, r *BackupReceipt) bool {
	return r != nil && r.FormatVersion == 1 && r.Target == m.Target && r.OperationID == m.OperationID && r.ManifestSHA256 == Hash(m) && validHash(r.ArchiveSHA256) && validHash(r.BackupManifestSHA256) && r.BeforeSnapshotSHA256 == Hash(BeforeSnapshot(m.Before))
}
