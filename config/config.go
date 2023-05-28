package config

import (
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	DefaultTimeout = 5 * time.Minute
)

type Config struct {
	Maintainer     string `toml:"maintainer"`
	ConfigPath     string
	TargetPlatform string `toml:"target_platform"`
	SourceURL      string `toml:"source_url"`

	Excludes []string           `toml:"excludes"`
	Modules  map[string]*Module `toml:"modules"`
}

type Module struct {
	WD      string        `toml:"wd"` // the directory we'll be working in
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
	_, err = toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, err
	}

	cfg.ConfigPath = path
	cfg.assignDefaults()
	return cfg, nil
}

func (c *Config) assignDefaults() {
	for _, mod := range c.Modules {
		if mod.Timeout <= 0 {
			mod.Timeout = DefaultTimeout
		}
	}
}
