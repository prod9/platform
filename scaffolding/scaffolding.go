// Package scaffolding discovers repository initialization targets, plans their files,
// and applies confirmed plans.
package scaffolding

import (
	"errors"
	"os"
	"path/filepath"

	"platform.prodigy9.co/framework"
)

// Target is the repository state initialization will plan against.
type Target struct {
	dir       string
	framework framework.Framework
}

// Discover resolves the initialization target rooted at dir.
func Discover(dir string) (*Target, error) {
	dir, err := resolveDir(dir)
	if err != nil {
		return nil, err
	}
	if err := validateDir(dir); err != nil {
		return nil, err
	}

	fw, err := framework.Discover(dir)
	if errors.Is(err, framework.ErrNoFramework) {
		return &Target{dir: dir}, nil
	}
	if err != nil {
		return nil, err
	}

	return &Target{dir: dir, framework: fw}, nil
}

// ScaffoldVars reports the framework-specific values initialization needs.
func (t *Target) ScaffoldVars() []string {
	if t.framework == nil {
		return nil
	}
	return t.framework.ScaffoldVars(t.dir)
}

func resolveDir(dir string) (string, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = wd
	}

	if filepath.IsAbs(dir) {
		return dir, nil
	}
	return filepath.Abs(dir)
}
