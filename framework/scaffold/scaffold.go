// Package scaffold is the one files/templating mechanism behind `platform init`: it
// defines the shapes a framework's Scaffold returns and resolves their template holes.
// Generic — no discovery, no orchestration, no per-framework data; everything
// type-specific comes in through the Spec a framework hands it.
package scaffold

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"platform.prodigy9.co/conf"
)

// File is one file a framework's Scaffold contributes, beyond the universal
// platform.toml + launcher the driver writes for every repo. Path is relative to the
// repo root (routing already applied); a ".tmpl" suffix marks Content as a
// text/template that Resolve fills (and strips) with the scaffold Data.
type File struct {
	Path    string
	Content []byte
	Mode    fs.FileMode
}

// Spec is a framework's full contribution to a freshly scaffolded repo: the platform.toml
// module it adds, the default [vars] it seeds, the files it ships (already **resolved** — the
// framework fills its own holes via Resolve), and the strategy value a fresh platform.toml
// gets. The driver generates platform.toml and writes the files as-is.
type Spec struct {
	Module   *conf.Module
	Vars     map[string]any
	Files    []File
	Strategy string
}

// Env carries the environment facts the driver hands every framework's Scaffold: the
// operator identity and repository from init's universal prompts, and the dagger SDK
// version the binary links. Frameworks pull what they need; most need none of it.
type Env struct {
	Repository      string
	MaintainerEmail string
	DaggerVersion   string
}

// Data fills the placeholders in ".tmpl" files at init time, keyed by the hole name the
// template writes ({{ .ModulePath }} reads Data["ModulePath"]). It is a map, not a struct,
// because the holes belong to whoever authored the .tmpl: a framework knows its own
// templates and fills its own keys, and this package must not grow a field per framework.
// A hole with no entry is a hard error at Resolve, so a typo cannot silently render empty.
type Data map[string]any

// Resolve resolves a framework's files for installation: ".tmpl" files pass through
// text/template with data (missing keys are hard errors) and lose the suffix;
// everything else passes through verbatim — non-template CUE braces must never meet
// the template engine. Input order is preserved.
func Resolve(files []File, data Data) ([]File, error) {
	out := make([]File, 0, len(files))
	for _, f := range files {
		resolved, err := resolveFile(f, data)
		if err != nil {
			return nil, fmt.Errorf("scaffold: resolve %s: %w", f.Path, err)
		}
		out = append(out, resolved)
	}
	return out, nil
}

func resolveFile(f File, data Data) (File, error) {
	if !strings.HasSuffix(f.Path, ".tmpl") {
		return f, nil
	}

	tmpl, err := template.New(f.Path).Option("missingkey=error").Parse(string(f.Content))
	if err != nil {
		return File{}, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return File{}, err
	}
	return File{Path: strings.TrimSuffix(f.Path, ".tmpl"), Content: buf.Bytes(), Mode: f.Mode}, nil
}
