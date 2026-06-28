package bootstrapper

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed platform.template
var platformTemplate string

type Info struct {
	ProjectName     string
	Maintainer      string
	MaintainerEmail string
	Repository      string
	ImagePrefix     string
	GoVersion       string // TODO: Probably should detect from user's environment

	// CUE module scaffold inputs for `platform init`. ModulePath is the operator's chosen
	// module path (prompted greenfield-only); DefsModule/DefsVersion pin the infra-defs
	// dependency the baseline apps import. An empty ModulePath skips the scaffold.
	ModulePath  string
	DefsModule  string
	DefsVersion string
}

func renderTemplate(content string, info *Info) ([]byte, error) {
	tmpl, err := template.New("").Parse(content)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, info); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
