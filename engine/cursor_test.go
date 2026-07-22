package engine

import (
	"context"
	"errors"
	"testing"

	"dagger.io/dagger"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/framework"
	"platform.prodigy9.co/framework/scaffold"
)

// stubFramework records the steps it is asked to execute, so a test can assert the engine
// drives the plan in order and stops where it should. It never touches dagger — the cursor's
// sequencing is what is under test, not the container work.
type stubFramework struct {
	steps  []framework.Step
	seen   []framework.Step
	failAt framework.Step
}

var errStubStep = errors.New("stub step failed")

func (*stubFramework) Name() string                 { return "stub" }
func (*stubFramework) Layout() framework.Layout     { return framework.LayoutBasic }
func (*stubFramework) Discover(string) bool         { return false }
func (*stubFramework) ScaffoldVars(string) []string { return nil }

func (f *stubFramework) Plan(*framework.BuildUnit) []framework.Step { return f.steps }

func (*stubFramework) Scaffold(context.Context, string, scaffold.Env, map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{}, nil
}

// newStubRun builds a cursor with its client already in hand, so no test ever dials a
// Dagger engine: the stub framework ignores the client entirely.
func newStubRun(fw *stubFramework) *Run {
	unit := &framework.BuildUnit{Framework: fw}
	return &Run{unit: unit, steps: fw.Plan(unit), client: &dagger.Client{}}
}

func (f *stubFramework) Execute(_ context.Context, _ *dagger.Client, _ *framework.BuildUnit, step framework.Step, in *dagger.Container) (*dagger.Container, error) {
	f.seen = append(f.seen, step)
	if step == f.failAt {
		return nil, errStubStep
	}
	return in, nil
}

func TestRunDrivesEveryStepInOrder(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one", "two", "three"}}
	run := newStubRun(fw)

	for run.Next(context.Background()) {
	}

	_, err := run.Result()
	require.NoError(t, err)
	require.Equal(t, fw.steps, fw.seen)
}

func TestRunStopsAtTheFailedStep(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one", "two", "three"}, failAt: "two"}
	run := newStubRun(fw)

	for run.Next(context.Background()) {
	}

	_, err := run.Result()
	require.ErrorIs(t, err, errStubStep)
	require.Equal(t, []framework.Step{"one", "two"}, fw.seen, "the third step must never run")
}

func TestRunWithNoStepsIsImmediatelyDone(t *testing.T) {
	fw := &stubFramework{}
	run := newStubRun(fw)

	require.False(t, run.Next(context.Background()))
	container, err := run.Result()
	require.NoError(t, err)
	require.Nil(t, container)
}
