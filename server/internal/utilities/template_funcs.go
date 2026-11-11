package utilities

import (
	"fmt"
	"html/template"
	"math"
)

// TemplateFuncs returns the functions available to every HTML template.
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		// cat maps a list position to one of the 6 categorical colours
		// (cat-1 … cat-6) used for per-type charts.
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
		"cat": func(i int) int {
			return i%6 + 1
		},
		// pct is part as a percentage of whole, clamped to 0–100 for bars.
		"pct": func(part, whole float64) float64 {
			if whole <= 0 {
				return 0
			}
			return math.Max(0, math.Min(100, part/whole*100))
		},
		"abs": math.Abs,
		"money": func(v float64) string {
			return fmt.Sprintf("%.2f %s", v, CurrencySymbol())
		},
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
