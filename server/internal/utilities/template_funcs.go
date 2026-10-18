package utilities

import (
	"fmt"
	"html/template"
)

// TemplateFuncs returns the functions available to all HTML templates.
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
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
