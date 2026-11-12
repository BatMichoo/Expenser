// Package seeds holds the development seed data (applied with SEED_DEV_DATA).
package seeds

import "embed"

//go:embed *.sql
var FS embed.FS
