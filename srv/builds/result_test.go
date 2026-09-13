package builds

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReduceModulesWaitsForQueuedSiblings(t *testing.T) {
	failed := FoldModule(BuildModule{ID: 1, ClaimedAt: at(1)}, []*BuildEvent{
		{ID: 2, Kind: EventRunDone, At: at(3), Error: "failed"},
	})
	queued := FoldModule(BuildModule{ID: 2}, nil)
	result := ReduceModules([]ModuleResult{failed, queued})
	require.Equal(t, StatusRunning, result.Status)
	require.Equal(t, "failed", result.Error)
	require.Equal(t, at(1), result.StartedAt)
	require.True(t, result.FinishedAt.IsZero())
}

func TestReduceModulesUsesEventOrderForErrorAndObservationTimesForDuration(t *testing.T) {
	first := FoldModule(BuildModule{ID: 1, ClaimedAt: at(2)}, []*BuildEvent{
		{ID: 4, Kind: EventRunDone, At: at(5), Error: "second recorded"},
	})
	second := FoldModule(BuildModule{ID: 2, ClaimedAt: at(1)}, []*BuildEvent{
		{ID: 3, Kind: EventRunDone, At: at(6), Error: "first recorded"},
	})
	result := ReduceModules([]ModuleResult{first, second})
	require.Equal(t, StatusFailed, result.Status)
	require.Equal(t, "first recorded", result.Error)
	require.Equal(t, at(1), result.StartedAt)
	require.Equal(t, at(6), result.FinishedAt)
}

func TestReduceModulesAllQueuedOrAllSuccessful(t *testing.T) {
	queued := FoldModule(BuildModule{ID: 1}, nil)
	require.Equal(t, StatusQueued, ReduceModules([]ModuleResult{queued}).Status)
	success := FoldModule(BuildModule{ID: 1, ClaimedAt: at(1)}, []*BuildEvent{
		{ID: 1, Kind: EventRunDone, At: at(2)},
	})
	require.Equal(t, StatusSucceeded, ReduceModules([]ModuleResult{success}).Status)
}
