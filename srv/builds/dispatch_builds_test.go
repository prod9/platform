package builds

import (
	"context"
	"encoding/json"
	"testing"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/worker"
	"github.com/stretchr/testify/require"
)

func pendingJobs(t *testing.T, ctx context.Context, name string) []*worker.Job {
	jobs := []*worker.Job{}
	require.NoError(t, data.Select(ctx, &jobs, `SELECT * FROM jobs WHERE name = $1 AND status = 'pending' ORDER BY id`, name))
	return jobs
}

func scheduledModuleIDs(t *testing.T, ctx context.Context) []int64 {
	ids := []int64{}
	for _, job := range pendingJobs(t, ctx, "build-module") {
		run := BuildModuleJob{}
		require.NoError(t, json.Unmarshal([]byte(job.Payload), &run))
		ids = append(ids, run.BuildModuleID)
	}
	return ids
}

func TestDispatcherSchedulesOnlyUnclaimedModules(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "api")
	modules := modulesFor(t, ctx, build.ID)
	claimTestModule(t, ctx, modules[0])
	require.NoError(t, (&DispatchBuilds{}).Run(ctx))
	require.Equal(t, []int64{modules[1].ID}, scheduledModuleIDs(t, ctx))
	require.NoError(t, (&DispatchBuilds{}).Run(ctx))
	require.Equal(t, []int64{modules[1].ID, modules[1].ID}, scheduledModuleIDs(t, ctx))
	require.Len(t, pendingJobs(t, ctx, "dispatch-builds"), 1)
}
