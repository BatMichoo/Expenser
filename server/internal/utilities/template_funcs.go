package utilities

import (
	"bytes"
	"expenser/internal/groceryunits"
	"expenser/internal/version"
	"fmt"
	"html/template"
	"io/fs"
	"math"
	"strconv"
	"strings"
	"time"
)

// ParseTemplates parses the app's templates with TemplateFuncs, plus two
// functions that need the parsed set itself:
//   - include renders a template by name, so the layout can show whichever
//     page a handler chose without a list of page names;
//   - hasTemplate reports whether a template with that name exists.
func ParseTemplates(fsys fs.FS, patterns ...string) (*template.Template, error) {
	var set *template.Template
	pages := template.FuncMap{
		"include": func(name string, data any) (template.HTML, error) {
			var b bytes.Buffer
			if err := set.ExecuteTemplate(&b, name, data); err != nil {
				return "", err
			}
			// Safe: the output was escaped by html/template as it rendered.
			return template.HTML(b.String()), nil
		},
		"hasTemplate": func(name string) bool { return name != "" && set.Lookup(name) != nil },
	}
	set, err := template.New("").Funcs(TemplateFuncs()).Funcs(pages).ParseFS(fsys, patterns...)
	return set, err
}

// assetVersion changes on every start (so on every deploy) and is appended to
// stylesheet and script URLs, so browsers do not keep using a cached copy of
// an older version.
var assetVersion = strconv.FormatInt(time.Now().Unix(), 36)

// TemplateFuncs returns the functions available to every HTML template.
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		// metadataDetails is MetadataDetails: an expense's metadata as text.
		"metadataDetails": MetadataDetails,
		// deref reads an optional amount (callers check it is set).
		"deref": func(v *float64) float64 {
			if v == nil {
				return 0
			}
			return *v
		},
		"derefi": func(v *int) int {
			if v == nil {
				return 0
			}
			return *v
		},
		"negi": func(v int) int {
			return -v
		},
		"list": func(items ...string) []string {
			return items
		},
		"neg": func(v float64) float64 {
			return -v
		},
		// pct is part as a percentage of whole, clamped to 0–100 for bars.
		"pct": func(part, whole float64) float64 {
			if whole <= 0 {
				return 0
			}
			return math.Max(0, math.Min(100, part/whole*100))
		},
		// asset is the URL of a static file with the current asset version.
		"asset": func(path string) string {
			return "/static/" + path + "?v=" + assetVersion
		},
		// appVersion is the running build's version, shown in the footer.
		"appVersion": func() string { return "v" + version.Version },
		// sizeValue prints a pack size for a number input: rounded to the
		// column's 4 decimals, without float noise or trailing zeros.
		"sizeValue": func(v float64) string {
			return strconv.FormatFloat(groceryunits.Round4(v), 'f', -1, 64)
		},
		"abs":   math.Abs,
		"sub":   func(a, b float64) float64 { return a - b },
		"money": Money,
		// moneyValue is MoneyValue: an amount for a form input ("12.50").
		"moneyValue": MoneyValue,
		"rough":      RoughAmount,
		"contains": func(slice interface{}, item string) bool {
			s, ok := slice.([]string)
			if !ok {
				return false
			}
			for _, val := range s {
				if val == item {
					return true
				}
			}
			return false
		},
		"T": func(lang, key string) string {
			return T(lang, key)
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"emptySlice": func() []string {
			return []string{}
		},
		"float64": func(v interface{}) float64 {
			switch i := v.(type) {
			case float64:
				return i
			case int64:
				return float64(i)
			case int:
				return float64(i)
			default:
				return 0.0
			}
		},
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
	}
}

// RoughAmount formats v as a short whole number for chart labels, using a
// "k" suffix from 1000 up: 87.4 -> "87", 1234 -> "1.2k", 12500 -> "13k".
// Zero gives "" so empty months stay unlabelled.
func RoughAmount(v float64) string {
	switch {
	case v == 0:
		return ""
	case math.Abs(v) < 999.5:
		return fmt.Sprintf("%.0f", math.Round(v))
	case math.Abs(v) < 9950:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", math.Round(v/100)/10), ".0") + "k"
	default:
		return fmt.Sprintf("%.0fk", math.Round(v/1000))
	}
}
