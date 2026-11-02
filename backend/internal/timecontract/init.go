package timecontract

import (
	"context"
	"database/sql"
	"time"
)

func FreshInit(ctx context.Context, db *sql.DB, dir, operation string, migrate func() error) error {
	unlock, e := operationLock(ctx, db)
	if e != nil {
		return e
	}
	defer unlock()
	var n int
	if !validOperation(operation) || db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()").Scan(&n) != nil || n != 0 {
		return ErrContract
	}
	target, e := GetTarget(ctx, db)
	if e != nil {
		return e
	}
	if !StateAbsent(dir) {
		return ErrContract
	}
	s := State{FormatVersion: 1, Target: target, Status: "pending", OperationID: operation, Kind: "fresh-init"}
	if WriteState(dir, s) != nil {
		return ErrContract
	}
	if migrate() == nil {
		snap, e := Capture(ctx, db)
		if e != nil {
			return e
		}
		h := Hash(snap)
		_, e = db.ExecContext(ctx, "INSERT INTO time_contract VALUES(1,1,'fresh-utc',0,?,?,?)", operation, h, time.Now().UTC())
		if e != nil {
			return ErrContract
		}
		s.Status = "ready"
		s.ManifestSHA256 = h
		s.ContractOperationID = operation
		s.ContractManifestSHA256 = h
		s.ReceiptSHA256 = h
		return WriteState(dir, s)
	}
	return ErrContract
}
