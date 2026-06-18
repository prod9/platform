package bootstrapper

import (
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestValidateWD(t *testing.T) {
	// A non-existent target is a hard error — bootstrap never creates the repo
	// root itself.
	err := validateWD(filepath.Join(t.TempDir(), "nope"))
	r.Error(t, err)

	// A path that resolves to a file, not a directory, is rejected.
	file := filepath.Join(t.TempDir(), "afile")
	r.NoError(t, os.WriteFile(file, []byte("x"), 0644))
	r.ErrorIs(t, validateWD(file), ErrWDNotDir)

	// A directory that is not inside any git repo is rejected (hard error,
	// per the GitOps appliance: the infra repo must be a repo).
	bare := t.TempDir()
	r.ErrorIs(t, validateWD(bare), ErrWDNotGit)

	// A git repo passes.
	repo := gitRepo(t)
	r.NoError(t, validateWD(repo))

	// A subdirectory of a git repo passes too — detection walks up, matching
	// git's own notion of "inside a work tree".
	sub := filepath.Join(repo, "deep", "nested")
	r.NoError(t, os.MkdirAll(sub, 0755))
	r.NoError(t, validateWD(sub))
}

// gitRepo returns a fresh temp dir marked as a git repo. validateWD only needs
// a .git entry to exist, so no real `git init` is required — keeps it hermetic.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	r.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0755))
	return dir
}
