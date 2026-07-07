package scaffold

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	ErrWDNotDir = errors.New("scaffold: target path is not a directory")
	ErrWDNotGit = errors.New("scaffold: target directory is not inside a git repository")
)

// validateWD checks the directory scaffold is about to write into: it must
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

// IsGitRoot reports whether dir is itself a git repo root — a .git entry directly
// in dir, without walking up. `ops init` uses this so it creates a standalone repo
// at the target even when that target is nested inside another git work tree (e.g.
// an infra repo developed in-place under another checkout). IsGitRepo's walk-up
// would see the parent and wrongly skip `git init`.
func IsGitRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
