package builds

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTranscriberWritesEveryCallbackToTheStream(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "api")
	scribe := newTranscriber(ctx, build.ID)

	scribe.StepStarted("api", "build", at(1))
	scribe.StepOutput("api", "build", at(2), "compiled", "warning")
	scribe.StepDone("api", "build", at(3), nil)
	scribe.ImageBuilt("api", "ghcr.io/prod9/api", at(4))
	scribe.Published("api", "ghcr.io/prod9/api:v1.2.3", "sha256:abc", at(5))
	scribe.RunDone("api", at(6), nil)

	require.NoError(t, scribe.Err())
	events := eventsFor(t, ctx, build.ID)
	require.Equal(t, []EventKind{
		EventStepStarted, EventStepDone, EventImageBuilt, EventPublished, EventRunDone,
	}, kindsOf(events), "captured output is not an event of its own")
	require.Equal(t, "compiled", events[1].Stdout, "the step's output rides its terminal row")
	require.Equal(t, "warning", events[1].Stderr)
	require.Equal(t, "ghcr.io/prod9/api:v1.2.3", events[3].Image)
	require.Equal(t, "sha256:abc", events[3].Hash)
	require.Equal(t, at(6), events[4].At.UTC(), "the engine's own callback time is preserved")
}

// TestTranscriberKeepsEachStepsOutputToItsOwnRow is what the held capture has to get right:
// a build runs its units concurrently, so two steps are in flight at once and neither may
// end up wearing the other's output.
func TestTranscriberKeepsEachStepsOutputToItsOwnRow(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "api")
	scribe := newTranscriber(ctx, build.ID)

	scribe.StepOutput("api", "build", at(1), "api compiled", "")
	scribe.StepOutput("web", "build", at(2), "web compiled", "")
	scribe.StepDone("web", "build", at(3), nil)
	scribe.StepDone("api", "build", at(4), nil)

	events := eventsFor(t, ctx, build.ID)
	require.Equal(t, "web", events[0].Unit)
	require.Equal(t, "web compiled", events[0].Stdout)
	require.Equal(t, "api", events[1].Unit)
	require.Equal(t, "api compiled", events[1].Stdout)
}

// TestTranscriberRecordsWhatAStepFailedWith keeps the failure in the stream rather than in
// the job's return value: a build that failed and was recorded is a job that worked.
func TestTranscriberRecordsWhatAStepFailedWith(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "api")
	scribe := newTranscriber(ctx, build.ID)

	scribe.StepDone("api", "build", at(1), errors.New("exit status 1"))
	scribe.RunDone("api", at(2), errors.New("exit status 1"))

	require.NoError(t, scribe.Err())
	require.Equal(t, StatusFailed, Latest(eventsFor(t, ctx, build.ID)).Status)
}

// TestTranscriberKeepsAWriteFailureForTheJob is the one error a build job must fail on: a
// callback cannot return, so an unwritable stream is held until Run can report it.
func TestTranscriberKeepsAWriteFailureForTheJob(t *testing.T) {
	ctx := setupDB(t)
	scribe := newTranscriber(ctx, 404)

	scribe.RunDone("api", at(1), nil)

	require.Error(t, scribe.Err(), "no build 404 exists for the event to belong to")
}

// TestTranscriberIsSilentUntilTheEngineSpeaks pins what a build job reads to decide whether
// the stream needs a terminal event of its own.
func TestTranscriberIsSilentUntilTheEngineSpeaks(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "api")
	scribe := newTranscriber(ctx, build.ID)

	require.True(t, scribe.Silent())
	scribe.StepStarted("api", "build", at(1))
	require.False(t, scribe.Silent())
}

func kindsOf(events []*BuildEvent) []EventKind {
	kinds := []EventKind{}
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}
