package engine

import (
	"context"
	"time"

	"dagger.io/dagger"
	"platform.prodigy9.co/framework"
)

// Run drives one unit's planned steps, one per Next. The engine imposes the sequence and
// nothing else: what a step does is the framework's business, and the container is only
// the medium each step hands to the next.
//
// It is a cursor rather than a single call because a build's observable structure is its
// steps — driving them one at a time is what lets each be timed and reported separately.
//
//	run := NewRun(eng, unit, obs)
//	for run.Next(ctx) {
//	}
//	container, err := run.Result()
type Run struct {
	eng  *Engine
	unit *framework.BuildUnit
	obs  Observer

	steps []framework.Step
	next  int
	done  bool

	container *dagger.Container
	client    *dagger.Client
	err       error
}

// NewRun asks the unit's framework for its plan and reports every step to obs, which may
// be nil for a silent run. It does no engine work — a client is grabbed at the first Next
// — so opening a run is free and never dials.
func NewRun(eng *Engine, unit *framework.BuildUnit, obs Observer) *Run {
	return &Run{eng: eng, unit: unit, obs: obs, steps: unit.Framework.Plan(unit)}
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

	r.report(func(obs Observer, at time.Time) {
		obs.StepStarted(r.unit.Name, step.String(), at)
	})
	container, err := r.execute(ctx, step)
	r.report(func(obs Observer, at time.Time) {
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

// execute runs one step to completion on the run's client, so that the step's whole cost
// — dialing included — lands inside the boundary the observer times.
func (r *Run) execute(ctx context.Context, step framework.Step) (*dagger.Container, error) {
	client, err := r.dial(ctx)
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

// finish ends the run, reporting it once however the cursor arrived here, and always
// returns false so every stopping path in Next can end with it.
func (r *Run) finish() bool {
	if r.done {
		return false
	}

	r.done = true
	r.report(func(obs Observer, at time.Time) { obs.RunDone(r.unit.Name, at, r.err) })
	return false
}

// report is the one place a nil observer is skipped, and the one place the callback clock
// is read — so every report is stamped at the moment it happens.
func (r *Run) report(emit func(obs Observer, at time.Time)) {
	if r.obs == nil {
		return
	}
	emit(r.obs, time.Now())
}

// dial grabs the run's client on first use and reuses it for every later step: a
// container is bound to the client that built it, so the steps of one run cannot be spread
// across the fleet. Round-robin spreads whole runs instead.
func (r *Run) dial(ctx context.Context) (*dagger.Client, error) {
	if r.client != nil {
		return r.client, nil
	}
	client, err := r.eng.Client(ctx)
	if err != nil {
		return nil, err
	}
	r.client = client
	return client, nil
}

// Result is the built container and the run's error, valid once Next has returned false. A
// failed run yields no container: what it holds is the last step that did succeed, which is
// a half-built image and never something a caller should publish or shell into.
func (r *Run) Result() (*dagger.Container, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.container, nil
}

// Client is the engine client that built this run's container. Callers that keep operating
// on the container (preview's tunnel) must use it, since the container is bound to it.
func (r *Run) Client() *dagger.Client { return r.client }
