package builder

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"platform.prodigy9.co/config"
)

var (
	ErrBadModule = errors.New("invalid module")
)

type Job struct {
	Config  *config.Config
	Builder Builder

	Name     string
	WorkDir  string
	Timeout  time.Duration
	Platform string
	Excludes []string

	Repository  string
	ImageName   string
	PackageName string
	BinaryName  string

	Publish          bool
	PublishImageName string
}

func JobsFromArgs(cfg *config.Config, args []string) (jobs []*Job, err error) {
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

func JobFromModule(cfg *config.Config, name string, mod *config.Module) (*Job, error) {
	b, err := FindBuilder(mod.Builder)
	if err != nil {
		return nil, err
	}

	modpath := filepath.Join(cfg.ConfigDir, mod.WorkDir)
	modpath = filepath.Clean(modpath)

	return &Job{
		Config:  cfg,
		Builder: b,

		Name:     name,
		WorkDir:  modpath,
		Timeout:  mod.Timeout,
		Platform: cfg.Platform,
		Excludes: cfg.Excludes,

		Repository:  cfg.Repository,
		ImageName:   mod.ImageName,
		PackageName: mod.PackageName,
		BinaryName:  mod.BinaryName,
	}, nil
}
