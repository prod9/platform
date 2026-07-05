package engine

import (
	"context"
	"errors"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/internal"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
)

var ErrNoJobs = errors.New("engine: empty units list, nothing to do")

// BuildAndPublish is the reusable publish unit: it opens an engine, builds every module
// matched by args, tags each image with tag, and publishes it. The local `publish` and
// `deploy` commands drive it now; a tag-watch platform server drives the same unit later.
func BuildAndPublish(cfg *project.Project, args []string, tag string) error {
	attempt, err := builder.AttemptFrom(cfg, args, builder.PublishBuild)
	if err != nil {
		return err
	}

	eng := New(fxconfig.Configure())
	defer eng.Close()
	ctx := NewContext(context.Background(), eng)

	for _, unit := range attempt.Units {
		unit.ImageName = unit.ImageName + ":" + tag
	}

	builds, err := Build(ctx, attempt)
	if err != nil {
		return err
	}
	results, err := Publish(ctx, builds...)
	if err != nil {
		return err
	}

	var errs []error
	for _, result := range results {
		if result.Err != nil {
			errs = append(errs, result.Err)
		}
	}
	return errors.Join(errs...)
}

// Registry credentials for publishing built images, supplied via fx env config.
var (
	RegistryConfig         = fxconfig.Str("REGISTRY")
	RegistryUsernameConfig = fxconfig.Str("REGISTRY_USERNAME")
	RegistryPasswordConfig = fxconfig.Str("REGISTRY_PASSWORD")
)

type (
	BuildResult struct {
		Unit      *builder.BuildUnit
		Container *dagger.Container
		Err       error

		// client is the engine client that built Container. Publish reuses it so the
		// registry secret comes from the same engine the container belongs to.
		client *dagger.Client
	}

	PublishResult struct {
		BuildResult
		ImageName string
		ImageHash string
	}
)

// Client returns the engine client that built this result's container. Callers that need to
// keep operating on the container (e.g. preview's tunnel) must use it, since the container
// is bound to the engine that produced it.
func (r BuildResult) Client() *dagger.Client { return r.client }

// Build runs every unit in attempt on the engine carried by ctx, fanning out one unit per
// goroutine and round-robining them across the discovered engine fleet.
func Build(ctx context.Context, attempt *builder.BuildAttempt) ([]BuildResult, error) {
	if len(attempt.Units) == 0 {
		return nil, ErrNoJobs
	}
	eng := FromContext(ctx)

	m := &internal.Multiplexer[*builder.BuildUnit, BuildResult]{}
	m.Reset(attempt.Units)
	return m.Start(func(idx int, unit *builder.BuildUnit) BuildResult {
		client, err := eng.Client(ctx)
		if err != nil {
			return BuildResult{Unit: unit, Err: err}
		}

		unitCtx, cancel := context.WithTimeout(ctx, unit.Timeout)
		defer cancel()

		container, err := unit.Builder.Build(unitCtx, client, unit)
		if err != nil {
			return BuildResult{Unit: unit, Err: err, client: client}
		}

		container, err = container.Sync(unitCtx)
		if err != nil {
			return BuildResult{Unit: unit, Err: err, client: client}
		}
		return BuildResult{Unit: unit, Container: container, client: client}
	}), nil
}

// Publish pushes every successfully-built container, reusing each build's own engine client
// so the registry secret is created by the engine that owns the container.
func Publish(ctx context.Context, builds ...BuildResult) ([]PublishResult, error) {
	if len(builds) == 0 {
		return nil, ErrNoJobs
	}
	eng := FromContext(ctx)
	registry := fxconfig.Get(eng.cfg, RegistryConfig)
	username := fxconfig.Get(eng.cfg, RegistryUsernameConfig)
	password := fxconfig.Get(eng.cfg, RegistryPasswordConfig)

	m := &internal.Multiplexer[BuildResult, PublishResult]{}
	m.Reset(builds)
	return m.Start(func(idx int, build BuildResult) PublishResult {
		if build.Err != nil {
			return PublishResult{BuildResult: build}
		}

		client := build.client
		if client == nil {
			c, err := eng.Client(ctx)
			if err != nil {
				build.Err = err
				return PublishResult{BuildResult: build}
			}
			client = c
		}

		container := build.Container
		if username != "" {
			secret := client.SetSecret(RegistryPasswordConfig.Name(), password)
			container = container.WithRegistryAuth(registry, username, secret)
		}

		hash, err := container.Publish(ctx, build.Unit.ImageName)
		if err != nil {
			build.Err = err
			return PublishResult{BuildResult: build}
		}

		buildlog.Image("publish", build.Unit.ImageName, hash)
		return PublishResult{
			BuildResult: build,
			ImageName:   build.Unit.ImageName,
			ImageHash:   hash,
		}
	}), nil
}
