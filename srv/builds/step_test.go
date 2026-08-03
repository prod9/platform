package builds

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func stepEvent(kind EventKind, unit, step string, minute int) *BuildEvent {
	return &BuildEvent{Kind: kind, Unit: unit, Step: step, At: at(minute)}
}

func TestStepsWithoutEventsIsEmpty(t *testing.T) {
	require.Empty(t, Steps(nil))
}

func TestStepsPairStartAndDoneWithCapturedOutput(t *testing.T) {
	done := stepEvent(EventStepDone, "api", "test", 2)
	done.Stdout, done.Stderr = "ok\n", "warn\n"

	steps := Steps([]*BuildEvent{
		stepEvent(EventStepStarted, "api", "test", 1),
		done,
		event(EventRunDone, "api", 3),
	})

	require.Len(t, steps, 1)
	require.Equal(t, 0, steps[0].Attempt)
	require.Equal(t, "api", steps[0].Unit)
	require.Equal(t, "test", steps[0].Step)
	require.Equal(t, at(1), steps[0].StartedAt)
	require.Equal(t, at(2), steps[0].FinishedAt)
	require.Equal(t, "ok\n", steps[0].Stdout)
	require.Equal(t, "warn\n", steps[0].Stderr)
	require.Empty(t, steps[0].Error)
}

// A started step with no step_done yet is still listed — the detail view shows what is
// running, not only what finished.
func TestStepsKeepUnfinishedSteps(t *testing.T) {
	steps := Steps([]*BuildEvent{
		stepEvent(EventStepStarted, "api", "base", 1),
	})

	require.Len(t, steps, 1)
	require.Equal(t, at(1), steps[0].StartedAt)
	require.True(t, steps[0].FinishedAt.IsZero())
}

// Steps split on the same terminal boundary as fold, so a retry's steps carry the next
// attempt ordinal and the two folds can never disagree on where an attempt ends.
func TestStepsStampTheAttemptOrdinalAcrossRetries(t *testing.T) {
	broke := stepEvent(EventStepDone, "api", "test", 2)
	broke.Error = "exit status 1"
	failed := event(EventRunDone, "api", 3)
	failed.Error = "exit status 1"

	events := []*BuildEvent{
		stepEvent(EventStepStarted, "api", "test", 1),
		broke,
		failed,
		stepEvent(EventStepStarted, "api", "test", 4),
		stepEvent(EventStepDone, "api", "test", 5),
		event(EventRunDone, "api", 6),
	}

	steps := Steps(events)
	require.Len(t, steps, 2)
	require.Equal(t, 0, steps[0].Attempt)
	require.Equal(t, "exit status 1", steps[0].Error)
	require.Equal(t, 1, steps[1].Attempt)
	require.Empty(t, steps[1].Error)
	require.Len(t, fold(events), 2)
}

// Two units run concurrently, so their same-named steps are distinct entries told apart
// by unit.
func TestStepsAreKeyedByUnitAndStep(t *testing.T) {
	steps := Steps([]*BuildEvent{
		stepEvent(EventStepStarted, "api", "test", 1),
		stepEvent(EventStepStarted, "web", "test", 2),
		stepEvent(EventStepDone, "api", "test", 3),
		stepEvent(EventStepDone, "web", "test", 4),
	})

	require.Len(t, steps, 2)
	require.Equal(t, "api", steps[0].Unit)
	require.Equal(t, "web", steps[1].Unit)
	require.Equal(t, at(3), steps[0].FinishedAt)
	require.Equal(t, at(4), steps[1].FinishedAt)
}
