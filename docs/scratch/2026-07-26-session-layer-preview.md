<!-- not spec/decision because: unruled code preview of the Session layer; becomes a spec
update to engine.md once chakrit rules -->

# Session layer — code preview

Supersedes the A–D options in
[`2026-07-26-session-lifetime-options.md`](2026-07-26-session-lifetime-options.md): none of
them named the missing layer. `*dagger.Client` **is a session**, not a pooled connection —
containers are handles into it — so the pool was an `sql.DB` abstraction applied to a
non-fungible resource. Naming the session is the fix; the lifetime bug falls out.

## Package shape after

| file | holds |
|-----------------|--------------------------------------------------------------|
| `runners.go`    | the fleet as a stateless roster: `Hosts`, `Dial`, and the resolver seam |
| `session.go`    | `NewSession`, `Session` — the lifetime — plus `Build`, `BuildAndPublish`, `Clean` |
| `results.go`    | `BuildResult`, `PublishResult` — pure data                    |
| `run.go`        | `Run`, unchanged but for taking a `*Session`                  |
| `observer.go`, `multiplexer.go`, `arch.go` | unchanged (`buildArch` moves to `*Session`) |

`engine.go` disappears: `Engine`, the `clients` pool, the round-robin cursor, and
`NewContext`/`FromContext` all go with it. The engine was being smuggled through `context`
and type-asserted (`engine.go:100-102`); with a `Session` the caller holds, that hidden
dependency evaporates.

## `runners.go`

The `runners` **type is deleted**, not converted: once `Hosts` reads cfg, the struct's only
remaining job was the injected `lookup` seam, and `dns`/`port` were cfg reads it cached for
no reason. Free functions plus one package-level seam replace it. Package-level *state* was
rejected — `cfg` is built per caller (`fxconfig.Configure()` per command, `srv` its own), so
a global roster would need init-order coupling and would make two sessions share a roster
neither owns, and parallel tests would lose the isolation they get free today.

```go
// The roster is not a cache: it resolves endpoints on demand and dials them, and holds
// nothing between calls. Two calls a second apart may see different engines as pods come
// and go — that is the point, not a defect.

var lookupHost = net.DefaultResolver.LookupHost   // swapped in tests, never at runtime

// cfgFrom takes the config off the context, falling back to a fresh Configure() when the
// caller seeded none. Configure() is NewSource(defaultSource.provider, defaultSource.vars)
// — the same value a command would have built, with no env read at call time.
func cfgFrom(ctx context.Context) *fxconfig.Source {
	if cfg := fxconfig.FromContext(ctx); cfg != nil {
		return cfg
	}
	return fxconfig.Configure()
}

// Hosts resolves the configured engine endpoints via DNS — no k8s API, no RBAC. An empty
// result means none are configured, which is the caller's cue to fall back to local.
func Hosts(ctx context.Context) ([]string, error)

// Dial connects to one uniformly-chosen endpoint, or to a local auto-provisioned engine
// when none are configured. Uniform choice replaces the round-robin cursor: the same
// distribution over a run of picks, with no state kept between calls.
func Dial(ctx context.Context) (*dagger.Client, error)
```

## `session.go`

```go
// Session is the span during which containers are usable. Every container is a handle into
// the connection that built it, and every connection is dialed into the session's own
// context — so a container stays valid for exactly as long as its session, and dies with
// Close. Nothing else ends a session: no caller's cancellation reaches one, because a
// per-unit timeout that killed the connection would invalidate containers the caller is
// still holding.
//
// A session opens as many connections as its work needs and closes them together. It is
// safe for concurrent use.
type Session struct {
	ctx context.Context   // carries both the lifetime and the config

	mu    sync.Mutex
	conns []*dagger.Client
}

// NewSession opens a session on the fleet. It dials nothing: connections are made as runs
// need them, so opening is free and a session that never builds never touches an engine.
// ctx is the lifetime every connection will be dialed into — a process scope, never a
// request or per-unit one — and it carries the config the roster reads.
func NewSession(ctx context.Context) *Session {
	return &Session{ctx: ctx}
}

// connect adds one connection to the session and hands it over. A run calls this once and
// reuses what it gets, because a container is bound to the connection that built it; a
// session driving many runs calls it many times, and that is what spreads runs across the
// fleet.
func (s *Session) connect() (*dagger.Client, error) {
	client, err := Dial(s.ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns = append(s.conns, client)
	return client, nil
}

// Close ends every connection this session opened, and with them every container built on
// one. Call it once, deferred, by whoever opened the session.
func (s *Session) Close() error

// Build builds every module matched by modnames (all of them when empty), one run per unit,
// each on its own connection. Results are data: they outlive the run but not the session.
func (s *Session) Build(ctx context.Context, cfg *conf.Model, modnames []string, obs Observer) ([]BuildResult, error) {
	// buildArch moves off *Engine onto *Session; it reads CI through cfgFrom(s.ctx), not a
	// stored config field.
	units, err := framework.Units(cfg, modnames, s.buildArch(cfg))
	if err != nil {
		return nil, err
	}
	return s.build(ctx, units, obs, nil)
}

// BuildAndPublish builds every matched module and pushes each image as its run finishes.
// Publishing is a bracket around the run, not a second pass: the connection that built the
// container is still in hand, so the registry secret is minted on the session that owns the
// container it authenticates.
func (s *Session) BuildAndPublish(ctx context.Context, cfg *conf.Model, modnames []string, tag string, obs Observer) ([]PublishResult, error)

// Clean prunes every fleet engine's local cache, forcing later builds to run cold.
func (s *Session) Clean(ctx context.Context) error

// Unsafe hands over a raw connection, and the name is the warning: past here a caller
// expresses Dagger operations the session otherwise owns. ls's ad-hoc container reaches
// through it; no new caller joins it.
func (s *Session) Unsafe() (*dagger.Client, error) { return s.connect() }
```

The one internal that carries both paths, with the publish bracket where the connection is
still a local:

```go
// build runs every unit, one goroutine each, and optionally publishes each as it finishes.
// The per-unit timeout bounds a unit's steps and nothing else — it never reaches a dial, so
// a unit ending cannot close a connection the caller still needs.
func (s *Session) build(ctx context.Context, units []*framework.BuildUnit, obs Observer, push *pushSpec) ([]BuildResult, error) {
	if len(units) == 0 {
		return nil, ErrNoJobs
	}

	m := &multiplexer[*framework.BuildUnit, BuildResult]{}
	m.Reset(units)
	return m.Start(func(idx int, unit *framework.BuildUnit) BuildResult {
		unitCtx, cancel := context.WithTimeout(ctx, unit.Timeout)
		defer cancel()

		run := NewRun(s, unit, obs)
		for run.Next(unitCtx) {
		}
		return run.Result()
	}), nil
}
```

## `results.go`

```go
// BuildResult is what a run leaves behind: scalars minted from the observed stream, plus
// the image it produced. The container is the session's, not the result's — the result
// records which image exists, the session decides how long it can be touched.
type BuildResult struct {
	Unit *framework.BuildUnit
	Err  error

	container *dagger.Container
	out       *outcome
	obs       Observer
}

// UnsafeContainer hands over the built image, and the name is the warning: past here a
// caller expresses container operations the engine otherwise owns. export's file, exec's
// shell and preview's tunnel reach through it; no new caller joins them. Valid only while
// the session that built it is open. A failed run yields nothing.
func (r BuildResult) UnsafeContainer() *dagger.Container { return r.container }
```

The `client` field is **gone**, not renamed. It existed for exactly one line —
`client.SetSecret` at `engine.go:248` — and only because publish was a separate pass in time.
As a bracket, the connection is a local variable at the moment the secret is minted. That
also deletes the `engine.go:236-244` fallback, which dialed a *different* connection when
`build.client` was nil and attached a secret from one session to a container from another.

`engine.Publish` is **deleted** rather than made a method: it is exported with zero callers
outside the package, and its `...BuildResult` argument can only be minted by `Run.Result`,
so possessing it means you already built in that session. "Publish something built earlier"
is a registry-to-registry copy — no container, no engine, not this function.

## Call sites

```go
// cmd/export.go — and build, exec, preview, publish, clean, ls all take this shape.
ctx := fxconfig.NewContext(context.Background(), fxconfig.Configure())
sess := engine.NewSession(ctx)
defer sess.Close()

results, err := sess.Build(ctx, cfg, args, newObserver())
// ...
container := result.UnsafeContainer()
if _, err := container.Export(ctx, outname); err != nil { ... }
```

`engine.NewContext(ctx, eng)` disappears from all five commands. `cmd/ls.go:46` — ledger
28's open item — becomes `sess.Unsafe()` and stops being a special case: it is the same
one-door hatch every other ad-hoc caller uses.

## Open, not decided here

- **Liveness.** The old pool re-pinged and redialed a vanished engine (`clients.go:44-55`).
  A command-length session never needs it. `srv` runs for days across changing pods — so
  either `connect()` gains a retry, or `srv` opens a session per build and the question
  dissolves. Decide before `srv`'s build worker lands, not after.
- **`Session.ctx` on a struct.** Go style dislikes a stored context. It is the honest
  encoding here — the struct *is* a lifetime — but it deserves a stated exception rather
  than passing unremarked.
- **`Hosts`/`Dial` exported.** Both are free functions on the roster; nothing outside the
  package needs them today. Unexport unless a caller appears.
