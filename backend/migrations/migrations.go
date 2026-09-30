// Package migrations embeds the versioned SQL migrations applied on boot.
// The SQL files live beside this package so that golang-migrate can stream
// them from the compiled binary without external assets.
package migrations

import "embed"

// FS holds the versioned SQL migrations (000001_baseline.up.sql, ...).
//
//go:embed *.sql
var FS embed.FS
