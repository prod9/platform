package build

import (
	"path/filepath"

	"platform.prodigy9.co/config"
)

type Job struct {
	Name           string
	WD             string
	Timeout        config.Timeout
	TargetPlatform string
	Excludes       []string

	SourceURL   string
	ImageName   string
	PackageName string
	BinaryName  string
}

func JobFromModule(cfg *config.Config, name string, mod *config.Module) *Job {
	modpath := filepath.Dir(cfg.ConfigPath)
	modpath = filepath.Join(modpath, mod.WD)
	modpath = filepath.Clean(modpath)

	return &Job{
		Name:           name,
		WD:             modpath,
		Timeout:        mod.Timeout,
		TargetPlatform: cfg.TargetPlatform,
		Excludes:       cfg.Excludes,

		SourceURL:   cfg.SourceURL,
		ImageName:   mod.ImageName,
		PackageName: mod.PackageName,
		BinaryName:  mod.BinaryName,
	}
}
