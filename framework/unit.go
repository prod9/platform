package framework

import (
	"errors"
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
	Framework Framework

	Name     string
	WorkDir  string
	Timeout  time.Duration
	Arch     string
	Excludes []string

	Env         map[string]string
	Port        int
	CommandName string
	CommandArgs []string

	AssetDirs   []string
	BuildDir    string
	ImageName   string
	PackageName string
	Repository  string

	// Vars is the [ops.vars] table, carried for the Infra framework to feed its render
	// (CUE @tag / directive \(var) interpolation). Empty for ordinary app frameworks.
	Vars map[string]any
}

func unitFromModule(cfg *project.Project, name string, mod *project.Module, purpose Purpose) (*BuildUnit, error) {
	b, err := FindFramework(mod.Framework)
	if err != nil {
		return nil, err
	}

	modpath := filepath.Join(cfg.ConfigDir, mod.WorkDir)
	modpath = filepath.Clean(modpath)

	arch := resolveArch(archFor(cfg, purpose))

	return &BuildUnit{
		Framework: b,

		Name:     name,
		WorkDir:  modpath,
		Timeout:  mod.Timeout.Duration(),
		Arch:     arch,
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

		Vars: cfg.Ops.Vars,
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
