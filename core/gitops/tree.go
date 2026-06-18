package gitops

import (
	"os"
	"path/filepath"
	"sort"
)

// Tree is a rendered manifest set keyed by repo-relative path
// (<component>/<filename>), each value the file's (multi-doc) YAML payload. It
// is the uniform output of every render route and the unit Publish packages.
type Tree map[string][]byte

// Paths returns the tree's keys in sorted order — a stable iteration order for
// deterministic disk writes and archive digests.
func (t Tree) Paths() []string {
	paths := make([]string, 0, len(t))
	for p := range t {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// WriteDir writes every entry under dir, creating component subdirectories.
// Files are truncate-written; existing content at a path is replaced.
func (t Tree) WriteDir(dir string) error {
	for _, rel := range t.Paths() {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, t[rel], 0o644); err != nil {
			return err
		}
	}
	return nil
}
