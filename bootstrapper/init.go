package bootstrapper

import (
	"path/filepath"
	"sort"
)

// AnalyzeInit computes the plan for `platform init`: a platform.toml seeded with the
// baseline's default [ops.vars], the selected component files (both `.platform`
// directives and `.cue` apps) written into apps/, and a `platform` launcher script
// that pins the platform CLI version so `ops render`/`publish` run reproducibly.
// Unlike Analyze (app onboarding) it does not require an existing git repository —
// `platform init` creates one.
func AnalyzeInit(dir string, info *Info, components map[string][]byte, defaultVars map[string]any) (*Plan, error) {
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

	script, err := renderTemplate(platformTemplate, info)
	if err != nil {
		return nil, err
	}

	files := []FileChange{
		projFile,
		fileChange(dir, "platform", script, 0744),
	}
	for _, name := range sortedKeys(components) {
		rel := filepath.Join("apps", name)
		files = append(files, fileChange(dir, rel, components[name], 0644))
	}

	mod, err := planCueModule(dir, info)
	if err != nil {
		return nil, err
	}
	if mod != nil {
		files = append(files, *mod)
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
