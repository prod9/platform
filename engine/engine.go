// Package engine is the Dagger execution layer: an Engine pools clients over the
// discovered runner endpoints (like sql.DB — dialed lazily, reused, round-robin), and
// Build/Publish fan a project's units out across them. BuildAndPublish is the reusable
// build+tag+push unit shared by the publish command today and a tag-watch server later.
package engine

import (
	"context"
	"errors"
	"sync/atomic"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/framework"
	"platform.prodigy9.co/internal/buildlog"
)

// Engine is the process-wide handle to the Dagger engine fleet — like sql.DB, a
// concurrency-safe set of connections dialed lazily and reused. Build one from config,
// share it across the process, and carry it on a context with NewContext.
//
// It orchestrates two single-purpose units: runners (which endpoints exist) and clients
// (one reused, ping-checked client per endpoint). Engine itself holds no lock — only a
// round-robin cursor — so the good path is obvious: discover, pick, get.
type Engine struct {
	cfg     *fxconfig.Source
	runners *runners
	clients *clients
	cursor  atomic.Uint64
}

func New(cfg *fxconfig.Source) *Engine {
	return &Engine{
		cfg:     cfg,
		runners: newRunners(cfg),
		clients: newClients(),
	}
}

// Close tears down every dialed engine connection. Call once at process/server shutdown.
func (e *Engine) Close() error { return e.clients.Close() }

// Client picks the next endpoint round-robin over the currently-discovered set and returns
// a live client for it. Build/Publish use it per unit; commands that need ad-hoc Dagger
// access (ls, preview) call it directly.
func (e *Engine) Client(ctx context.Context) (*dagger.Client, error) {
	hosts, err := e.resolveHosts(ctx)
	if err != nil {
		return nil, err
	}

	next := e.cursor.Add(1) - 1
	host := hosts[int(next%uint64(len(hosts)))]
	return e.clients.Get(ctx, host)
}

// Clean prunes the build cache of every engine in the fleet, forcing subsequent builds to
// run cold. It sheds stale or poisoned cache entries a fresh checkout would not carry.
func (e *Engine) Clean(ctx context.Context) error {
	hosts, err := e.resolveHosts(ctx)
	if err != nil {
		return err
	}

	for _, host := range hosts {
		client, err := e.clients.Get(ctx, host)
		if err != nil {
			return err
		}
		if err := client.Engine().LocalCache().Prune(ctx); err != nil {
			return err
		}
	}
	return nil
}

// resolveHosts returns the discovered engine endpoints, or a single empty host — meaning
// the local engine — when none are discovered.
func (e *Engine) resolveHosts(ctx context.Context) ([]string, error) {
	hosts, err := e.runners.Hosts(ctx)
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return []string{""}, nil
	}
	return hosts, nil
}

type engineContextKey struct{}

// NewContext returns a context carrying eng, so downstream Build/Publish resolve it via
// FromContext — the same shape as fx/data's request-scoped *sqlx.DB.
func NewContext(ctx context.Context, eng *Engine) context.Context {
	return context.WithValue(ctx, engineContextKey{}, eng)
}

// FromContext returns the engine carried by ctx, panicking if none is present. Use it where the engine is a precondition.
func FromContext(ctx context.Context) *Engine {
	return ctx.Value(engineContextKey{}).(*Engine)
}

var ErrNoJobs = errors.New("engine: empty units list, nothing to do")

// BuildAndPublish composes Build and Publish over the engine carried by ctx: it builds
// every module matched by args, tags each image with tag, and publishes it — reusing the
// caller's engine instead of opening its own. It returns every publish result so a
// driver can record what shipped; both the local `publish` command and the platform
// server's build runner drive it.
func BuildAndPublish(ctx context.Context, cfg *conf.Model, modnames []string, tag string, obs Observer) ([]PublishResult, error) {
	units, err := framework.Units(cfg, modnames, cfg.PublishArch)
	if err != nil {
		return nil, err
	}

	for _, unit := range units {
		unit.ImageName = unit.ImageName + ":" + tag
	}

	builds, err := build(ctx, units, obs)
	if err != nil {
		return nil, err
	}
	results, err := Publish(ctx, builds...)
	if err != nil {
		return nil, err
	}

	var errs []error
	for _, result := range results {
		if result.Err != nil {
			errs = append(errs, result.Err)
		}
	}
	return results, errors.Join(errs...)
}

// Registry credentials for publishing built images, supplied via fx env config.
var (
	RegistryConfig         = fxconfig.Str("REGISTRY")
	RegistryUsernameConfig = fxconfig.Str("REGISTRY_USERNAME")
	RegistryPasswordConfig = fxconfig.Str("REGISTRY_PASSWORD")
)

type (
	BuildResult struct {
		Unit      *framework.BuildUnit
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

// Build builds every module matched by modnames (all of them when it is empty) on the
// engine carried by ctx. It constructs the units itself — resolving the arch from cfg —
// so callers never name a platform; what they need afterwards they read off
// BuildResult.Unit.
func Build(ctx context.Context, cfg *conf.Model, modnames []string, obs Observer) ([]BuildResult, error) {
	units, err := framework.Units(cfg, modnames, FromContext(ctx).buildArch(cfg))
	if err != nil {
		return nil, err
	}
	return build(ctx, units, obs)
}

// build runs every unit on the engine carried by ctx, fanning out one unit per goroutine
// and round-robining them across the discovered engine fleet. Every unit reports to the
// one observer, naming itself in each callback — the fan-in is the observer itself.
func build(ctx context.Context, units []*framework.BuildUnit, obs Observer) ([]BuildResult, error) {
	if len(units) == 0 {
		return nil, ErrNoJobs
	}
	eng := FromContext(ctx)

	m := &multiplexer[*framework.BuildUnit, BuildResult]{}
	m.Reset(units)
	return m.Start(func(idx int, unit *framework.BuildUnit) BuildResult {
		unitCtx, cancel := context.WithTimeout(ctx, unit.Timeout)
		defer cancel()

		run := NewRun(eng, unit, obs)
		for run.Next(unitCtx) {
		}

		container, err := run.Result()
		return BuildResult{Unit: unit, Container: container, Err: err, client: run.Client()}
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

	m := &multiplexer[BuildResult, PublishResult]{}
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
