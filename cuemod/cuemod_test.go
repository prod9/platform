package cuemod

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestPathStripsMajorSuffix(t *testing.T) {
	// Callers form import paths like `<module>/defaults`, so the `@vN` major-version
	// suffix of an existing module must be stripped.
	dir := t.TempDir()
	mod := filepath.Join(dir, "cue.mod", "module.cue")
	r.NoError(t, os.MkdirAll(filepath.Dir(mod), 0o755))
	r.NoError(t, os.WriteFile(mod,
		[]byte("module: \"kept.example/infra@v0\"\nlanguage: version: \"v0.15.4\"\n"), 0o644))

	path, err := Path(dir)
	r.NoError(t, err)
	r.Equal(t, "kept.example/infra", path)
}

func TestPresent(t *testing.T) {
	dir := t.TempDir()
	r.False(t, Present(dir))

	mod := filepath.Join(dir, "cue.mod", "module.cue")
	r.NoError(t, os.MkdirAll(filepath.Dir(mod), 0o755))
	r.NoError(t, os.WriteFile(mod, []byte("module: \"x.example\"\n"), 0o644))
	r.True(t, Present(dir))
}
