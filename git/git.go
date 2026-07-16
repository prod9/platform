// Package git is platform's one git-exec boundary. Package-level funcs answer
// repo-shape questions (IsRoot); Context runs git against a project's repository,
// caching the per-process constants.
package git

import (
	"os"
	"path/filepath"
)

// IsRoot reports whether dir is itself a git repo root — a .git entry directly in dir,
// without walking up. A nested standalone repo (an infra repo developed in-place under
// another checkout) must have its own .git; the parent's does not count.
func IsRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
