// Package templates holds the HTML templates, built into the binary.
package templates

import "embed"

// FS holds every template: components/, pages/ and responses/.
//
//go:embed */*.html
var FS embed.FS

// Patterns are the template files in FS, for template.ParseFS.
var Patterns = []string{"*/*.html"}
