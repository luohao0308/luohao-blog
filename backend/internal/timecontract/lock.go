package timecontract

import (
	"context"
	"database/sql"
)

func operationLock(ctx context.Context, db *sql.DB) (func(), error) {
	c, e := db.Conn(ctx)
	if e != nil {
		return nil, ErrContract
	}
	var n int
	if c.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT(DATABASE(),':utc-contract'),0)").Scan(&n) != nil || n != 1 {
		_ = c.Close()
		return nil, ErrContract
	}
	return func() {
		_, _ = c.ExecContext(context.Background(), "SELECT RELEASE_LOCK(CONCAT(DATABASE(),':utc-contract'))")
		_ = c.Close()
	}, nil
}
