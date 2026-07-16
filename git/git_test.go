package git

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()

	_, err := Run(t.Context(), dir, "init", "-q", "-b", "main")
	r.NoError(t, err)

	// stdout comes back trimmed.
	branch, err := Run(t.Context(), dir, "symbolic-ref", "--short", "HEAD")
	r.NoError(t, err)
	r.Equal(t, "main", branch)

	// on failure, git's stderr lands in the error.
	_, err = Run(t.Context(), dir, "rev-parse", "--verify", "nonexistent")
	r.Error(t, err)
	r.ErrorContains(t, err, "fatal")
}

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
