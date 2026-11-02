package timecontract

import (
	"context"
	"database/sql"
	mysql "github.com/go-sql-driver/mysql"
	"time"
)

type LegacyEvidence struct {
	Target        Target `json:"target"`
	SessionOffset int    `json:"session_offset_seconds"`
	Samples       int    `json:"validated_time_cells"`
}

// VerifyLegacyDSN reads the original driver/session mapping, never mutates it.
// It verifies each existing TIMESTAMP cell's Go interpretation rather than
// guessing an offset from a timezone abbreviation or schema version.
func VerifyLegacyDSN(ctx context.Context, raw string) (LegacyEvidence, error) {
	silenceOnce.Do(func() { _ = mysql.SetLogger(silentDriverLogger{}) })
	var out LegacyEvidence
	c, e := mysql.ParseDSN(raw)
	if e != nil || !c.ParseTime {
		return out, ErrContract
	}
	db, e := sql.Open("mysql", raw)
	if e != nil {
		return out, ErrContract
	}
	defer func() { _ = db.Close() }()
	out.Target, e = GetTarget(ctx, db)
	if e != nil {
		return out, e
	}
	if db.QueryRowContext(ctx, "SELECT TIMESTAMPDIFF(SECOND,UTC_TIMESTAMP(),NOW())").Scan(&out.SessionOffset) != nil || out.SessionOffset != 28800 {
		return out, ErrContract
	}
	for table, columns := range timestampColumns {
		for _, col := range columns {
			r, e := db.QueryContext(ctx, "SELECT `"+col+"`,UNIX_TIMESTAMP(`"+col+"`) FROM `"+table+"` WHERE `"+col+"` IS NOT NULL")
			if e != nil {
				return out, ErrContract
			}
			for r.Next() {
				var stamp time.Time
				var epoch int64
				if r.Scan(&stamp, &epoch) != nil || stamp.Unix()-epoch != 28800 {
					_ = r.Close()
					return out, ErrContract
				}
				out.Samples++
			}
			e = r.Err()
			_ = r.Close()
			if e != nil {
				return out, ErrContract
			}
		}
	}
	if out.Samples == 0 {
		return out, ErrContract
	}
	return out, nil
}
