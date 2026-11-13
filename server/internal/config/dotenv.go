package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadDevEnv loads <module root>/.env.dev into the process environment for
// local development. It never overrides variables that are already set, so
// real environment values (Docker, CI) win. loaded is false, with no error,
// when there is no module root or no file: that is the normal case in
// production. Call it explicitly before LoadConfig; LoadConfig itself only
// reads the environment.
func LoadDevEnv() (path string, loaded bool, err error) {
	root := GetProjectRootDir()
	if root == "" {
		return "", false, nil
	}
	path = filepath.Join(root, ".env.dev")
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return path, false, nil
	}
	if err := godotenv.Load(path); err != nil {
		return path, false, err
	}
	return path, true, nil
}
