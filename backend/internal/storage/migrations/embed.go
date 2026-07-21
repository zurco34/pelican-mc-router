package migrations

import "embed"

// Files contains all embedded SQL migration files.
//
//go:embed *.sql
var Files embed.FS
