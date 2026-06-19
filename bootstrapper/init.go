package bootstrapper

import (
	"path/filepath"
	"sort"
)

// AnalyzeInit computes the plan for `platform init`: a platform.toml seeded with
// the baseline's default [ops.vars], plus the embedded baseline directive files
// written under baseline/. Unlike Analyze (app onboarding) it writes neither the
// platform build script nor the CI pipeline, and it does not require an existing
// git repository — `platform init` creates one.
func AnalyzeInit(dir string, info *Info, baselineFiles map[string][]byte, defaultVars map[string]string) (*Plan, error) {
	dir, err := resolveWD(dir)
	if err != nil {
		return nil, err
	}
	if err := validateInitWD(dir); err != nil {
		return nil, err
	}

	projFile, vars, err := planProjectFile(dir, info, defaultVars)
	if err != nil {
		return nil, err
	}

	files := []FileChange{projFile}
	for _, name := range sortedKeys(baselineFiles) {
		rel := filepath.Join("baseline", name)
		files = append(files, fileChange(dir, rel, baselineFiles[name], 0644))
	}
	return &Plan{Dir: dir, Files: files, Vars: vars}, nil
}

func sortedKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
