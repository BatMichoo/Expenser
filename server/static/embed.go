// Package static holds the stylesheets, scripts and icons served under
// /static, built into the binary.
package static

import "embed"

//go:embed css js favicon.png
var FS embed.FS
