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

	"platform.prodigy9.co/project"
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

// Spec is a framework's full declarative contribution to a freshly scaffolded repo:
// the platform.toml module it adds, the default [vars] it seeds, the files it
// ships (holes unresolved), the strategy value a fresh platform.toml gets, and whether
// the target must become its own fresh git repository. It is the pure output of
// Scaffold — the driver gathers operator inputs, generates platform.toml, resolves the
// Files' holes via Resolve, and writes.
type Spec struct {
	Module       *project.Module
	Vars         map[string]any
	Files        []File
	Strategy     string
	NeedsGitRepo bool
}

// Data fills the placeholders in ".tmpl" files at init time — all discoverable, none
// prompted: DaggerVersion comes from the linked SDK; ModulePath is the CUE module a
// .tmpl hole resolves to, read from an existing cue.mod or defaulted to the repository;
// ImageBase is derived from the repository.
type Data struct {
	DaggerVersion string
	ModulePath    string
	ImageBase     string // OCI artifact base for the flux self-sync (oci://<ImageBase>)
}

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
