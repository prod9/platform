package project

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/internal/timeouts"
)

type (
	Project struct {
		ConfigPath string `toml:"-"`
		ConfigDir  string `toml:"-"`

		Maintainer string `toml:"maintainer"`
		Repository string `toml:"repository"`

		// LocalArch is the arch for local builds (build/preview/export/ls) —
		// "auto" tracks the host for fast native iteration. PublishArch is the arch
		// for server-bound builds (publish/deploy) — defaults to amd64 so an arm
		// laptop never ships an unrunnable image. Values are bare archs
		// (auto|amd64|arm64); the OS is always linux. Platform is the deprecated
		// single-target key (a full linux/arch string), kept for backward
		// compatibility; it seeds LocalArch when unset (see assignDefaults).
		Platform    string `toml:"platform,omitempty"`
		LocalArch   string `toml:"local_arch,omitempty"`
		PublishArch string `toml:"publish_arch,omitempty"`

		Strategy string `toml:"strategy"`

		Excludes []string           `toml:"excludes"`
		Modules  map[string]*Module `toml:"modules,omitempty"`
		Ops      Ops                `toml:"ops"`
	}

	Module struct {
		WorkDir string           `toml:"workdir,omitempty"` // the directory we'll be working in
		Timeout timeouts.Timeout `toml:"timeout,omitempty"`
		Builder string           `toml:"builder,omitempty"`

		// container settings
		Env         map[string]string `toml:"env,omitempty"`
		Port        int               `toml:"port,omitempty"`
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
		Strategy:    "datestamp",
		LocalArch:   "auto",
		PublishArch: "amd64",
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
		wd_, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		wd = wd_
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
	// Backward compatibility: the deprecated single-target `platform` key (a full
	// linux/arch string) seeds the local arch when no explicit `local_arch` is given.
	if p.Platform != "" && p.LocalArch == "" {
		p.LocalArch = p.Platform
	}
	if p.LocalArch == "" {
		p.LocalArch = ProjectDefaults.LocalArch
	}
	if p.PublishArch == "" {
		p.PublishArch = ProjectDefaults.PublishArch
	}

	for _, mod := range p.Modules {
		if mod.Timeout <= 0 {
			mod.Timeout = ModuleDefaults.Timeout
		}
	}
}

func (p *Project) assignEnvOverrides() {
	if platform, ok := os.LookupEnv("PLATFORM"); ok {
		buildlog.Config("platform", platform)
		p.LocalArch = platform
		p.PublishArch = platform
	}
}

// InferOpsImage derives the OCI image base from a repository address: a github.com path maps
// to its ghcr.io mirror (github.com/org/repo → ghcr.io/org/repo). Empty for anything else —
// callers that require an image (e.g. the flux self-sync URL) treat empty as unset.
func InferOpsImage(repository string) string {
	if strings.HasPrefix(repository, "github.com") {
		return "ghcr.io" + repository[10:]
	}
	return ""
}

func (p *Project) inferValues() {
	base := InferOpsImage(p.Repository)

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

// NormalizeVars lowercases every [ops.vars] key to the canonical consumption form. platform.toml
// prefers env-style keys (NGINX_GATEWAY_VERSION); both render routes consume the lowercase
// derivation — `\(nginx_gateway_version)` in directives, `@tag(nginx_gateway_version)` in CUE.
// Pure name normalization; values and their types are untouched.
func NormalizeVars(vars map[string]any) map[string]any {
	out := make(map[string]any, len(vars))
	for name, val := range vars {
		out[strings.ToLower(name)] = val
	}
	return out
}
