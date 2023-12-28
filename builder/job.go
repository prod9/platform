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

type Job struct {
	Config  *project.Project
	Builder Interface

	Name     string
	WorkDir  string
	Timeout  time.Duration
	Platform string
	Excludes []string

	Env         map[string]string
	AssetDirs   []string
	BuildDir    string
	CommandArgs []string
	CommandName string
	ImageName   string
	PackageName string
	Repository  string
}

func JobsFromArgs(cfg *project.Project, args []string) (jobs []*Job, err error) {
	if len(args) == 0 {
		for modname, mod := range cfg.Modules {
			if job, err := JobFromModule(cfg, modname, mod); err != nil {
				return nil, err
			} else {
				jobs = append(jobs, job)
			}
		}

	} else {
		for len(args) > 0 {
			modname := args[0]
			args = args[1:]

			if mod, ok := cfg.Modules[modname]; !ok {
				return nil, fmt.Errorf(modname+": %w", ErrBadModule)
			} else if job, err := JobFromModule(cfg, modname, mod); err != nil {
				return nil, err
			} else {
				jobs = append(jobs, job)
			}
		}
	}

	return jobs, nil
}

func JobFromModule(cfg *project.Project, name string, mod *project.Module) (*Job, error) {
	b, err := FindBuilder(mod.Builder)
	if err != nil {
		return nil, err
	}

	modpath := filepath.Join(cfg.ConfigDir, mod.WorkDir)
	modpath = filepath.Clean(modpath)

	var platform string
	if strings.ToLower(cfg.Platform) == "auto" {
		// since linux is the most compatible, we should be safe here
		// also assumes that platform is built for the same architecture since it's meant
		// to be run locally on the target machine
		platform = "linux/" + runtime.GOARCH
	} else {
		platform = cfg.Platform
	}

	return &Job{
		Config:  cfg,
		Builder: b,

		Name:     name,
		WorkDir:  modpath,
		Timeout:  mod.Timeout.Duration(),
		Platform: platform,
		Excludes: cfg.Excludes,

		Env:         mod.Env,
		AssetDirs:   mod.AssetDirs,
		BuildDir:    mod.BuildDir,
		CommandArgs: mod.CommandArgs,
		CommandName: mod.CommandName,
		ImageName:   mod.ImageName,
		PackageName: mod.PackageName,
		Repository:  cfg.Repository,
	}, nil
}
