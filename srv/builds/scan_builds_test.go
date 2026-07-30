package builds

import (
	"context"
	"encoding/json"
	"testing"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/worker"
	"github.com/stretchr/testify/require"
)

// pendingJobs reads the queue fx would run, oldest first, so a test asserts on what the
// scan put there rather than on the scan's own return value.
func pendingJobs(t *testing.T, ctx context.Context, name string) []*worker.Job {
	jobs := []*worker.Job{}
	require.NoError(t, data.Select(ctx, &jobs,
		`SELECT * FROM jobs WHERE name = $1 AND status = 'pending' ORDER BY id`, name))
	return jobs
}

// scheduledBuildIDs is what the scan asked for: the build each queued job carries.
func scheduledBuildIDs(t *testing.T, ctx context.Context) []int64 {
	ids := []int64{}
	for _, job := range pendingJobs(t, ctx, "build") {
		run := RunBuild{}
		require.NoError(t, json.Unmarshal([]byte(job.Payload), &run))
		ids = append(ids, run.BuildID)
	}
	return ids
}

func TestScanSchedulesOneJobPerUnreportedBuild(t *testing.T) {
	ctx := setupDB(t)
	first := queueTestBuild(t, ctx, "api")
	second := queueTestBuild(t, ctx, "web")

	require.NoError(t, (&ScanBuilds{}).Run(ctx))

	require.Equal(t, []int64{first.ID, second.ID}, scheduledBuildIDs(t, ctx),
		"every build nothing has reported on is work")
}

// TestScanLeavesABuildThatHasBeenPickedUp is what keeps one build from running twice: the
// build job reports before it does anything, so an event means a worker already has it.
func TestScanLeavesABuildThatHasBeenPickedUp(t *testing.T) {
	ctx := setupDB(t)
	taken := queueTestBuild(t, ctx, "api")
	fresh := queueTestBuild(t, ctx, "web")

	started := &AppendEvent{BuildID: taken.ID, Kind: EventStepStarted, Unit: "api", At: at(1)}
	require.NoError(t, started.Execute(ctx, nil))
	require.NoError(t, (&ScanBuilds{}).Run(ctx))

	require.Equal(t, []int64{fresh.ID}, scheduledBuildIDs(t, ctx))
}

func TestScanRearmsItself(t *testing.T) {
	ctx := setupDB(t)

	require.NoError(t, (&ScanBuilds{}).Run(ctx))

	require.Len(t, pendingJobs(t, ctx, "scan_builds"), 1,
		"nothing else queues the scan, so it must queue its own next run")
}

// TestScanRearmsOnceWhateverElseIsQueued pins the singleton: the scan's name is fx's dedupe
// key, so a second sweep must not leave two pumps behind.
func TestScanRearmsOnceWhateverElseIsQueued(t *testing.T) {
	ctx := setupDB(t)

	require.NoError(t, (&ScanBuilds{}).Run(ctx))
	require.NoError(t, (&ScanBuilds{}).Run(ctx))

	require.Len(t, pendingJobs(t, ctx, "scan_builds"), 1)
}
