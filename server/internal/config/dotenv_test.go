package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const probeVar = "EXPENSER_DOTENV_PROBE"

// writeFile writes content to dir/name.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

// inDir changes the working directory for the test and restores it after.
func inDir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(old) })
}

// resolved follows symlinks so paths under a symlinked TempDir compare equal.
func resolved(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return r
}

func TestLoadDevEnvLoadsFromModuleRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module x\n")
	writeFile(t, root, ".env.dev", probeVar+"=from-file\n")
	sub := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	inDir(t, sub)
	t.Setenv(probeVar, "") // restored after the test
	os.Unsetenv(probeVar)

	path, loaded, err := LoadDevEnv()
	require.NoError(t, err)
	assert.True(t, loaded)
	assert.Equal(t, filepath.Join(resolved(t, root), ".env.dev"), resolved(t, path))
	assert.Equal(t, "from-file", os.Getenv(probeVar))
}

func TestLoadDevEnvDoesNotOverride(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module x\n")
	writeFile(t, root, ".env.dev", probeVar+"=from-file\n")
	inDir(t, root)
	t.Setenv(probeVar, "real")

	_, loaded, err := LoadDevEnv()
	require.NoError(t, err)
	assert.True(t, loaded)
	assert.Equal(t, "real", os.Getenv(probeVar))
}

func TestLoadDevEnvWithoutModuleRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env.dev", probeVar+"=from-file\n")
	inDir(t, dir)
	t.Setenv(probeVar, "")
	os.Unsetenv(probeVar)

	path, loaded, err := LoadDevEnv()
	require.NoError(t, err)
	assert.False(t, loaded)
	assert.Equal(t, "", path)
	_, set := os.LookupEnv(probeVar)
	assert.False(t, set)
}

func TestLoadDevEnvMissingFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module x\n")
	inDir(t, root)

	path, loaded, err := LoadDevEnv()
	require.NoError(t, err)
	assert.False(t, loaded)
	assert.Equal(t, filepath.Join(resolved(t, root), ".env.dev"), resolved(t, filepath.Dir(path))+"/.env.dev")
}
