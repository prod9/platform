// Package conf models platform.toml — the per-repo build/delivery config. Load walks up
// to the file and resolves it (defaults, PLATFORM env overrides, inferred values);
// Generate writes a fresh file and MergeVars folds default [vars] in surgically,
// preserving operator edits.
package conf

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
	Model struct {
		ConfigPath string `toml:"-"`
		ConfigDir  string `toml:"-"`

		Maintainer string `toml:"maintainer"`
		Repository string `toml:"repository"`

		// LocalArch is the arch for local builds (build/preview/export/ls) —
		// "auto" tracks the host for fast native iteration. PublishArch is the arch
		// for server-bound builds (publish) — defaults to amd64 so an arm
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

		// Vars is the verbatim DSL \(var) table (top-level [vars]) — a generic open
		// map whose values keep their TOML type (string/int/bool). Pure passthrough:
		// render feeds it project-wide across apps/ (CUE @tag holes and directive
		// \(var) interpolation); no defaults or inference beyond the baseline seed.
		Vars map[string]any `toml:"vars,omitempty"`
	}

	Module struct {
		WorkDir   string           `toml:"workdir,omitempty"` // the directory we'll be working in
		Timeout   timeouts.Timeout `toml:"timeout,omitempty"`
		Framework string           `toml:"framework,omitempty"`

		// Builder is the deprecated legacy key for Framework — read as an alias
		// (assignDefaults folds it in and clears it), never written.
		Builder string `toml:"builder,omitempty"`

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
	ModelDefaults = &Model{
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

func Load(wd string) (*Model, error) {
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

	proj := &Model{}
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

func (p *Model) assignDefaults() {
	// Backward compatibility: the deprecated single-target `platform` key (a full
	// linux/arch string) seeds the local arch when no explicit `local_arch` is given.
	if p.Platform != "" && p.LocalArch == "" {
		p.LocalArch = p.Platform
	}
	if p.LocalArch == "" {
		p.LocalArch = ModelDefaults.LocalArch
	}
	if p.PublishArch == "" {
		p.PublishArch = ModelDefaults.PublishArch
	}

	for name, mod := range p.Modules {
		if mod.Timeout <= 0 {
			mod.Timeout = ModuleDefaults.Timeout
		}

		// Fold the deprecated `builder` key into `framework`; the canonical key wins
		// when both appear. Cleared so a re-encode emits only `framework`. The
		// deprecation note fires whenever the old key is read, folded or dropped.
		if mod.Builder != "" {
			buildlog.Config("modules."+name+".builder", "deprecated — rename the key to `framework`")
			if mod.Framework == "" {
				mod.Framework = mod.Builder
			}
			mod.Builder = ""
		}
	}
}

func (p *Model) assignEnvOverrides() {
	if platform, ok := os.LookupEnv("PLATFORM"); ok {
		buildlog.Config("platform", platform)
		p.LocalArch = platform
		p.PublishArch = platform
	}
}

// InferImageBase derives the OCI image base from a repository address: a github.com path maps
// to its ghcr.io mirror (github.com/org/repo → ghcr.io/org/repo). Empty for anything else —
// callers that require an image (per-module ImageName, the flux self-sync ref) treat empty
// as unset.
func InferImageBase(repository string) string {
	if strings.HasPrefix(repository, "github.com") {
		return "ghcr.io" + repository[10:]
	}
	return ""
}

func (p *Model) inferValues() {
	base := InferImageBase(p.Repository)

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
}

// NormalizeVars lowercases every [vars] key to the canonical consumption form. platform.toml
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
