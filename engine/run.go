package engine

import (
	"context"
	"errors"
	"time"

	"dagger.io/dagger"
	"platform.prodigy9.co/engine/observer"
	"platform.prodigy9.co/framework"
)

var (
	// syncContainer keeps cursor tests independent of a live Dagger engine.
	syncContainer = func(ctx context.Context, container *dagger.Container) (*dagger.Container, error) {
		return container.Sync(ctx)
	}

	// captureOutput reads what a finished step printed. It is a var so a test can drive the
	// report path without a live engine; nothing but a test replaces it.
	captureOutput = func(ctx context.Context, container *dagger.Container, err error) (stdout, stderr string) {
		// A failed step yields no container — dagger hands its streams over on the error
		// instead — and that is the output most worth having.
		var execErr *dagger.ExecError
		if errors.As(err, &execErr) {
			return execErr.Stdout, execErr.Stderr
		}
		if container == nil {
			return "", ""
		}

		// A step whose last operation was not an exec has nothing to read, which dagger
		// reports as an error: an absence of output, not a failure to report it.
		stdout, _ = container.Stdout(ctx)
		stderr, _ = container.Stderr(ctx)
		return stdout, stderr
	}
)

// Run is the internal cursor over a prepared module's framework steps.
// Its enclosing moduleOperation owns configuration, publication, and RunDone.
type Run struct {
	unit      *framework.BuildUnit
	obs       observer.Observer
	steps     []framework.Step
	next      int
	container *dagger.Container
	client    *dagger.Client
	err       error
}

func (r *Run) Next(ctx context.Context) bool {
	if r.err != nil || r.next >= len(r.steps) {
		return false
	}

	step := r.steps[r.next]
	r.next++

	r.obs.StepStart(r.unit.Name, step.String(), time.Now())
	container, err := r.execute(ctx, step)
	if stdout, stderr := captureOutput(ctx, container, err); stdout != "" || stderr != "" {
		r.obs.StepOutput(r.unit.Name, step.String(), time.Now(), stdout, stderr)
	}
	r.obs.StepDone(r.unit.Name, step.String(), time.Now(), err)
	if err != nil {
		r.err = err
		return false
	}

	r.container = container
	return r.next < len(r.steps)
}

func (r *Run) execute(ctx context.Context, step framework.Step) (*dagger.Container, error) {
	container, err := r.unit.Framework.Execute(ctx, r.client, r.unit, step, r.container)
	if err != nil {
		return nil, err
	}
	if container == nil {
		if r.next == len(r.steps) {
			return nil, errors.New("engine: final framework step produced no container")
		}
		return nil, nil
	}

	return syncContainer(ctx, container)
}
