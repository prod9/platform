package bootstrapper

// AnalyzeInit computes the platform.toml + `platform` launcher script + cue.mod scaffold for
// `platform init`: platform.toml is seeded with the baseline's default [ops.vars], and the
// script pins the platform CLI version so `ops render`/`publish` run reproducibly. Unlike
// Analyze (app onboarding) it does not require an existing git repository — `platform init`
// creates one. The baseline component files (apps/, defaults/) are the baseline package's
// concern: `ops init` renders them via baseline.Render and folds them in with Plan.AddFile.
func AnalyzeInit(dir string, info *Info, defaultVars map[string]any) (*Plan, error) {
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

	mod, err := planCueModule(dir, info)
	if err != nil {
		return nil, err
	}
	if mod != nil {
		files = append(files, *mod)
	}

	return &Plan{Dir: dir, Files: files, Vars: vars}, nil
}
