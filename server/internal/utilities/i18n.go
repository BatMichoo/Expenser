package utilities

import (
	"encoding/json"
	"expenser/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	translations = make(map[string]map[string]string)
	i18nOnce     sync.Once
	defaultLang  = "en"
)

// InitI18n loads translation files into memory.
func InitI18n() error {
	var err error
	i18nOnce.Do(func() {
		var i18nPath string
		projectRoot := config.GetProjectRootDir()
		
		// Determine path based on environment/project structure
		i18nPath = filepath.Join(projectRoot, "internal/i18n")
		
		files, readErr := os.ReadDir(i18nPath)
		if readErr != nil {
			err = fmt.Errorf("failed to read i18n directory: %w", readErr)
			return
		}

		for _, file := range files {
			if filepath.Ext(file.Name()) == ".json" {
				lang := file.Name()[:len(file.Name())-len(".json")]
				filePath := filepath.Join(i18nPath, file.Name())
				
				content, fileErr := os.ReadFile(filePath)
				if fileErr != nil {
					err = fmt.Errorf("failed to read translation file %s: %w", file.Name(), fileErr)
					return
				}

				var data map[string]string
				if unmarshalErr := json.Unmarshal(content, &data); unmarshalErr != nil {
					err = fmt.Errorf("failed to parse translation file %s: %w", file.Name(), unmarshalErr)
					return
				}

				translations[lang] = data
			}
		}
	})
	return err
}

// T translates a key into the given language.
func T(lang, key string) string {
	if lang == "" {
		lang = defaultLang
	}
	
	if langDict, ok := translations[lang]; ok {
		if val, ok := langDict[key]; ok {
			return val
		}
	}

	// Fallback to English if not found in requested language
	if lang != defaultLang {
		if enDict, ok := translations[defaultLang]; ok {
			if val, ok := enDict[key]; ok {
				return val
			}
		}
	}

	return key
}
