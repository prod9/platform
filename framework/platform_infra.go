package framework

import (
	"context"
	"maps"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
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

// Scaffold contributes the whole cluster baseline: the infra module, the baseline's
// default [vars] pins, the embedded component files (routed, holes unresolved), a
// greenfield cue.mod, and the "rolling" strategy seed. There is no app-vs-infra branch
// anywhere — Infra simply contributes more.
func (i Infra) Scaffold(ctx context.Context, wd string) (scaffold.Spec, error) {
	files, err := baselineFiles()
	if err != nil {
		return scaffold.Spec{}, err
	}
	if !cuemod.Present(wd) {
		files = append(files, cueModFile())
	}

	return scaffold.Spec{
		Module:       defaultModule(i, wd),
		Vars:         maps.Clone(DefaultVars),
		Files:        files,
		Strategy:     "rolling",
		ImportPrefix: "example.com",
	}, nil
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
