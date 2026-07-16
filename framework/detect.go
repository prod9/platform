package framework

import (
	"os"
	"path/filepath"
)

// detectFile is the discovery probe: reports whether wd carries the framework's
// marker file (go.mod, pnpm-lock.yaml, Dockerfile, …).
func detectFile(wd, filename string) (bool, error) {
	_, err := os.Stat(filepath.Join(wd, filename))
	switch {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}
