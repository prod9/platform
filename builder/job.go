package builder

import (
	"path/filepath"
	"time"

	"platform.prodigy9.co/config"
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
