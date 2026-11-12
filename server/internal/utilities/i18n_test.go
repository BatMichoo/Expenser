package utilities

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestTranslationsHaveSameKeys fails when a key exists in one language file
// but not in the others, so no user sees an untranslated key.
func TestTranslationsHaveSameKeys(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "i18n", "*.json"))
	if err != nil || len(files) < 2 {
		t.Fatalf("expected at least 2 translation files, got %v (err: %v)", files, err)
	}

	keys := map[string]map[string]bool{}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var dict map[string]string
		if err := json.Unmarshal(content, &dict); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		keys[file] = map[string]bool{}
		for k := range dict {
			keys[file][k] = true
		}
	}

	for fileA, keysA := range keys {
		for fileB, keysB := range keys {
			var missing []string
			for k := range keysA {
				if !keysB[k] {
					missing = append(missing, k)
				}
			}
			sort.Strings(missing)
			if len(missing) > 0 {
				t.Errorf("%s is missing keys from %s: %v", filepath.Base(fileB), filepath.Base(fileA), missing)
			}
		}
	}
}

func TestRoughAmount(t *testing.T) {
	cases := map[float64]string{0: "", 87.4: "87", 999.4: "999", 999.6: "1k", 1234: "1.2k", 2000: "2k", 9949: "9.9k", 9951: "10k", 12500: "13k"}
	for in, want := range cases {
		if got := RoughAmount(in); got != want {
			t.Errorf("RoughAmount(%v) = %q, want %q", in, got, want)
		}
	}
}
