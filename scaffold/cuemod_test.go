package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/mod/modfile"
	r "github.com/stretchr/testify/require"
)

func cueModInfo() *Info {
	info := testInfo()
	info.ModulePath = "test.example/infra"
	info.DefsModule = "prodigy9.co/defs@v0"
	info.DefsVersion = "v0.3.19"
	return info
}

// Greenfield init scaffolds cue.mod/module.cue: module path from Info, language version
// from the linked CUE engine, the defs dep pinned — the shape `ops render` loads.
func TestAnalyzeInit_scaffoldsCueModule(t *testing.T) {
	dir := t.TempDir()

	plan, err := AnalyzeInit(dir, cueModInfo(), nil)
	r.NoError(t, err)

	rel := filepath.Join("cue.mod", "module.cue")
	var fc *FileChange
	for i := range plan.Files {
		if plan.Files[i].Path == rel {
			fc = &plan.Files[i]
		}
	}
	r.NotNil(t, fc, "plan must scaffold cue.mod/module.cue")
	r.Equal(t, FileWrite, fc.Action)

	mf, err := modfile.Parse(fc.Content, rel)
	r.NoError(t, err)
	r.Equal(t, "test.example/infra", mf.Module)
	r.Equal(t, cue.LanguageVersion(), mf.Language.Version)
	r.Contains(t, mf.Deps, "prodigy9.co/defs@v0")
	r.Equal(t, "v0.3.19", mf.Deps["prodigy9.co/defs@v0"].Version)
}

// Re-init over an existing module is a no-op on cue.mod: it's the operator's truth, never
// clobbered — greenfield-only scaffolding.
func TestAnalyzeInit_keepsExistingCueModule(t *testing.T) {
	dir := t.TempDir()
	mod := filepath.Join(dir, "cue.mod", "module.cue")
	r.NoError(t, os.MkdirAll(filepath.Dir(mod), 0o755))
	r.NoError(t, os.WriteFile(mod,
		[]byte("module: \"kept.example/infra\"\nlanguage: version: \"v0.15.4\"\n"), 0o644))

	plan, err := AnalyzeInit(dir, cueModInfo(), nil)
	r.NoError(t, err)

	for _, f := range plan.Files {
		r.NotEqual(t, filepath.Join("cue.mod", "module.cue"), f.Path,
			"existing cue.mod must not be in the plan")
	}
}
