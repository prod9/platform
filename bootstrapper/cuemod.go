package bootstrapper

import (
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/mod/modfile"
)

// cueModuleFile is the conventional location of a CUE module file, relative to the repo root.
var cueModuleFile = filepath.Join("cue.mod", "module.cue")

// HasCueModule reports whether dir already roots a CUE module (cue.mod/module.cue present).
// `platform init` scaffolds one only on a greenfield repo — an existing module is the
// operator's truth and is never rewritten.
func HasCueModule(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, cueModuleFile))
	return err == nil
}

// planCueModule scaffolds cue.mod/module.cue for a fresh infra repo: the operator's module
// path, the language version of the linked CUE engine (so render never demands a newer
// language than it links), and the pinned infra-defs dependency the baseline apps import.
// Returns nil when ModulePath is unset or a module already exists — render loads whatever
// cue.mod is present, so the existing one stands.
func planCueModule(dir string, info *Info) (*FileChange, error) {
	if info.ModulePath == "" || HasCueModule(dir) {
		return nil, nil
	}

	file := &modfile.File{
		Module:   info.ModulePath,
		Language: &modfile.Language{Version: cue.LanguageVersion()},
	}
	if info.DefsModule != "" {
		file.Deps = map[string]*modfile.Dep{
			info.DefsModule: {Version: info.DefsVersion},
		}
	}

	content, err := modfile.Format(file)
	if err != nil {
		return nil, err
	}

	fc := fileChange(dir, cueModuleFile, content, 0644)
	return &fc, nil
}
