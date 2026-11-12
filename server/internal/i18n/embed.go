// Package i18n holds the translation files, built into the binary.
package i18n

import "embed"

// FS holds one <lang>.json file per language.
//
//go:embed *.json
var FS embed.FS
