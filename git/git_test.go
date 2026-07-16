package git

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestIsRoot(t *testing.T) {
	repo := gitRepo(t)

	// The repo root itself is a root.
	r.True(t, IsRoot(repo))

	// A nested subdir sits under the repo but is NOT its own root — a standalone repo
	// there needs its own `git init`.
	sub := filepath.Join(repo, "infra")
	r.NoError(t, os.MkdirAll(sub, 0755))
	r.False(t, IsRoot(sub))

	// A bare dir is not a root.
	r.False(t, IsRoot(t.TempDir()))
}

// gitRepo returns a fresh temp dir marked as a git repo root. IsRoot only needs a
// .git entry to exist, so no real `git init` is required — keeps it hermetic.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	r.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0755))
	return dir
}
