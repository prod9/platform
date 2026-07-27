package engine

import (
	"context"
	"time"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
	"platform.prodigy9.co/engine/observer"
	"platform.prodigy9.co/framework"
)

// Registry credentials for publishing built images, supplied via fx env config.
var (
	RegistryConfig         = fxconfig.Str("REGISTRY")
	RegistryUsernameConfig = fxconfig.Str("REGISTRY_USERNAME")
	RegistryPasswordConfig = fxconfig.Str("REGISTRY_PASSWORD")
)

// Run drives one unit's planned steps, one per Next. The engine imposes the sequence and
// nothing else: what a step does is the framework's business, and the container is only
// the medium each step hands to the next.
//
// It is a cursor rather than a single call because a build's observable structure is its
// steps — driving them one at a time is what lets each be timed and reported separately.
//
//	run := NewRun(sess, unit, caller)
//	for run.Next(ctx) {
//	}
//	result := run.Result()
type Run struct {
	sess *Session
	unit *framework.BuildUnit
	out  *observer.Outcome
	obs  observer.Observer

	steps []framework.Step
	next  int
	done  bool

	container *dagger.Container
	client    *dagger.Client
	err       error
}

// NewRun asks the unit's framework for its plan and reports every step to an accumulator it
// injects, plus caller when there is one. The fold is run-owned because Result mints its
// scalars from it. It does no engine work — a connection is taken at the first Next — so
// opening a run is free and never dials.
//
// A framework that plans nothing opens an already-failed run: it executes no step, reports
// no image, and hands back a result carrying ErrEmptyPlan. Rejecting it here rather than
// letting the cursor fall straight through is what makes Result's guarantee hold — an
// imageless success is not a state a caller can be handed.
func NewRun(sess *Session, unit *framework.BuildUnit, caller observer.Observer) *Run {
	obs, out := observer.Accumulate(caller)

	run := &Run{sess: sess, unit: unit, out: out, obs: obs, steps: unit.Framework.Plan(unit)}
	if len(run.steps) == 0 {
		run.err = ErrEmptyPlan
	}
	return run
}

// Next executes exactly one step and reports whether the run should continue. It returns
// false at the end of the plan and on the first failure; the error is held on the run
// rather than aborting anything else, so a caller driving several units loses only the one.
func (r *Run) Next(ctx context.Context) bool {
	if r.err != nil || r.next >= len(r.steps) {
		return r.finish()
	}

	step := r.steps[r.next]
	r.next++

	r.report(func(obs observer.Observer, at time.Time) {
		obs.StepStarted(r.unit.Name, step.String(), at)
	})
	container, err := r.execute(ctx, step)
	r.report(func(obs observer.Observer, at time.Time) {
		obs.StepDone(r.unit.Name, step.String(), at, err)
	})

	if err != nil {
		r.err = err
		return r.finish()
	}

	r.container = container
	if r.next >= len(r.steps) {
		return r.finish()
	}
	return true
}

// execute runs one step to completion on the run's connection, so that the step's whole
// cost — dialing included — lands inside the boundary the observer times.
func (r *Run) execute(ctx context.Context, step framework.Step) (*dagger.Container, error) {
	client, err := r.connect()
	if err != nil {
		return nil, err
	}

	container, err := r.unit.Framework.Execute(ctx, client, r.unit, step, r.container)
	if err != nil || container == nil {
		return nil, err
	}

	// Force the work here rather than letting it pile up lazily: the step boundary is
	// where a step's cost is attributable, and a deferred exec would be billed to whichever
	// later step happened to trigger it.
	return container.Sync(ctx)
}

// connect takes the run's connection from its session on first use and reuses it for every
// later step: a container is bound to the connection that built it, so the steps of one run
// cannot be spread across the fleet. The session spreads whole runs instead.
//
// It takes no context: the connection is dialed into the session's lifetime, never the
// step timeout that happens to be current when the first step runs.
func (r *Run) connect() (*dagger.Client, error) {
	if r.client != nil {
		return r.client, nil
	}

	client, err := r.sess.connect()
	if err != nil {
		return nil, err
	}

	r.client = client
	return client, nil
}

// finish ends the run, reporting it once however the cursor arrived here, and always
// returns false so every stopping path in Next can end with it.
func (r *Run) finish() bool {
	if r.done {
		return false
	}

	r.done = true
	if r.err == nil {
		r.report(func(obs observer.Observer, at time.Time) {
			obs.ImageBuilt(r.unit.Name, r.unit.ImageName, at)
		})
	}

	r.report(func(obs observer.Observer, at time.Time) { obs.RunDone(r.unit.Name, at, r.err) })
	return false
}

// Result joins the accumulator's fold with the live container this run owns, and is valid
// once Next has returned false. It is the only site those two halves meet — the scalars can
// only come from the stream and the container never leaves the run — so a result that claims
// a success with no image, or a hash from a build that never published, cannot be built.
//
// A failed run yields no container: what it holds is the last step that did succeed, which
// is a half-built image and never something a caller should publish or shell into.
func (r *Run) Result() BuildResult {
	container := r.container
	if r.out.Err != nil {
		container = nil
	}

	return BuildResult{
		Unit:      r.unit,
		Err:       r.out.Err,
		container: container,
		client:    r.client,
		out:       r.out,
		obs:       r.obs,
	}
}

// registryCreds is what the publish bracket needs from config, read once per verb rather
// than once per unit. An empty username means no auth is attached at all, and dagger pushes
// with the local docker credentials — that is the laptop path.
type registryCreds struct{ registry, username, password string }

func registryCredsFrom(cfg *fxconfig.Source) registryCreds {
	return registryCreds{
		registry: fxconfig.Get(cfg, RegistryConfig),
		username: fxconfig.Get(cfg, RegistryUsernameConfig),
		password: fxconfig.Get(cfg, RegistryPasswordConfig),
	}
}

// publish pushes what this run built, on the run's own connection — which is what lets the
// registry secret be minted by the session that owns the container it authenticates. It is a
// bracket around the step loop rather than a second pass over results: pushing is the
// engine's registry concern and no stack's build knowledge, and there is no reachable state
// where the container is in hand and its connection is not.
func (r *Run) publish(ctx context.Context, creds registryCreds) PublishResult {
	result := r.Result()
	if result.Err != nil {
		return PublishResult{BuildResult: result}
	}

	container := result.container
	if creds.username != "" {
		secret := r.client.SetSecret(RegistryPasswordConfig.Name(), creds.password)
		container = container.WithRegistryAuth(creds.registry, creds.username, secret)
	}

	hash, err := container.Publish(ctx, r.unit.ImageName)
	if err != nil {
		result.Err = err
		return PublishResult{BuildResult: result}
	}

	r.report(func(obs observer.Observer, at time.Time) {
		obs.Published(r.unit.Name, r.unit.ImageName, hash, at)
	})
	return PublishResult{
		BuildResult: result,
		ImageName:   r.out.Image,
		ImageHash:   r.out.Hash,
	}
}

// report is the one place the callback clock is read, so every report is stamped at the
// moment it happens. There is no nil check: NewRun guarantees an observer.
func (r *Run) report(emit func(obs observer.Observer, at time.Time)) {
	emit(r.obs, time.Now())
}
