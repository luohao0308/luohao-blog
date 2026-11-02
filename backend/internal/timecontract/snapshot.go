package timecontract

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

var timestampColumns = map[string][]string{
	"abouts": {"created_at", "updated_at"}, "account_email_tokens": {"created_at", "updated_at", "expires_at"}, "article_revisions": {"created_at"}, "articles": {"created_at", "updated_at", "published_at"}, "categories": {"created_at", "updated_at"}, "collections": {"saved_at"}, "comments": {"created_at", "updated_at"}, "projects": {"created_at", "updated_at"}, "subscribers": {"created_at", "updated_at"}, "tags": {"created_at", "updated_at"}, "users": {"created_at", "updated_at"}, "wechat_identities": {"created_at"}, "wechat_sync_tasks": {"created_at", "updated_at", "next_attempt_at"},
}

type Row struct {
	ID        string `json:"id"`
	Time      string `json:"time_digest"`
	After     string `json:"after_time_digest"`
	Protected string `json:"protected_digest"`
}
type Table struct {
	Name string `json:"name"`
	Rows []Row  `json:"rows"`
}
type Snapshot struct {
	SchemaDigest string  `json:"schema_digest"`
	Tables       []Table `json:"tables"`
}
type Manifest struct {
	Evidence    LegacyEvidence `json:"legacy_evidence"`
	Version     int            `json:"format_version"`
	Target      Target         `json:"target"`
	OperationID string         `json:"operation_id"`
	Offset      int            `json:"offset_seconds"`
	Before      Snapshot       `json:"before"`
}
type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func capture(ctx context.Context, q queryer, offset int, lock bool) (Snapshot, error) {
	var out Snapshot
	var bad int
	for _, check := range []string{"SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND (TABLE_TYPE<>'BASE TABLE' OR ENGINE IS NULL OR ENGINE<>'InnoDB')", "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()", "SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA=DATABASE()"} {
		r, e := q.QueryContext(ctx, check)
		if e != nil {
			return out, ErrContract
		}
		if !r.Next() || r.Scan(&bad) != nil || bad != 0 {
			_ = r.Close()
			return out, ErrContract
		}
		_ = r.Close()
	}
	if !noTimestampReferences(ctx, q) {
		return out, ErrContract
	}
	rows, e := q.QueryContext(ctx, "SELECT TABLE_NAME,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,COALESCE(COLUMN_DEFAULT,'NULL'),EXTRA,COALESCE(CHARACTER_SET_NAME,''),COALESCE(COLLATION_NAME,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME NOT IN ('schema_migrations','time_contract') ORDER BY TABLE_NAME,ORDINAL_POSITION")
	if e != nil {
		return out, ErrContract
	}
	cols := map[string][]string{}
	types := map[string]string{}
	schema := []string{}
	seenTS := map[string][]string{}
	for rows.Next() {
		var t, c, typ, nul, def, extra, charset, collation string
		if rows.Scan(&t, &c, &typ, &nul, &def, &extra, &charset, &collation) != nil {
			_ = rows.Close()
			return out, ErrContract
		}
		if !identifier(t) || !identifier(c) || strings.Contains(strings.ToLower(extra), "on update") {
			_ = rows.Close()
			return out, ErrContract
		}
		cols[t] = append(cols[t], c)
		types[t+"."+c] = typ
		schema = append(schema, strings.Join([]string{t, c, typ, nul, def, extra, charset, collation}, "|"))
		if strings.HasPrefix(typ, "timestamp") {
			seenTS[t] = append(seenTS[t], c)
		}
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil || Hash(seenTS) != Hash(timestampColumns) {
		return out, ErrContract
	}

	objectRows, oe := q.QueryContext(ctx, "SELECT TABLE_NAME,TABLE_TYPE,COALESCE(ENGINE,'NULL') FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME NOT IN ('schema_migrations','time_contract') ORDER BY TABLE_NAME")
	if oe != nil {
		return out, ErrContract
	}
	for objectRows.Next() {
		var n, kind, engine string
		if objectRows.Scan(&n, &kind, &engine) != nil {
			_ = objectRows.Close()
			return out, ErrContract
		}
		schema = append(schema, strings.Join([]string{n, kind, engine}, "|"))
	}
	oe = objectRows.Err()
	_ = objectRows.Close()
	if oe != nil {
		return out, ErrContract
	}

	for _, query := range []string{"SELECT TABLE_NAME,INDEX_NAME,NON_UNIQUE,SEQ_IN_INDEX,COLUMN_NAME,COALESCE(SUB_PART,0) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME<>'time_contract' ORDER BY TABLE_NAME,INDEX_NAME,SEQ_IN_INDEX", "SELECT TABLE_NAME,CONSTRAINT_NAME,REFERENCED_TABLE_NAME,UPDATE_RULE,DELETE_RULE FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() ORDER BY TABLE_NAME,CONSTRAINT_NAME", "SELECT TABLE_NAME,CONSTRAINT_NAME,COLUMN_NAME,ORDINAL_POSITION,COALESCE(REFERENCED_TABLE_NAME,''),COALESCE(REFERENCED_COLUMN_NAME,'') FROM information_schema.KEY_COLUMN_USAGE WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME<>'time_contract' ORDER BY TABLE_NAME,CONSTRAINT_NAME,ORDINAL_POSITION"} {
		rr, e := q.QueryContext(ctx, query)
		if e != nil {
			return out, ErrContract
		}
		cc, e := rr.Columns()
		if e != nil {
			_ = rr.Close()
			return out, ErrContract
		}
		for rr.Next() {
			values := make([]sql.NullString, len(cc))
			args := make([]any, len(cc))
			for i := range values {
				args[i] = &values[i]
			}
			if rr.Scan(args...) != nil {
				_ = rr.Close()
				return out, ErrContract
			}
			row := []string{}
			for _, v := range values {
				row = append(row, v.String)
			}
			schema = append(schema, strings.Join(row, "|"))
		}
		e = rr.Err()
		_ = rr.Close()
		if e != nil {
			return out, ErrContract
		}
	}
	out.SchemaDigest = Hash(schema)
	if len(cols) != 15 {
		return out, ErrContract
	}
	names := []string{}
	for t := range cols {
		names = append(names, t)
	}
	sort.Strings(names)
	for _, t := range names {
		if _, ok := timestampColumns[t]; !ok && t != "wechat_comment_checks" && t != "article_tags" {
			return out, ErrContract
		}
		ts, before, after, protect := timestampColumns[t], []string{}, []string{}, []string{}
		isTS := map[string]bool{}
		for _, c := range ts {
			if types[t+"."+c] != "timestamp" {
				return out, ErrContract
			}
			if offset > 0 {
				rr, e := q.QueryContext(ctx, "SELECT COUNT(*) FROM `"+t+"` WHERE `"+c+"` IS NOT NULL AND (UNIX_TIMESTAMP(`"+c+"`)=0 OR UNIX_TIMESTAMP(`"+c+"`)>2147483647-28800)")
				if e != nil {
					return out, ErrContract
				}
				if !rr.Next() || rr.Scan(&bad) != nil || bad != 0 {
					_ = rr.Close()
					return out, ErrContract
				}
				_ = rr.Close()
			}

			isTS[c] = true
			before = append(before, "IF(`"+c+"` IS NULL,NULL,CAST(UNIX_TIMESTAMP(`"+c+"`) AS UNSIGNED))")
			after = append(after, fmt.Sprintf("IF(`%s` IS NULL,NULL,CAST(UNIX_TIMESTAMP(`%s`)+%d AS UNSIGNED))", c, c, offset))
		}
		for _, c := range cols[t] {
			if !isTS[c] {
				protect = append(protect, "`"+c+"`")
			}
		}
		id := "CAST(`id` AS CHAR)"
		order := "`id`"
		if t == "article_tags" {
			id = "CAST(JSON_ARRAY(article_id,tag_id) AS CHAR)"
			order = "article_id,tag_id"
		}
		digest := func(v []string) string { return "SHA2(CAST(JSON_ARRAY(" + strings.Join(v, ",") + ") AS CHAR),256)" }
		query := "SELECT " + id + "," + digest(before) + "," + digest(after) + "," + digest(protect) + " FROM `" + t + "` ORDER BY " + order
		if lock {
			query += " FOR UPDATE"
		}
		rr, e := q.QueryContext(ctx, query)
		if e != nil {
			return out, ErrContract
		}
		table := Table{Name: t, Rows: []Row{}}
		for rr.Next() {
			var r Row
			if rr.Scan(&r.ID, &r.Time, &r.After, &r.Protected) != nil {
				_ = rr.Close()
				return out, ErrContract
			}
			table.Rows = append(table.Rows, r)
		}
		e = rr.Err()
		_ = rr.Close()
		if e != nil {
			return out, ErrContract
		}
		out.Tables = append(out.Tables, table)
	}
	return out, nil
}
func identifier(v string) bool {
	if v == "" {
		return false
	}
	for _, c := range v {
		if c != '_' && (c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}
func Capture(ctx context.Context, db *sql.DB) (Snapshot, error) { return capture(ctx, db, 0, false) }
func PlanLegacy(ctx context.Context, db *sql.DB, operation string, proof ...LegacyEvidence) (Manifest, error) {
	m := Manifest{Version: 1, OperationID: operation, Offset: 28800}
	var version int
	var dirty bool
	if db.QueryRowContext(ctx, "SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty) != nil || version != 20 || dirty {
		return m, ErrContract
	}
	var e error
	if !legacyMetadataAbsent(ctx, db) {
		return m, ErrContract
	}
	if !validOperation(operation) {
		return m, ErrContract
	}
	m.Target, e = GetTarget(ctx, db)
	if e != nil {
		return m, e
	}
	if len(proof) != 1 || proof[0].Target != m.Target || proof[0].SessionOffset != 28800 || proof[0].Samples < 1 {
		return m, ErrContract
	}
	m.Evidence = proof[0]
	m.Before, e = capture(ctx, db, 28800, false)
	return m, e
}

func legacyMetadataAbsent(ctx context.Context, q queryer) bool {
	r, e := q.QueryContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='time_contract'")
	if e != nil {
		return false
	}
	defer func() { _ = r.Close() }()
	var n int
	return r.Next() && r.Scan(&n) == nil && n == 0
}

// The InnoDB dictionary requires PROCESS and includes referencing children in
// other schemas. Timestamp columns cannot participate in either direction.
// A permissions error is rejection, never evidence of zero FKs.
func noTimestampReferences(ctx context.Context, q queryer) bool {
	conditions := []string{}
	args := []any{}
	for table, columns := range timestampColumns {
		quoted := []string{}
		for _, c := range columns {
			quoted = append(quoted, "'"+c+"'")
		}
		conditions = append(conditions, "(f.REF_NAME=CONCAT(DATABASE(),?) AND c.REF_COL_NAME IN ("+strings.Join(quoted, ",")+"))")
		args = append(args, "/"+table)
		conditions = append(conditions, "(f.FOR_NAME=CONCAT(DATABASE(),?) AND c.FOR_COL_NAME IN ("+strings.Join(quoted, ",")+"))")
		args = append(args, "/"+table)
	}
	r, e := q.QueryContext(ctx, "SELECT COUNT(*) FROM information_schema.INNODB_FOREIGN f JOIN information_schema.INNODB_FOREIGN_COLS c ON c.ID=f.ID WHERE "+strings.Join(conditions, " OR "), args...)
	if e != nil {
		return false
	}
	defer func() { _ = r.Close() }()
	var n int
	return r.Next() && r.Scan(&n) == nil && n == 0
}
