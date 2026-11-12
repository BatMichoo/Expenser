package migrations

import "embed"

// SQL holds the SQL migrations. The Go ones register themselves with goose
// when this package is imported.
//
//go:embed *.sql
var SQL embed.FS
