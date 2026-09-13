package builds

import (
	"errors"
	"testing"

	"fx.prodigy9.co/data"
	"github.com/stretchr/testify/require"
)

func TestTranscriberWritesPairedLifecycleAndFinalResult(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "api")
	module := claimTestModule(t, ctx, modulesFor(t, ctx, build.ID)[0])
	scribe := newTranscriber(ctx, module.ID)
	scribe.RunStart("api", at(1))
	scribe.CloneStart("api", at(2))
	scribe.CloneDone("api", at(3), nil)
	scribe.ConfigStart("api", at(4))
	scribe.ConfigDone("api", "runner:8080", at(5), nil)
	scribe.StepStart("api", "build", at(6))
	scribe.StepOutput("api", "build", at(7), "compiled", "warning")
	scribe.StepDone("api", "build", at(8), nil)
	scribe.PublishStart("api", at(9))
	scribe.PublishDone("api", at(10), nil)
	scribe.RunDone("api", "ghcr.io/prod9/api:release-1", "sha256:abc", at(11), nil)
	require.NoError(t, scribe.Err())
	require.True(t, scribe.Complete())
	events := eventsFor(t, ctx, build.ID)
	require.Equal(t, []EventKind{EventRunStart, EventCloneStart, EventCloneDone, EventConfigStart, EventConfigDone, EventStepStart, EventStepDone, EventPublishStart, EventPublishDone, EventRunDone}, kindsOf(events))
	require.Equal(t, "compiled", events[6].Stdout)
	require.Equal(t, "warning", events[6].Stderr)
	require.Empty(t, events[8].Image)
	require.Equal(t, "ghcr.io/prod9/api:release-1", events[9].Image)
	require.Equal(t, "sha256:abc", events[9].Hash)
	assigned := modulesFor(t, ctx, build.ID)[0]
	require.Equal(t, "runner:8080", assigned.EngineHost)
	require.Equal(t, at(5), assigned.EngineAssignedAt.UTC())
}

func TestConfigurationEventAndAssignmentAreAtomic(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "api")
	module := modulesFor(t, ctx, build.ID)[0]
	// Assignment without a claim violates the module invariant, so neither write commits.
	scribe := newTranscriber(ctx, module.ID)
	scribe.ConfigDone("api", "local", at(1), nil)
	require.Error(t, scribe.Err())
	require.Empty(t, eventsFor(t, ctx, build.ID))
	require.Empty(t, modulesFor(t, ctx, build.ID)[0].EngineHost)
	claimed := claimTestModule(t, ctx, module)
	failed := newTranscriber(ctx, claimed.ID)
	failed.ConfigDone("api", "", at(2), errors.New("bad config"))
	require.NoError(t, failed.Err())
	require.Empty(t, modulesFor(t, ctx, build.ID)[0].EngineHost)
}

func TestTranscriberStopsAfterWriteFailure(t *testing.T) {
	ctx := setupDB(t)
	scribe := newTranscriber(ctx, 404)
	scribe.RunStart("api", at(1))
	scribe.RunDone("api", "", "", at(2), nil)
	require.Error(t, scribe.Err())
	require.False(t, scribe.Complete())
	var count int
	require.NoError(t, data.Get(ctx, &count, `SELECT count(*) FROM build_events`))
	require.Zero(t, count)
}

func kindsOf(events []*BuildEvent) []EventKind {
	kinds := make([]EventKind, len(events))
	for i, event := range events {
		kinds[i] = event.Kind
	}
	return kinds
}
