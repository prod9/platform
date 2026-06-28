package builder

import (
	"fmt"

	"platform.prodigy9.co/project"
)

// Purpose says what a build is for, which selects the target arch: local builds
// track the host arch for speed; publish/deploy builds pin the server arch so an
// arm laptop never ships an unrunnable image.
type Purpose int

const (
	LocalBuild Purpose = iota
	PublishBuild
)

// BuildAttempt is one invocation of the build pipeline — a single trigger (a CLI
// `platform build`, a webhook push) that fans out over one BuildUnit per selected
// module. Purpose pins the arch the whole attempt builds for.
type BuildAttempt struct {
	Purpose Purpose
	Units   []*BuildUnit
}

// Attempt assembles a BuildAttempt over the given units. Callers that already hold
// resolved units (e.g. preview, narrowing to one module) use this instead of poking
// the struct, so attempt construction stays inside the builder package.
func Attempt(purpose Purpose, units ...*BuildUnit) *BuildAttempt {
	return &BuildAttempt{Purpose: purpose, Units: units}
}

// AttemptFrom builds one unit per selected module (all modules when args is
// empty). The command declares the Purpose; unitFromModule resolves it into each
// unit's arch target, so the build stage reads a complete definition rather than
// being told the platform through arguments.
func AttemptFrom(cfg *project.Project, args []string, purpose Purpose) (*BuildAttempt, error) {
	var units []*BuildUnit
	if len(args) == 0 {
		for modname, mod := range cfg.Modules {
			unit, err := unitFromModule(cfg, modname, mod, purpose)
			if err != nil {
				return nil, err
			}
			units = append(units, unit)
		}
	} else {
		for _, modname := range args {
			mod, ok := cfg.Modules[modname]
			if !ok {
				return nil, fmt.Errorf("%s: %w", modname, ErrBadModule)
			}
			unit, err := unitFromModule(cfg, modname, mod, purpose)
			if err != nil {
				return nil, err
			}
			units = append(units, unit)
		}
	}

	return Attempt(purpose, units...), nil
}
