package initcmd

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestValidateDir(t *testing.T) {
	// A non-existent target is a hard error — scaffold never creates the repo
	// root itself.
	err := validateDir(filepath.Join(t.TempDir(), "nope"))
	r.Error(t, err)

	// A path that resolves to a file, not a directory, is rejected.
	file := filepath.Join(t.TempDir(), "afile")
	r.NoError(t, os.WriteFile(file, []byte("x"), 0644))
	r.ErrorIs(t, validateDir(file), ErrWDNotDir)

	// A plain directory that is not a git root is rejected — platform never runs
	// `git init`, the operator must have done it first.
	r.ErrorIs(t, validateDir(t.TempDir()), ErrWDNotGit)

	// A git repo root passes.
	r.NoError(t, validateDir(gitRepo(t)))
}

// gitRepo returns a fresh temp dir marked as a git repo root. git.IsRoot only needs a
// .git entry to exist, so no real `git init` is required — keeps it hermetic.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	r.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0755))
	return dir
}
