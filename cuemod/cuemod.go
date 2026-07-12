// Package cuemod reads a repo's CUE module file (cue.mod/module.cue) — operator truth for
// the module path. Both init (to seed the apps' `import "<module>/defaults"` lines) and
// render (to load the apps package by its module-qualified path) read it here; neither
// derives the module path from platform.toml.
package cuemod

import (
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/mod/modfile"
)

// ModuleFile is the conventional location of a CUE module file, relative to the repo root.
var ModuleFile = filepath.Join("cue.mod", "module.cue")

// Present reports whether dir already roots a CUE module (cue.mod/module.cue present). The
// Infra scaffold contributes one only on a greenfield repo — an existing module is the
// operator's truth and is never rewritten.
func Present(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ModuleFile))
	return err == nil
}

// Path reads the import path of an existing CUE module (cue.mod/module.cue), stripping any
// `@vN` major-version suffix so callers can form import paths like `<module>/defaults`.
func Path(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, ModuleFile))
	if err != nil {
		return "", err
	}

	file, err := modfile.Parse(data, ModuleFile)
	if err != nil {
		return "", err
	}

	path, _, _ := strings.Cut(file.Module, "@")
	return path, nil
}
