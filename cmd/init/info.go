package initcmd

import (
	"os"
	"path/filepath"
)

// Info carries the operator inputs `init` prompts for.
type Info struct {
	Maintainer      string
	MaintainerEmail string
	Repository      string
}

func resolveWD(wd string) (string, error) {
	if wd == "" {
		wd_, err := os.Getwd()
		if err != nil {
			return "", err
		}
		wd = wd_
	}

	if !filepath.IsAbs(wd) {
		abs, err := filepath.Abs(wd)
		if err != nil {
			return "", err
		}
		wd = abs
	}

	return wd, nil
}
