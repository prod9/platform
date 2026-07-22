package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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
func newStubRun(fw *stubFramework, obs Observer) *Run {
	unit := &framework.BuildUnit{Framework: fw, Name: "stubunit"}
	return &Run{unit: unit, obs: obs, steps: fw.Plan(unit), client: &dagger.Client{}}
}

// recorder is an Observer that keeps every callback as a readable line, so a test asserts
// on the sequence a caller would see rather than on three separate counters.
type recorder struct {
	lines []string
	errs  []error
}

func (r *recorder) StepStarted(unit, step string, _ time.Time) {
	r.lines = append(r.lines, "started "+unit+"/"+step)
}

func (r *recorder) StepDone(unit, step string, _ time.Time, err error) {
	r.lines = append(r.lines, "done "+unit+"/"+step)
	r.errs = append(r.errs, err)
}

func (r *recorder) RunDone(unit string, _ time.Time, err error) {
	r.lines = append(r.lines, "rundone "+unit)
	r.errs = append(r.errs, err)
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
	run := newStubRun(fw, nil)

	for run.Next(context.Background()) {
	}

	_, err := run.Result()
	require.NoError(t, err)
	require.Equal(t, fw.steps, fw.seen)
}

func TestRunStopsAtTheFailedStep(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one", "two", "three"}, failAt: "two"}
	run := newStubRun(fw, nil)

	for run.Next(context.Background()) {
	}

	_, err := run.Result()
	require.ErrorIs(t, err, errStubStep)
	require.Equal(t, []framework.Step{"one", "two"}, fw.seen, "the third step must never run")
}

func TestRunReportsEveryStepToTheObserver(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one", "two"}}
	obs := &recorder{}
	run := newStubRun(fw, obs)

	for run.Next(context.Background()) {
	}

	require.Equal(t, []string{
		"started stubunit/one", "done stubunit/one",
		"started stubunit/two", "done stubunit/two",
		"rundone stubunit",
	}, obs.lines)
	require.Equal(t, []error{nil, nil, nil}, obs.errs)
}

func TestRunReportsTheFailureOnTheStepThatFailed(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one", "two", "three"}, failAt: "two"}
	obs := &recorder{}
	run := newStubRun(fw, obs)

	for run.Next(context.Background()) {
	}

	require.Equal(t, []string{
		"started stubunit/one", "done stubunit/one",
		"started stubunit/two", "done stubunit/two",
		"rundone stubunit",
	}, obs.lines, "the run ends where the step failed, and says so once")
	require.Equal(t, []error{nil, errStubStep, errStubStep}, obs.errs)
}

func TestRunDoneIsReportedOnlyOnce(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one"}}
	obs := &recorder{}
	run := newStubRun(fw, obs)

	for i := 0; i < 5; i++ {
		run.Next(context.Background())
	}

	require.Equal(t, 1, strings.Count(strings.Join(obs.lines, "\n"), "rundone"))
}

func TestRunWithNoStepsIsImmediatelyDone(t *testing.T) {
	fw := &stubFramework{}
	run := newStubRun(fw, nil)

	require.False(t, run.Next(context.Background()))
	container, err := run.Result()
	require.NoError(t, err)
	require.Nil(t, container)
}
