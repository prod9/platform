package initcmd

import (
	_ "embed"
	"os"
	"path/filepath"
)

// platformTemplate is the version-pinned launcher script every scaffolded repo gets,
// written verbatim — the one templating mechanism is framework/scaffold, and the
// launcher carries no holes.
//
//go:embed platform.template
var platformTemplate string

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
