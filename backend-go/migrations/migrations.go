package migrations

import "embed"

// FS holds the SQL files, compiled into the program.
//
//go:embed *.sql
var FS embed.FS
