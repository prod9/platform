// Package engine is the Dagger execution layer. It has two pieces and only one of them
// carries a lifetime: the roster in this file — which engine endpoints exist and how to dial
// one — and Session (session.go), the span during which the containers a build produced are
// usable. There is deliberately no client pool: a *dagger.Client is a session rather than a
// fungible connection, so pooling one is the abstraction this package rejects.
package engine

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"sort"

	"dagger.io/dagger"
	"fx.prodigy9.co/cmd/prompts"
	fxconfig "fx.prodigy9.co/config"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine/observer"
	"platform.prodigy9.co/framework"
	"platform.prodigy9.co/internal/buildlog"
)

var (
	// DaggerEngineConfig is the headless-Service DNS name of the Dagger engine pool, e.g.
	// dagger-engine.platform.svc.cluster.local. Unset means no remote engines are configured
	// and the roster falls back to a local auto-provisioned one — an explicit operator choice,
	// never inferred from the environment.
	DaggerEngineConfig = fxconfig.Str("DAGGER_ENGINE")
	// DaggerEnginePortConfig is the engine pod port; the default mirrors apps/dagger-engine.cue.
	DaggerEnginePortConfig = fxconfig.IntDef("DAGGER_ENGINE_PORT", 1234)

	ErrNoJobs = errors.New("engine: empty units list, nothing to do")

	// lookupHost is the resolver seam: swapped in tests, never at runtime.
	lookupHost = net.DefaultResolver.LookupHost
)

type (
	BuildResult struct {
		Unit *framework.BuildUnit
		Err  error

		// container is the image this run produced; it leaves the package only through
		// UnsafeContainer and is valid only while the session that built it is open. client
		// is the connection it is bound to.
		container *dagger.Container
		client    *dagger.Client

		// out and obs are the run's report, carried past the run so a publish continues the
		// same stream and mints its scalars from the same fold. Only Run.Result fills them
		// in — a BuildResult is never assembled anywhere else.
		out *observer.Outcome
		obs observer.Observer
	}

	PublishResult struct {
		BuildResult
		ImageName string
		ImageHash string
	}
)

// UnsafeContainer hands over the built image, and the name is the warning: past here a
// caller expresses container operations, which the engine otherwise owns exclusively.
// export's file, exec's shell and preview's tunnel reach through it until the engine grows
// verbs of its own for them; no new caller joins them.
//
// The container carries its own client internally, so Export, WithExec and Publish need
// nothing else. A tunnel is not a container operation — Host().Tunnel needs a client this
// door does not hand over, so preview forwards its port with Service.Up instead. Valid only
// while the session that built it is open, and a failed run yields none.
func (r BuildResult) UnsafeContainer() *dagger.Container { return r.container }

// hosts resolves the configured engine endpoints via DNS — no k8s API, no RBAC — and
// reports only what it finds: an empty slice when DAGGER_ENGINE is unset or resolves to
// nothing, and a real error when the lookup itself fails. Falling back to a local engine is
// not its decision. The resolver caches per the record TTL, so a new pod becomes selectable
// as soon as DNS reflects it; nothing else is remembered, and two calls a second apart may
// legitimately see different engines as pods come and go.
func hosts(ctx context.Context) ([]string, error) {
	cfg := cfgFrom(ctx)
	dns := fxconfig.Get(cfg, DaggerEngineConfig)
	if dns == "" {
		return nil, nil
	}

	addrs, err := lookupHost(ctx, dns)
	if err != nil {
		return nil, fmt.Errorf("resolving dagger engines at %s: %w", dns, err)
	}

	port := fxconfig.Get(cfg, DaggerEnginePortConfig)
	endpoints := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		endpoints = append(endpoints, fmt.Sprintf("tcp://%s:%d", addr, port))
	}

	sort.Strings(endpoints)
	return endpoints, nil
}

// dial connects to one uniformly-chosen endpoint, or to a local auto-provisioned engine when
// none are configured.
func dial(ctx context.Context) (*dagger.Client, error) {
	endpoints, err := hosts(ctx)
	if err != nil {
		return nil, err
	}
	return dialHost(ctx, pick(endpoints))
}

// pick chooses one endpoint uniformly at random, or the empty host — the local engine — when
// none are configured. Random replaces the round-robin cursor: the distribution over a run
// of picks is the same and it keeps no state between calls, which is what lets the roster
// stay a roster.
func pick(endpoints []string) string {
	if len(endpoints) == 0 {
		return ""
	}
	return endpoints[rand.IntN(len(endpoints))]
}

// dialHost connects to the engine at host. An empty host carries no runner host, so dagger
// auto-provisions and reuses the local engine — that is how the roster asks for "local".
func dialHost(ctx context.Context, host string) (*dagger.Client, error) {
	opts := []dagger.ClientOpt{dagger.WithLogOutput(buildlog.OutputForDagger())}
	if host != "" {
		opts = append(opts, dagger.WithRunnerHost(host))
	}
	return dagger.Connect(ctx, opts...)
}

// cfgFrom takes the config off ctx, falling back to a fresh Configure() when the caller
// seeded none — the same value a command would have built, with no env read at call time.
// Config may ride a context because it is inert and has no lifetime; a Session owns
// connections and so is always passed explicitly.
func cfgFrom(ctx context.Context) *fxconfig.Source {
	if cfg := fxconfig.FromContext(ctx); cfg != nil {
		return cfg
	}
	return fxconfig.Configure()
}

// buildArch answers the only arch question there is: does this image outlive the box that
// built it? A plain build is discarded here and takes the host arch for speed — except
// under CI, where the build exists to be pushed and must carry the servers' arch. The rule
// is unexported because it is only ever an input to an entrypoint that is about to build.
func (s *Session) buildArch(cfg *conf.Model) string {
	if fxconfig.Get(cfgFrom(s.ctx), prompts.CIConfig) {
		return cfg.PublishArch
	}
	return cfg.LocalArch
}
