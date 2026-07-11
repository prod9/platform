package cmd

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	ErrWDNotDir = errors.New("scaffold: target path is not a directory")
	ErrWDNotGit = errors.New("scaffold: target directory is not inside a git repository")
)

// validateDir checks the directory scaffold is about to write into exists and is a
// directory. It runs before framework discovery; the git gate is framework-set (a
// scaffold that creates its own repo needs none) and is checked afterward in Analyze.
func validateDir(dir string) error {
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
// in dir, without walking up. `init` uses this so it creates a standalone repo
// at the target even when that target is nested inside another git work tree (e.g.
// an infra repo developed in-place under another checkout). IsGitRepo's walk-up
// would see the parent and wrongly skip `git init`.
func IsGitRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
