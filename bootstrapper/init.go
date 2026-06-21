package bootstrapper

import (
	"path/filepath"
	"sort"
)

// AnalyzeInit computes the plan for `platform init`: a platform.toml seeded with
// the baseline's default [ops.vars], plus the embedded baseline — directive files
// under baseline/ and CUE app files under apps/. Unlike Analyze (app onboarding) it
// writes neither the platform build script nor the CI pipeline, and it does not
// require an existing git repository — `platform init` creates one.
func AnalyzeInit(dir string, info *Info, baselineFiles, appFiles map[string][]byte, defaultVars map[string]any) (*Plan, error) {
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
	files = appendUnder(dir, files, "baseline", baselineFiles)
	files = appendUnder(dir, files, "apps", appFiles)
	return &Plan{Dir: dir, Files: files, Vars: vars}, nil
}

// appendUnder plans each file in m as a write under subdir, in deterministic order.
func appendUnder(dir string, files []FileChange, subdir string, m map[string][]byte) []FileChange {
	for _, name := range sortedKeys(m) {
		rel := filepath.Join(subdir, name)
		files = append(files, fileChange(dir, rel, m[name], 0644))
	}
	return files
}

func sortedKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
