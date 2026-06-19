package bootstrapper

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	ErrWDNotDir = errors.New("bootstrap: target path is not a directory")
	ErrWDNotGit = errors.New("bootstrap: target directory is not inside a git repository")
)

// validateWD checks the directory bootstrap is about to write into: it must
// exist, be a directory, and live inside a git repository. The git check is a
// hard gate — the appliance baseline is delivered through GitOps, so a non-repo
// target is virtually always a mistake.
func validateWD(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return ErrWDNotDir
	}
	if !IsGitRepo(dir) {
		return ErrWDNotGit
	}
	return nil
}

// validateInitWD checks the `platform init` target exists and is a directory.
// Unlike validateWD it does not require a git repository — init creates one.
func validateInitWD(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return ErrWDNotDir
	}
	return nil
}

// IsGitRepo reports whether dir is inside a git work tree, walking up toward the
// filesystem root looking for a .git entry (a directory in a normal clone, a
// file in a worktree/submodule — os.Stat accepts both).
func IsGitRepo(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
