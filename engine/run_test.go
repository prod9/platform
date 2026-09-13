package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"dagger.io/dagger"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/engine/observer"
	"platform.prodigy9.co/framework"
	"platform.prodigy9.co/framework/scaffold"
)

var errStubStep = errors.New("stub step failed")

// stubFramework records the steps it is asked to execute, so a test can assert the engine
// drives the plan in order and stops where it should. It never touches dagger — the cursor's
// sequencing is what is under test, not the container work.
type stubFramework struct {
	steps   []framework.Step
	seen    []framework.Step
	failAt  framework.Step
	emptyAt framework.Step
}

func (*stubFramework) Name() string                 { return "stub" }
func (*stubFramework) Layout() framework.Layout     { return framework.LayoutBasic }
func (*stubFramework) Discover(string) bool         { return false }
func (*stubFramework) ScaffoldVars(string) []string { return nil }

func (f *stubFramework) Plan(*framework.BuildUnit) []framework.Step { return f.steps }

func (*stubFramework) Scaffold(context.Context, string, scaffold.Env, map[string]string) (scaffold.Spec, error) {
	return scaffold.Spec{}, nil
}

func (f *stubFramework) Execute(_ context.Context, _ *dagger.Client, _ *framework.BuildUnit, step framework.Step, in *dagger.Container) (*dagger.Container, error) {
	f.seen = append(f.seen, step)
	if step == f.failAt {
		return nil, errStubStep
	}
	if step == f.emptyAt {
		return nil, nil
	}
	return &dagger.Container{}, nil
}

// recorder is an Observer that keeps every callback as a readable line, so a test asserts
// on the sequence a caller would see rather than on five separate counters.
type recorder struct {
	lines []string
	errs  []error
}

func (r *recorder) StepStart(unit, step string, _ time.Time) {
	r.lines = append(r.lines, "started "+unit+"/"+step)
}

func (r *recorder) StepOutput(unit, step string, _ time.Time, stdout, stderr string) {
	r.lines = append(r.lines, "output "+unit+"/"+step+"/"+stdout+"/"+stderr)
}

func (r *recorder) StepDone(unit, step string, _ time.Time, err error) {
	r.lines = append(r.lines, "done "+unit+"/"+step)
	r.errs = append(r.errs, err)
}

func (r *recorder) RunStart(unit string, _ time.Time) { r.lines = append(r.lines, "runstart "+unit) }

func (r *recorder) CloneStart(unit string, _ time.Time) {
	r.lines = append(r.lines, "clonestart "+unit)
}

func (r *recorder) CloneDone(unit string, _ time.Time, err error) {
	r.lines = append(r.lines, "clonedone "+unit)
	r.errs = append(r.errs, err)
}

func (r *recorder) ConfigStart(unit string, _ time.Time) {
	r.lines = append(r.lines, "configstart "+unit)
}

func (r *recorder) ConfigDone(unit, host string, _ time.Time, err error) {
	r.lines = append(r.lines, "configdone "+unit+"/"+host)
	r.errs = append(r.errs, err)
}

func (r *recorder) PublishStart(unit string, _ time.Time) {
	r.lines = append(r.lines, "publishstart "+unit)
}

func (r *recorder) PublishDone(unit string, _ time.Time, err error) {
	r.lines = append(r.lines, "publishdone "+unit)
	r.errs = append(r.errs, err)
}

func (r *recorder) RunDone(unit, image, hash string, _ time.Time, err error) {
	r.lines = append(r.lines, "rundone "+unit+"/"+image+"/"+hash)
	r.errs = append(r.errs, err)
}

func TestRunDrivesEveryStepInOrder(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one", "two", "three"}}
	run := newStubRun(t, fw, nil)

	for run.Next(context.Background()) {
	}

	require.NoError(t, run.err)
	require.Equal(t, fw.steps, fw.seen)
}

func TestRunStopsAtTheFailedStep(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one", "two", "three"}, failAt: "two"}
	run := newStubRun(t, fw, nil)

	for run.Next(context.Background()) {
	}

	result := run
	require.ErrorIs(t, result.err, errStubStep)
	require.Equal(t, []framework.Step{"one", "two"}, fw.seen, "the third step must never run")
}

func TestRunReportsEveryStepToTheObserver(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one", "two"}}
	obs := &recorder{}
	run := newStubRun(t, fw, obs)

	for run.Next(context.Background()) {
	}

	require.Equal(t, []string{
		"started stubunit/one", "done stubunit/one",
		"started stubunit/two", "done stubunit/two",
	}, obs.lines, "the cursor reports only framework steps")
	require.Equal(t, []error{nil, nil}, obs.errs)
}

func TestRunReportsTheFailureOnTheStepThatFailed(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one", "two", "three"}, failAt: "two"}
	obs := &recorder{}
	run := newStubRun(t, fw, obs)

	for run.Next(context.Background()) {
	}

	require.Equal(t, []string{
		"started stubunit/one", "done stubunit/one",
		"started stubunit/two", "done stubunit/two",
	}, obs.lines, "the cursor stops at the failed step")
	require.Equal(t, []error{nil, errStubStep}, obs.errs)
}

func TestStepCursorDoesNotCompleteOperation(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one"}}
	obs := &recorder{}
	run := newStubRun(t, fw, obs)

	for i := 0; i < 5; i++ {
		run.Next(context.Background())
	}

	require.Equal(t, 0, strings.Count(strings.Join(obs.lines, "\n"), "rundone"), "step cursor must not close the enclosing operation")
}

// TestRunReportsCapturedOutputBeforeTheStepEnds pins the ordering a consumer that only
// stores the terminal row depends on: the output is in hand by the time StepDone arrives.
func TestRunReportsCapturedOutputBeforeTheStepEnds(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one"}}
	obs := &recorder{}
	run := newStubRun(t, fw, obs)

	restore := captureOutput
	captureOutput = func(context.Context, *dagger.Container, error) (string, string) {
		return "compiled", "warning"
	}
	defer func() { captureOutput = restore }()

	for run.Next(context.Background()) {
	}

	require.Equal(t, []string{
		"started stubunit/one",
		"output stubunit/one/compiled/warning",
		"done stubunit/one",
	}, obs.lines)
}

// TestCaptureOutputTakesAFailedStepsOutputFromItsError pins where the output of the step
// that broke the build comes from: a failed step yields no container, and dagger carries
// its streams on the error instead.
func TestCaptureOutputTakesAFailedStepsOutputFromItsError(t *testing.T) {
	execErr := &dagger.ExecError{Stdout: "compiling", Stderr: "syntax error"}

	stdout, stderr := captureOutput(context.Background(), nil, fmt.Errorf("step: %w", execErr))

	require.Equal(t, "compiling", stdout)
	require.Equal(t, "syntax error", stderr)
}

func TestCaptureOutputReportsNothingWhenThereIsNeither(t *testing.T) {
	stdout, stderr := captureOutput(context.Background(), nil, errStubStep)

	require.Empty(t, stdout)
	require.Empty(t, stderr)
}

// TestRunReportsNoOutputForAStepThatCapturedNothing keeps an empty capture off the stream:
// a step whose last operation was not an exec has no output, and a row of empty strings is
// not a thing that happened.
func TestRunReportsNoOutputForAStepThatCapturedNothing(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one"}}
	obs := &recorder{}
	run := newStubRun(t, fw, obs)

	for run.Next(context.Background()) {
	}

	require.NotContains(t, obs.lines, "output stubunit/one//")
}

func TestRunRejectsFinalStepWithoutContainer(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"one", "two"}, emptyAt: "two"}
	run := newStubRun(t, fw, nil)
	for run.Next(t.Context()) {
	}
	require.ErrorContains(t, run.err, "produced no container")
	require.Equal(t, []framework.Step{"one", "two"}, fw.seen)
}

// Host-only preparation (framework.PlatformInfra's deps step) has no container yet.
func TestRunAllowsHostOnlyIntermediateStep(t *testing.T) {
	fw := &stubFramework{steps: []framework.Step{"deps", "build"}, emptyAt: "deps"}
	run := newStubRun(t, fw, nil)
	for run.Next(t.Context()) {
	}
	require.NoError(t, run.err)
	require.Equal(t, fw.steps, fw.seen)
	require.NotNil(t, run.container)
}

// newStubRun builds a cursor with its client already in hand, so no test ever dials a
// Dagger engine: the stub framework ignores the client entirely.
func newStubRun(t *testing.T, fw *stubFramework, obs observer.Observer) *Run {
	t.Helper()
	stubContainerEvaluation(t)

	unit := &framework.BuildUnit{Framework: fw, Name: "stubunit", ImageName: "stubimage"}
	composed, _ := observer.Accumulate(obs)
	run := &Run{unit: unit, obs: composed, steps: fw.Plan(unit), client: &dagger.Client{}}
	return run
}

func stubContainerEvaluation(t *testing.T) {
	t.Helper()

	previousCapture := captureOutput
	captureOutput = func(context.Context, *dagger.Container, error) (string, string) { return "", "" }
	t.Cleanup(func() { captureOutput = previousCapture })

	previous := syncContainer
	syncContainer = func(_ context.Context, container *dagger.Container) (*dagger.Container, error) { return container, nil }
	t.Cleanup(func() { syncContainer = previous })
}
