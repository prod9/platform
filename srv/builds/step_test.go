package builds

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStepsPairByModuleAndPreserveStartOrder(t *testing.T) {
	steps := Steps([]*BuildEvent{
		{ID: 1, BuildModuleID: 8, Kind: EventStepStart, Step: "test", At: at(1)},
		{ID: 2, BuildModuleID: 3, Kind: EventStepStart, Step: "test", At: at(2)},
		{ID: 3, BuildModuleID: 3, Kind: EventStepDone, Step: "test", At: at(3), Stdout: "ok", Stderr: "warn"},
		{ID: 4, BuildModuleID: 8, Kind: EventStepDone, Step: "test", At: at(4), Error: "failed"},
		{ID: 5, BuildModuleID: 3, Kind: EventStepStart, Step: "build", At: at(5)},
	})
	require.Len(t, steps, 3)
	require.Equal(t, int64(8), steps[0].BuildModuleID)
	require.Equal(t, at(4), steps[0].FinishedAt)
	require.Equal(t, "failed", steps[0].Error)
	require.Equal(t, int64(3), steps[1].BuildModuleID)
	require.Equal(t, "ok", steps[1].Stdout)
	require.Equal(t, "warn", steps[1].Stderr)
	require.Equal(t, at(5), steps[2].StartedAt)
	require.True(t, steps[2].FinishedAt.IsZero())
}
