package framework

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"platform.prodigy9.co/conf"
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

	// Vars is the resolved [vars] table, carried on every unit. A framework that
	// interpolates (CUE @tag / directive \(var)) consumes it; one that doesn't ignores
	// it — a per-framework capability, not a per-project mode.
	Vars map[string]any
}

// RepositoryURL is the https form of the scheme-less platform.toml repository — the
// org.opencontainers.image.source value GitHub parses to link a pushed package to its
// repository. Linkage is what routes registry_package webhook events to the repo's
// webhooks; a non-URL value leaves the package unlinked and the webhook silently deaf.
func (u *BuildUnit) RepositoryURL() string {
	return "https://" + u.Repository
}

func unitFromModule(cfg *conf.Model, name string, mod *conf.Module, purpose Purpose) (*BuildUnit, error) {
	fw, err := FindFramework(mod.Framework)
	if err != nil {
		return nil, err
	}

	modpath := filepath.Join(cfg.ConfigDir, mod.WorkDir)
	modpath = filepath.Clean(modpath)

	arch := resolveArch(archFor(cfg, purpose))

	return &BuildUnit{
		Framework: fw,

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

		Vars: cfg.Vars,
	}, nil
}

// archFor picks the configured arch for a build's purpose.
func archFor(cfg *conf.Model, purpose Purpose) string {
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
