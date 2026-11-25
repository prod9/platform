package framework

import (
	"context"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"fx.prodigy9.co/errutil"
	"platform.prodigy9.co/framework/gitops"
	"platform.prodigy9.co/framework/scaffold"
	"platform.prodigy9.co/rocks"
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

// Scaffold contributes the infra module and the "rolling" strategy seed. The cluster
// baseline comes from the infra-lua rock's templates, which init does not fetch yet
// (docs/spec/scaffolding.md).
func (i PlatformInfra) Scaffold(_ context.Context, wd string, _ scaffold.Env, _ map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{
		Module:   defaultModule(i, wd),
		Strategy: "rolling",
	}, nil
}

// Plan splits the render's one network-bound stage off from its evaluation. Neither step
// runs an exec — the render happens in-process and the image is a pure tree write — so both
// are silent, but they are timed apart, which is the whole point: fetching the rocks is the
// part that reaches the network and can stall.
func (PlatformInfra) Plan(*BuildUnit) []Step {
	return []Step{StepDeps, StepBuild}
}

func (PlatformInfra) Execute(ctx context.Context, client *dagger.Client, unit *BuildUnit, step Step, in *dagger.Container) (container *dagger.Container, err error) {
	defer errutil.Wrap("platform/infra", &err)

	switch step {
	case StepDeps:
		// Host-side work: it fills the rock cache the render then reads. A step is under no
		// obligation to touch the container, so this one passes its input straight along —
		// here that is the nil the first step always receives.
		_, err = fetchRocks(ctx, unit)
		return in, err

	case StepBuild:
		// The deps step filled the cache, so this fetch only finds the cached directories.
		rockDirs, fetchErr := fetchRocks(ctx, unit)
		if fetchErr != nil {
			return nil, fetchErr
		}
		tree, renderErr := gitops.Render(unit.WorkDir, unit.Vars, rockDirs)
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

func fetchRocks(ctx context.Context, unit *BuildUnit) ([]string, error) {
	fetch := rocks.FetchPins{
		Pins:     unit.Rocks,
		Username: unit.RegistryUsername,
		Password: unit.RegistryPassword,
	}
	return fetch.Run(ctx)
}
