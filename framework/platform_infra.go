package framework

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/gitops"
	"platform.prodigy9.co/framework/scaffold"
)

const (
	infraLuaPrefix = "vendor/infra-lua"
	infraLuaURL    = "https://github.com/prod9/infra-lua"

	// defaultsTemplate is the shared definitions module, which goes to the repo root.
	defaultsTemplate = "defaults.lua"
)

// PlatformInfra builds an infra repo's delivery image: it renders the repo's apps/*.lua to
// a manifest tree in-process, then packs that tree into a plain `FROM scratch` image as ONE
// layer — Flux's source-controller extracts a single layer from an OCI artifact, so a
// multi-layer image silently delivers one file and prune orphans the rest of the cluster.
// kustomize-controller applies the extracted YAML — no bespoke OCI pusher (see the
// infra-publishes-as-plain-image decision). It is a real framework module so infra
// delivery is the ordinary `publish` verb.
type PlatformInfra struct{ noScaffoldVars }

var _ Framework = PlatformInfra{}

func (PlatformInfra) Name() string   { return "platform/infra" }
func (PlatformInfra) Layout() Layout { return LayoutBasic }

// Discover matches an "infra" glob on the directory name — "infra", "fi-infra",
// "bluepages-infra", "infra-stage9" all qualify. A directory marker like apps/ is a poor
// signal (an ordinary app may also have apps/), so identity is the name.
func (PlatformInfra) Discover(wd string) bool {
	return strings.Contains(filepath.Base(wd), "infra")
}

// Vendors names infra-lua, whose templates are the cluster baseline Scaffold copies
// (docs/spec/scaffolding.md).
func (PlatformInfra) Vendors() []scaffold.Vendor {
	return []scaffold.Vendor{{Prefix: infraLuaPrefix, URL: infraLuaURL}}
}

// Scaffold contributes the infra module, the "rolling" strategy seed and the cluster
// baseline: every Lua template of the vendored infra-lua, routed and copied byte for
// byte, and the vars the templates declare (docs/spec/scaffolding.md, "The Infra
// framework scaffolds from vendored infra-lua").
func (i PlatformInfra) Scaffold(_ context.Context, wd string, _ scaffold.Env, _ map[string]string) (scaffold.Spec, error) {
	templatesDir := filepath.Join(wd, filepath.FromSlash(infraLuaPrefix), "templates")
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return scaffold.Spec{}, fmt.Errorf("platform/infra: read the infra-lua templates: %w", err)
	}

	var files []scaffold.File
	var declarations []declaration
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".lua" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(templatesDir, name))
		if err != nil {
			return scaffold.Spec{}, fmt.Errorf("platform/infra: %w", err)
		}
		declared, err := gitops.DeclaredVars(wd, path.Join(infraLuaPrefix, "templates", name))
		if err != nil {
			return scaffold.Spec{}, fmt.Errorf("platform/infra: %w", err)
		}
		declarations = append(declarations, declaration{template: name, vars: declared})
		files = append(files, scaffold.File{Path: templateDestination(name), Content: content, Mode: 0o644})
	}

	vars, err := mergeDeclarations(declarations)
	if err != nil {
		return scaffold.Spec{}, err
	}
	return scaffold.Spec{
		Module:   defaultModule(i, wd),
		Vars:     vars,
		Files:    files,
		Strategy: "rolling",
	}, nil
}

// templateDestination routes a template: the shared definitions module to the repo root,
// every app template to apps/ under its own name.
func templateDestination(name string) string {
	if name == defaultsTemplate {
		return name
	}
	return path.Join("apps", name)
}

// declaration is the vars one template declares, with their defaults.
type declaration struct {
	template string
	vars     map[string]any
}

// mergeDeclarations is the baseline's default [vars]: every template's declared vars, a
// name two templates give different defaults failing.
func mergeDeclarations(declarations []declaration) (map[string]any, error) {
	vars := map[string]any{}
	declaredBy := map[string]string{}
	for _, decl := range declarations {
		for name, value := range decl.vars {
			existing, seen := vars[name]
			if seen && existing != value {
				return nil, fmt.Errorf("platform/infra: %s declares %s = %v, but %s declares %v",
					decl.template, name, value, declaredBy[name], existing)
			}
			vars[name] = value
			declaredBy[name] = decl.template
		}
	}
	return vars, nil
}

// Plan is the one build step: the render runs in-process over committed files, vendored
// libraries included, and the image is a pure tree write.
func (PlatformInfra) Plan(*BuildUnit) []Step {
	return []Step{StepBuild}
}

func (PlatformInfra) Execute(_ context.Context, client *dagger.Client, unit *BuildUnit, step Step, _ *dagger.Container) (container *dagger.Container, err error) {
	defer errutil.Wrap("platform/infra", &err)

	switch step {
	case StepBuild:
		tree, renderErr := gitops.Render(unit.WorkDir, unit.Vars)
		if renderErr != nil {
			return nil, renderErr
		}

		// client.Container() with no From is an empty (scratch) image. The whole rendered
		// tree is staged as one Directory and mounted in a single WithDirectory, so the
		// pushed manifest carries exactly one tar+gzip layer holding every
		// <component>/<filename> — per-file WithNewFile calls would emit one layer each,
		// and Flux extracts only the first.
		dir := client.Directory()
		for _, path := range tree.Paths() {
			dir = dir.WithNewFile(path, string(tree[path]))
		}
		return client.Container(dagger.ContainerOpts{Platform: dagger.Platform(unit.Arch)}).
			WithLabel("org.opencontainers.image.source", unit.RepositoryURL()).
			WithDirectory("/", dir), nil

	default:
		return nil, unknownStep(step)
	}
}
