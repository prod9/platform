package engine

import (
	"errors"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/internal"
	"platform.prodigy9.co/internal/buildlog"
)

var ErrNoJobs = errors.New("engine: empty units list, nothing to do")

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

		// pool is the per-unit view that built Container. Publish reuses it so the registry
		// secret comes from the same engine client the container belongs to.
		pool *Pool
	}

	PublishResult struct {
		BuildResult
		ImageName string
		ImageHash string
	}
)

func Build(pool *Pool, attempt *builder.BuildAttempt) ([]BuildResult, error) {
	if len(attempt.Units) == 0 {
		return nil, ErrNoJobs
	}

	m := &internal.Multiplexer[*builder.BuildUnit, BuildResult]{}
	m.Reset(attempt.Units)
	return m.Start(func(idx int, unit *builder.BuildUnit) BuildResult {
		eng := pool.bind(idx)
		ctx, cancel := eng.unitContext(unit.Timeout)
		defer cancel()

		container, err := unit.Builder.Build(eng, unit)
		if err != nil {
			return BuildResult{Unit: unit, Container: nil, Err: err, pool: eng}
		}

		container, err = container.Sync(ctx)
		if err != nil {
			return BuildResult{Unit: unit, Container: nil, Err: err, pool: eng}
		} else {
			return BuildResult{Unit: unit, Container: container, Err: nil, pool: eng}
		}
	}), nil
}

func Publish(pool *Pool, builds ...BuildResult) ([]PublishResult, error) {
	if len(builds) == 0 {
		return nil, ErrNoJobs
	}

	fxcfg := fxconfig.Configure()
	registry := fxconfig.Get(fxcfg, RegistryConfig)
	username := fxconfig.Get(fxcfg, RegistryUsernameConfig)
	password := fxconfig.Get(fxcfg, RegistryPasswordConfig)

	m := &internal.Multiplexer[BuildResult, PublishResult]{}
	m.Reset(builds)
	return m.Start(func(idx int, build BuildResult) PublishResult {
		if build.Err != nil {
			return PublishResult{BuildResult: build}
		}

		// the container is bound to the engine that built it — its registry secret must
		// be created by that same engine's client, not an arbitrary pool member.
		eng := build.pool
		if eng == nil {
			eng = pool.bind(idx)
		}

		container := build.Container
		if username != "" {
			secret := eng.Client().SetSecret(RegistryPasswordConfig.Name(), password)
			container = container.WithRegistryAuth(registry, username, secret)
		}

		hash, err := container.Publish(eng.Context(), build.Unit.ImageName)
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
