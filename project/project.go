package project

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type (
	Project struct {
		ConfigPath string `toml:"-"`
		ConfigDir  string `toml:"-"`

		Maintainer   string   `toml:"maintainer"`
		Platform     string   `toml:"platform,omitempty"`
		Repository   string   `toml:"repository"`
		Strategy     string   `toml:"strategy"`
		Environments []string `toml:"environments"`

		Excludes []string           `toml:"excludes"`
		Modules  map[string]*Module `toml:"modules"`
	}

	Module struct {
		WorkDir string        `toml:"workdir,omitempty"` // the directory we'll be working in
		Timeout time.Duration `toml:"timeout,omitempty"`
		Builder string        `toml:"builder,omitempty"`

		ImageName   string            `toml:"image,omitempty"`
		PackageName string            `toml:"package,omitempty"`
		BinaryName  string            `toml:"binary,omitempty"`
		BinaryArgs  []string          `toml:"binary_args,omitempty"`
		AssetDirs   []string          `toml:"asset_dirs,omitempty"`
		Env         map[string]string `toml:"env,omitempty"`
		GoVersion   string            `toml:"go_version,omitempty"`

		Publish bool `toml:"publish"`
	}
)

var (
	ProjectDefaults = &Project{
		Strategy: "timestamp",
		Platform: "linux/amd64",
		Excludes: []string{
			"*.docker",
			"*.local",
			".dockerignore",
			".git",
			".github",
			".gitignore",
			".idea",
			".vscode",
			"node_modules",
			"platform.toml",
		},
		Modules: map[string]*Module{},
	}

	ModuleDefaults = &Module{
		Timeout: 5 * time.Second,
		Publish: false,
	}
)

func Configure(wd string) (*Project, error) {
	if wd == "" || wd == "." {
		if wd_, err := os.Getwd(); err != nil {
			return nil, err
		} else {
			wd = wd_
		}
	}

	path, err := ResolvePath(wd)
	if err != nil {
		return nil, err
	}

	cfg := &Project{}
	if _, err = toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	cfg.ConfigPath = path
	cfg.ConfigDir = filepath.Dir(path)

	cfg.assignDefaults()
	cfg.assignEnvOverrides()
	cfg.inferValues()
	return cfg, nil
}

func (c *Project) assignDefaults() {
	for _, mod := range c.Modules {
		if mod.Timeout <= 0 {
			mod.Timeout = ModuleDefaults.Timeout
		}
	}
}

func (c *Project) assignEnvOverrides() {
	if platform, ok := os.LookupEnv("PLATFORM"); ok {
		log.Println("platform overriden from", c.Platform, "to", platform)
		c.Platform = platform
	}
}

func (c *Project) inferValues() {
	for modname, mod := range c.Modules {
		if mod.BinaryName == "" {
			mod.BinaryName = modname
		}
		if mod.WorkDir == "" {
			mod.WorkDir = "."
		}
	}
}
