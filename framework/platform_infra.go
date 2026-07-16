package framework

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/cuemod"
	"platform.prodigy9.co/framework/scaffold"
	"platform.prodigy9.co/gitops"
)

// hasInfraName reports whether wd is an infra repo, matched by an "infra" glob on the
// directory name — "infra", "fi-infra", "bluepages-infra", "infra-stage9" all qualify. A
// directory marker like apps/ is a poor signal (an ordinary app may also have apps/), so
// identity is the name. It backs Infra.Discover — the framework's own discovery heuristic.
func hasInfraName(wd string) bool {
	return strings.Contains(filepath.Base(wd), "infra")
}

// Infra builds an infra repo's delivery image: it renders the repo's apps/ (CUE +
// .platform directives) to a manifest tree in-process, then packs that tree into a plain
// `FROM scratch` image. Publishing pushes it under the moving `latest` tag; Flux's
// OCIRepository extracts the layer via layerSelector and kustomize-controller applies the
// YAML — no bespoke OCI pusher (see the infra-publishes-as-plain-image decision). It is a
// real framework module so infra delivery is the ordinary `publish` verb.
type Infra struct{}

var _ Framework = Infra{}

func (Infra) Name() string   { return "platform/infra" }
func (Infra) Layout() Layout { return LayoutBasic }

// Discover matches by the infra-name glob (see hasInfraName), not a file marker.
func (Infra) Discover(wd string) bool {
	return hasInfraName(wd)
}

// Scaffold contributes the whole cluster baseline: the infra module, the baseline's default
// [vars] pins, the embedded component files (routed and resolved), a greenfield cue.mod, and
// the "rolling" strategy seed. There is no app-vs-infra branch anywhere — Infra simply
// contributes more, and owns resolving its own template holes.
func (i Infra) Scaffold(ctx context.Context, wd string, env scaffold.Env, inputs map[string]string) (scaffold.Spec, error) {
	files, err := infrabaseFiles()
	if err != nil {
		return scaffold.Spec{}, err
	}
	if !cuemod.Present(wd) {
		files = append(files, cueModFile())
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
		Vars:     maps.Clone(DefaultVars),
		Files:    resolved,
		Strategy: "rolling",
	}, nil
}

// cueModPrefixInput names the operator input carrying the CUE module path — the cue.mod
// `module:` value and the prefix of every `import "<prefix>/defaults"`. Asked only greenfield.
const cueModPrefixInput = "CUE_MOD_PREFIX"

// RequiredScaffoldInputs asks for the CUE module path only on a greenfield repo; an existing
// cue.mod is operator truth, read (never re-asked) in ScaffoldData.
func (Infra) RequiredScaffoldInputs(wd string) []string {
	if cuemod.Present(wd) {
		return nil
	}
	return []string{cueModPrefixInput}
}

// scaffoldData builds the baseline's template data: the CUE module path (from an existing
// cue.mod or the greenfield CUE_MOD_PREFIX input), the linked dagger SDK version, the
// maintainer email (the cluster-issuer's ACME contact), and the flux self-sync image base
// derived from the repository. Infra needs the SDK version for the engine image ref, so an
// empty one is a hard error here rather than a tagless ref downstream.
func (i Infra) scaffoldData(wd string, env scaffold.Env, inputs map[string]string) (scaffold.Data, error) {
	if env.DaggerVersion == "" {
		return scaffold.Data{}, errors.New("infra scaffold: the linked dagger SDK version is unknown")
	}

	modulePath, err := i.modulePath(wd, inputs)
	if err != nil {
		return scaffold.Data{}, err
	}

	return scaffold.Data{
		DaggerVersion:   env.DaggerVersion,
		MaintainerEmail: env.MaintainerEmail,
		ModulePath:      modulePath,
		ImageBase:       conf.InferImageBase(env.Repository),
	}, nil
}

// modulePath resolves the CUE module path: an existing cue.mod wins (operator truth);
// otherwise the greenfield CUE_MOD_PREFIX input, validated as a legal CUE module path (its
// first segment must be a domain — contain a dot — which CUE requires).
func (Infra) modulePath(wd string, inputs map[string]string) (string, error) {
	if cuemod.Present(wd) {
		return cuemod.Path(wd)
	}

	prefix := inputs[cueModPrefixInput]
	first, _, _ := strings.Cut(prefix, "/")
	if !strings.Contains(first, ".") {
		return "", fmt.Errorf("%s %q is not a valid CUE module path: its first segment must be a domain (contain a dot)", cueModPrefixInput, prefix)
	}
	return prefix, nil
}

func (i Infra) Build(ctx context.Context, client *dagger.Client, unit *BuildUnit) (container *dagger.Container, err error) {
	defer errutil.Wrap("platform/infra", &err)

	tree, err := gitops.Render(unit.WorkDir, gitops.RenderOptions{Vars: unit.Vars})
	if err != nil {
		return nil, err
	}

	// client.Container() with no From is an empty (scratch) image; add each rendered file
	// at its <component>/<filename> path. The published layer is a tar+gzip of exactly
	// these files, which is what Flux's layerSelector extracts.
	c := client.Container(dagger.ContainerOpts{Platform: dagger.Platform(unit.Arch)}).
		WithLabel("org.opencontainers.image.source", unit.Repository)
	for _, path := range tree.Paths() {
		c = c.WithNewFile("/"+path, string(tree[path]))
	}
	return c.Sync(ctx)
}
