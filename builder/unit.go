package builder

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"platform.prodigy9.co/project"
)

var (
	ErrBadModule = errors.New("invalid module")
)

type BuildUnit struct {
	Config  *project.Project
	Builder Interface

	Name     string
	WorkDir  string
	Timeout  time.Duration
	Platform string
	Excludes []string

	Env         map[string]string
	Port        *int
	CommandName string
	CommandArgs []string

	AssetDirs   []string
	BuildDir    string
	ImageName   string
	PackageName string
	Repository  string
}

// Purpose says what a build is for, which selects the target arch: local builds
// track the host arch for speed; publish/deploy builds pin the server arch so an
// arm laptop never ships an unrunnable image.
type Purpose int

const (
	LocalBuild Purpose = iota
	PublishBuild
)

// JobsFromArgs builds one job per selected module. The command declares its
// Purpose (local vs publish); JobFromModule resolves that into each Job's arch
// target, so the build stage reads a complete definition rather than being told
// the platform through arguments.
func JobsFromArgs(cfg *project.Project, args []string, purpose Purpose) (units []*BuildUnit, err error) {
	if len(args) == 0 {
		for modname, mod := range cfg.Modules {
			if unit, err := JobFromModule(cfg, modname, mod, purpose); err != nil {
				return nil, err
			} else {
				units = append(units, unit)
			}
		}

	} else {
		for len(args) > 0 {
			modname := args[0]
			args = args[1:]

			if mod, ok := cfg.Modules[modname]; !ok {
				return nil, fmt.Errorf(modname+": %w", ErrBadModule)
			} else if unit, err := JobFromModule(cfg, modname, mod, purpose); err != nil {
				return nil, err
			} else {
				units = append(units, unit)
			}
		}
	}

	return units, nil
}

func JobFromModule(cfg *project.Project, name string, mod *project.Module, purpose Purpose) (*BuildUnit, error) {
	b, err := FindBuilder(mod.Builder)
	if err != nil {
		return nil, err
	}

	modpath := filepath.Join(cfg.ConfigDir, mod.WorkDir)
	modpath = filepath.Clean(modpath)

	platform := resolveArch(archFor(cfg, purpose))

	return &BuildUnit{
		Config:  cfg,
		Builder: b,

		Name:     name,
		WorkDir:  modpath,
		Timeout:  mod.Timeout.Duration(),
		Platform: platform,
		Excludes: cfg.Excludes,

		Env:         mod.Env,
		Port:        mod.Port,
		CommandName: mod.CommandName,
		CommandArgs: mod.CommandArgs,

		AssetDirs:   mod.AssetDirs,
		BuildDir:    mod.BuildDir,
		ImageName:   mod.ImageName,
		PackageName: mod.PackageName,
		Repository:  cfg.Repository,
	}, nil
}

// archFor picks the configured arch for a build's purpose.
func archFor(cfg *project.Project, purpose Purpose) string {
	if purpose == PublishBuild {
		return cfg.PublishArch
	}
	return cfg.LocalArch
}

// resolveArch turns a configured arch into a concrete dagger platform string.
// "auto" tracks the host arch (fast native local builds); a bare arch like
// "amd64" becomes "linux/<arch>" since these containers are always linux; a full
// "linux/arch" string (the deprecated `platform` key or the PLATFORM env) is
// honored verbatim.
func resolveArch(spec string) string {
	switch {
	case strings.EqualFold(spec, "auto"):
		return "linux/" + runtime.GOARCH
	case strings.Contains(spec, "/"):
		return spec
	default:
		return "linux/" + spec
	}
}
