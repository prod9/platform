package framework

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/cuemod"
	"platform.prodigy9.co/framework/scaffold"
	"platform.prodigy9.co/framework/skel"
	"platform.prodigy9.co/gitops"
)

// PlatformInfra builds an infra repo's delivery image: it renders the repo's apps/ (CUE +
// .platform directives) to a manifest tree in-process, then packs that tree into a plain
// `FROM scratch` image as ONE layer — Flux's source-controller extracts a single layer
// from an OCI artifact, so a multi-layer image silently delivers one file and prune
// orphans the rest of the cluster. kustomize-controller applies the extracted YAML — no
// bespoke OCI pusher (see the infra-publishes-as-plain-image decision). It is a real
// framework module so infra delivery is the ordinary `publish` verb.
type PlatformInfra struct{}

var _ Framework = PlatformInfra{}

func (PlatformInfra) Name() string   { return "platform/infra" }
func (PlatformInfra) Layout() Layout { return LayoutBasic }

// Discover matches an "infra" glob on the directory name — "infra", "fi-infra",
// "bluepages-infra", "infra-stage9" all qualify. A directory marker like apps/ is a poor
// signal (an ordinary app may also have apps/), so identity is the name.
func (PlatformInfra) Discover(wd string) bool {
	return strings.Contains(filepath.Base(wd), "infra")
}

// Scaffold contributes the whole cluster baseline: the infra module, the baseline's default
// [vars] pins, the embedded component files (routed and resolved), a greenfield cue.mod, and
// the "rolling" strategy seed. There is no app-vs-infra branch anywhere — PlatformInfra simply
// contributes more, and owns resolving its own template holes.
func (i PlatformInfra) Scaffold(ctx context.Context, wd string, env scaffold.Env, inputs map[string]string) (scaffold.Spec, error) {
	files, err := skel.Files()
	if err != nil {
		return scaffold.Spec{}, err
	}
	if !cuemod.Present(wd) {
		files = append(files, scaffold.File{
			Path:    cuemod.ModuleFile + ".tmpl",
			Content: skel.CueMod,
			Mode:    0644,
		})
	}

	data, err := i.scaffoldData(wd, env, inputs)
	if err != nil {
		return scaffold.Spec{}, err
	}
	resolved, err := scaffold.Resolve(files, data)
	if err != nil {
		return scaffold.Spec{}, err
	}

	return scaffold.Spec{
		Module:   defaultModule(i, wd),
		Vars:     infraVars,
		Files:    resolved,
		Strategy: "rolling",
	}, nil
}

// ScaffoldVars asks for CUE_MOD_PREFIX — the cue.mod `module:` value and the prefix of
// every `import "<prefix>/defaults"` — only on a greenfield repo; an existing cue.mod is
// operator truth, read (never re-asked) in scaffoldData.
func (PlatformInfra) ScaffoldVars(wd string) []string {
	if cuemod.Present(wd) {
		return nil
	}
	return []string{"CUE_MOD_PREFIX"}
}

func (i PlatformInfra) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("platform/infra", &err)

	tree, err := gitops.Render(unit.WorkDir, gitops.RenderOptions{Vars: unit.Vars})
	if err != nil {
		return nil, err
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
	c := client.Container(dagger.ContainerOpts{Platform: dagger.Platform(unit.Arch)}).
		WithLabel("org.opencontainers.image.source", unit.RepositoryURL()).
		WithDirectory("/", dir)
	return c.Sync(ctx)
}

// scaffoldData builds the baseline's template data: the CUE module path (from an existing
// cue.mod or the greenfield CUE_MOD_PREFIX input), the linked dagger SDK version, the
// maintainer email (the cluster-issuer's ACME contact), and the flux self-sync image base
// derived from the repository. PlatformInfra needs the SDK version for the engine image ref, so an
// empty one is a hard error here rather than a tagless ref downstream.
func (i PlatformInfra) scaffoldData(wd string, env scaffold.Env, inputs map[string]string) (scaffold.Data, error) {
	if env.DaggerVersion == "" {
		return scaffold.Data{}, errors.New("infra scaffold: the linked dagger SDK version is unknown")
	}

	modulePath, err := i.modulePath(wd, inputs)
	if err != nil {
		return scaffold.Data{}, err
	}

	return scaffold.Data{
		"DaggerVersion":      env.DaggerVersion,
		"MaintainerEmail":    env.MaintainerEmail,
		"ModulePath":         modulePath,
		"ImageBase":          conf.InferImageBase(env.Repository),
		"CueLanguageVersion": cue.LanguageVersion(),
		"DefsModule":         infraDefsModule,
		"DefsVersion":        infraDefsVersion,
	}, nil
}

// modulePath resolves the CUE module path: an existing cue.mod wins (operator truth);
// otherwise the greenfield CUE_MOD_PREFIX input, validated as a legal CUE module path (its
// first segment must be a domain — contain a dot — which CUE requires).
func (PlatformInfra) modulePath(wd string, inputs map[string]string) (string, error) {
	if cuemod.Present(wd) {
		return cuemod.Path(wd)
	}

	prefix := inputs["CUE_MOD_PREFIX"]
	if err := cuemod.ValidatePath(prefix); err != nil {
		return "", fmt.Errorf("CUE_MOD_PREFIX %w", err)
	}
	return prefix, nil
}

// infraDefsModule is the infra-defs CUE dependency the baseline apps import;
// infraDefsVersion is the version a freshly-init'd infra repo pins into its cue.mod.
// v0.4.3 adds the Flux defs (#FluxOCIRepo/#FluxKustomization/#FluxReceiver) the flux-sync
// baseline composes; additive over v0.4.0's #NetworkPolicy access-grant + #pod_labels that
// platform.cue still needs to lock the engine's TCP port to the dispatcher.
const (
	infraDefsModule  = "prodigy9.co/defs@v0"
	infraDefsVersion = "v0.4.3"
)

// infraVars is the baseline's shipped [vars]: the version pins each baseline hook
// consumes. Keys are env-style (SCREAMING_SNAKE) — the preferred platform.toml form; render
// normalizes them to lowercase for both consumption routes, `\(cert_manager_version)` in
// directives and `@tag(cert_manager_version)` in CUE apps. Scaffold seeds these into a fresh
// platform.toml and merges on re-scaffold (new keys appended, operator values preserved).
// Pure interpolation inputs — component selection is not a var.
var infraVars = map[string]any{
	"CERT_MANAGER_VERSION":  "v1.20.2",
	"FLUX_VERSION":          "v2.8.8",
	"NGINX_GATEWAY_VERSION": "v2.6.7",
	"GATEWAY_API_VERSION":   "v1.5.1",

	// Per-deployment ingress hosts (render-time @tag holes): the platform server's own vanity
	// host and the Flux webhook-receiver route. prod9 self-host defaults; operators edit.
	"PLATFORM_HOSTNAME": "platform.prodigy9.co",
	"FLUX_HOSTNAME":     "flux.prodigy9.co",
}
