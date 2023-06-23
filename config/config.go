package config

import (
	"log"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	DefaultTimeout = 5 * time.Minute
)

type Config struct {
	Maintainer string `toml:"maintainer"`
	ConfigPath string `toml:"-"`
	Platform   string `toml:"platform"`
	Repository string `toml:"repository"`

	Excludes []string           `toml:"excludes"`
	Modules  map[string]*Module `toml:"modules"`
}

type Module struct {
	WorkDir string        `toml:"workdir"` // the directory we'll be working in
	Timeout time.Duration `toml:"timeout"`
	Builder string        `toml:"builder"`

	ImageName   string `toml:"image"`
	PackageName string `toml:"package"`
	BinaryName  string `toml:"binary"`
}

func Configure(wd string) (*Config, error) {
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

	cfg := &Config{}
	if _, err = toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	cfg.ConfigPath = path
	cfg.assignDefaults()
	cfg.assignEnvOverrides()
	cfg.inferValues()
	return cfg, nil
}

func (c *Config) assignDefaults() {
	for _, mod := range c.Modules {
		if mod.Timeout <= 0 {
			mod.Timeout = DefaultTimeout
		}
	}
}

func (c *Config) assignEnvOverrides() {
	if platform, ok := os.LookupEnv("PLATFORM"); ok {
		log.Println("platform overriden from", c.Platform, "to", platform)
		c.Platform = platform
	}
}

func (c *Config) inferValues() {
	for modname, mod := range c.Modules {
		if mod.BinaryName == "" {
			mod.BinaryName = modname
		}
		if mod.WorkDir == "" {
			mod.WorkDir = "."
		}
	}
}
