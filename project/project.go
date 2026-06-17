package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"platform.prodigy9.co/internal/plog"
	"platform.prodigy9.co/internal/timeouts"
)

// ErrNoOpsImage is returned when `ops publish` has no target: neither an
// explicit [ops] image nor a repository to infer one from.
var ErrNoOpsImage = errors.New("project: no [ops] image and none inferable from repository")

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
		Ops      Ops                `toml:"ops"`
	}

	// Ops configures `ops publish` — where rendered infra manifests land as the
	// OCI config artifact. Both fields fall back to convention: Image is inferred
	// from Repository (github.com/x → ghcr.io/x), Tag defaults to "latest".
	Ops struct {
		Image string `toml:"image,omitempty"`
		Tag   string `toml:"tag,omitempty"`
	}

	Module struct {
		WorkDir string           `toml:"workdir,omitempty"` // the directory we'll be working in
		Timeout timeouts.Timeout `toml:"timeout,omitempty"`
		Builder string           `toml:"builder,omitempty"`

		// container settings
		Env         map[string]string `toml:"env,omitempty"`
		Port        *int              `toml:"port,omitempty"`
		CommandName string            `toml:"cmd,omitempty"`
		CommandArgs []string          `toml:"args,omitempty"`

		// build process settings
		AssetDirs   []string `toml:"asset_dirs,omitempty"`
		BuildDir    string   `toml:"build_dir,omitempty"`
		GoVersion   string   `toml:"go_version,omitempty"`
		ImageName   string   `toml:"image,omitempty"`
		PackageName string   `toml:"package,omitempty"`
	}
)

var (
	ProjectDefaults = &Project{
		Strategy: "timestamp",
		Platform: "auto",
		Excludes: []string{
			"*.docker",
			"*.local",
			".dockerignore",
			".git",
			".github",
			".gitignore",
			".idea",
			".svelte-kit",
			".vscode",
			"build",
			"deploy",
			"dist",
			"node_modules",
			"platform.toml",
			"target",
		},
		Modules: map[string]*Module{},
	}

	ModuleDefaults = &Module{
		Timeout: timeouts.From(1 * time.Minute),
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

	proj := &Project{}
	if _, err = toml.DecodeFile(path, proj); err != nil {
		return nil, err
	}

	proj.ConfigPath = path
	proj.ConfigDir = filepath.Dir(path)

	proj.assignDefaults()
	proj.assignEnvOverrides()
	proj.inferValues()
	return proj, nil
}

func (p *Project) assignDefaults() {
	for _, mod := range p.Modules {
		if mod.Timeout <= 0 {
			mod.Timeout = ModuleDefaults.Timeout
		}
	}
}

func (p *Project) assignEnvOverrides() {
	if platform, ok := os.LookupEnv("PLATFORM"); ok {
		plog.Config("platform", platform)
		p.Platform = platform
	}
}

func (p *Project) inferValues() {
	var base string
	if strings.HasPrefix(p.Repository, "github.com") {
		base = "ghcr.io" + p.Repository[10:]
	}

	singleModule := len(p.Modules) == 1
	for name, mod := range p.Modules {
		if mod.WorkDir == "" {
			mod.WorkDir = "."
		}
		if mod.ImageName == "" && p.Repository != "" {
			if singleModule {
				mod.ImageName = base
			} else {
				mod.ImageName = base + "/" + name
			}
		}
	}

	if p.Ops.Image == "" {
		p.Ops.Image = base
	}
	if p.Ops.Tag == "" {
		p.Ops.Tag = "latest"
	}
}

// Ref resolves the OCI reference `ops publish` pushes to. tag overrides the
// configured/default Tag when non-empty (e.g. a per-env publish).
func (o Ops) Ref(tag string) (string, error) {
	if o.Image == "" {
		return "", ErrNoOpsImage
	}
	if tag == "" {
		tag = o.Tag
	}
	return o.Image + ":" + tag, nil
}
