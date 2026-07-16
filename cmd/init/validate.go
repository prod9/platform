package initcmd

import (
	"errors"
	"os"

	"platform.prodigy9.co/git"
)

var (
	ErrWDNotDir = errors.New("scaffold: target path is not a directory")
	ErrWDNotGit = errors.New("scaffold: target directory is not a git repository (run `git init` first)")
)

// validateDir checks the directory scaffold is about to write into: it exists, is a
// directory, and is its own git repo root. Platform never creates the repo — the operator
// runs `git init` (or clones) first, for every framework alike (delivery is git-based end
// to end, so a non-repo target is always a mistake). Runs before framework discovery; the
// git precondition is uniform, not framework-set.
func validateDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return ErrWDNotDir
	}
	if !git.IsRoot(dir) {
		return ErrWDNotGit
	}
	return nil
}
