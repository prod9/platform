package builds

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// at counts minutes off a fixed origin so an assertion names a moment rather than a
// duration arithmetic expression.
func at(minute int) time.Time {
	return time.Date(2026, 7, 29, 10, minute, 0, 0, time.UTC)
}

func event(kind EventKind, unit string, minute int) *BuildEvent {
	return &BuildEvent{Kind: kind, Unit: unit, At: at(minute)}
}

func TestLatestWithoutEventsIsQueued(t *testing.T) {
	latest := Latest(nil)

	require.Equal(t, StatusQueued, latest.Status)
	require.True(t, latest.StartedAt.IsZero())
}

func TestLatestIsRunningUntilEveryStartedUnitIsDone(t *testing.T) {
	latest := Latest([]*BuildEvent{
		event(EventStepStarted, "api", 1),
		event(EventStepStarted, "web", 2),
		event(EventRunDone, "api", 3),
	})

	require.Equal(t, StatusRunning, latest.Status)
	require.Equal(t, at(1), latest.StartedAt)
	require.True(t, latest.FinishedAt.IsZero())
}

func TestLatestSucceedsWhenEveryUnitFinishesClean(t *testing.T) {
	published := event(EventPublished, "api", 3)
	published.Image, published.Hash = "ghcr.io/prod9/app:v1.2.3", "sha256:abc"

	latest := Latest([]*BuildEvent{
		event(EventStepStarted, "api", 1),
		event(EventImageBuilt, "api", 2),
		published,
		event(EventRunDone, "api", 4),
	})

	require.Equal(t, StatusSucceeded, latest.Status)
	require.Equal(t, at(4), latest.FinishedAt)
	require.Equal(t, "ghcr.io/prod9/app:v1.2.3", latest.Image)
	require.Equal(t, "sha256:abc", latest.Hash)
	require.Empty(t, latest.Error)
}

// One failed unit fails the build even when its siblings finish clean, and the first
// error reported is the one kept: the step that broke is the cause.
func TestLatestFailsOnAnyUnitError(t *testing.T) {
	broke := event(EventStepDone, "web", 2)
	broke.Error = "exit status 1"
	terminal := event(EventRunDone, "web", 3)
	terminal.Error = "exit status 1"

	latest := Latest([]*BuildEvent{
		event(EventStepStarted, "api", 1),
		broke,
		terminal,
		event(EventRunDone, "api", 4),
	})

	require.Equal(t, StatusFailed, latest.Status)
	require.Equal(t, "exit status 1", latest.Error)
}

// A retry appends to the same stream, so a terminal boundary opens a new attempt and the
// list shows the newest one — the earlier failure stays readable behind it.
func TestFoldSplitsAttemptsOnTheTerminalBoundary(t *testing.T) {
	failed := event(EventRunDone, "api", 2)
	failed.Error = "boom"

	events := []*BuildEvent{
		event(EventStepStarted, "api", 1),
		failed,
		event(EventStepStarted, "api", 3),
		event(EventRunDone, "api", 4),
	}

	attempts := fold(events)
	require.Len(t, attempts, 2)
	require.Equal(t, StatusFailed, attempts[0].Status)
	require.Equal(t, StatusSucceeded, attempts[1].Status)
	require.Equal(t, attempts[1], Latest(events))
}
