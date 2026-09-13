package builds

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func at(minute int) time.Time {
	return time.Date(2026, 7, 29, 10, minute, 0, 0, time.UTC)
}

func TestFoldModuleExposesClaimBeforeFirstEvent(t *testing.T) {
	result := FoldModule(BuildModule{ID: 1, ClaimedAt: at(1), ClaimedBy: "worker"}, nil)
	require.Equal(t, StatusRunning, result.Status)
	require.Equal(t, at(1), result.StartedAt)
	require.True(t, result.FinishedAt.IsZero())
	require.Empty(t, result.LastEventKind)
}

func TestFoldModulePreservesFirstFailureUntilRunCloses(t *testing.T) {
	module := BuildModule{ID: 1, ClaimedAt: at(1)}
	events := []*BuildEvent{
		{ID: 1, Kind: EventCloneDone, At: at(3), Error: "clone failed"},
		{ID: 2, Kind: EventConfigDone, At: at(2), Error: "later error"},
	}
	result := FoldModule(module, events)
	require.Equal(t, StatusRunning, result.Status)
	require.Equal(t, "clone failed", result.Error)
	require.Equal(t, EventConfigDone, result.LastEventKind)
	require.Equal(t, at(2), result.LastEventAt)
	require.True(t, result.FinishedAt.IsZero())

	events = append(events, &BuildEvent{ID: 3, Kind: EventRunDone, At: at(4), Image: "image:built"})
	result = FoldModule(module, events)
	require.Equal(t, StatusFailed, result.Status)
	require.Equal(t, at(4), result.FinishedAt)
	require.Equal(t, "image:built", result.Image)
	require.Empty(t, result.Hash)
}

func TestFoldModuleTakesImageOnlyFromRunDone(t *testing.T) {
	module := BuildModule{ID: 1, ClaimedAt: at(1)}
	events := []*BuildEvent{{ID: 1, Kind: EventPublishDone, Image: "ignored", Hash: "ignored"}}
	result := FoldModule(module, events)
	require.Empty(t, result.Image)
	require.Empty(t, result.Hash)

	events = append(events, &BuildEvent{ID: 2, Kind: EventRunDone, At: at(3), Image: "published", Hash: "sha256:abc"})
	result = FoldModule(module, events)
	require.Equal(t, StatusSucceeded, result.Status)
	require.Equal(t, "published", result.Image)
	require.Equal(t, "sha256:abc", result.Hash)
}
