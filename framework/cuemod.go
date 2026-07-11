package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/mod/modfile"
	"platform.prodigy9.co/framework/scaffold"
)

// cueModuleFile is the conventional location of a CUE module file, relative to the repo root.
var cueModuleFile = filepath.Join("cue.mod", "module.cue")

// HasCueModule reports whether dir already roots a CUE module (cue.mod/module.cue present).
// Infra.Scaffold contributes one only on a greenfield repo — an existing module is the
// operator's truth and is never rewritten.
func HasCueModule(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, cueModuleFile))
	return err == nil
}

// CueModulePath reads the import path of an existing CUE module (cue.mod/module.cue),
// stripping any `@vN` major-version suffix so callers can form import paths like
// `<module>/defaults`.
func CueModulePath(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, cueModuleFile))
	if err != nil {
		return "", err
	}

	file, err := modfile.Parse(data, cueModuleFile)
	if err != nil {
		return "", err
	}

	path, _, _ := strings.Cut(file.Module, "@")
	return path, nil
}

// cueModFile is the Infra framework's greenfield cue.mod/module.cue contribution: the
// operator's module path (a {{.ModulePath}} hole the driver resolves), the linked CUE
// evaluator's language version (so render never demands a newer language than it links),
// and the pinned infra-defs dependency the baseline apps import.
func cueModFile() scaffold.File {
	content := fmt.Sprintf(
		"module: \"{{.ModulePath}}\"\nlanguage: {\n\tversion: %q\n}\ndeps: {\n\t%q: {\n\t\tv: %q\n\t}\n}\n",
		cue.LanguageVersion(), DefsModule, DefsVersion)
	return scaffold.File{Path: cueModuleFile + ".tmpl", Content: []byte(content), Mode: 0644}
}
