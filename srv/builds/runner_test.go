package builds

import (
	"context"
	"errors"
	"testing"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
)

func stubPublishBuild(t *testing.T, image, digest string, err error) {
	orig := publishBuild
	publishBuild = func(ctx context.Context, cfg *config.Source, build *Build) (string, string, error) {
		return image, digest, err
	}
	t.Cleanup(func() { publishBuild = orig })
}

func startRunner(t *testing.T, ctx context.Context) (cancel func(), done chan struct{}) {
	runnerCtx, stop := context.WithCancel(ctx)
	done = make(chan struct{})
	go func() {
		defer close(done)
		RunQueued(runnerCtx, fxtest.Configure())
	}()
	t.Cleanup(func() { stop(); requireRunnerExit(t, done) })
	return stop, done
}

func requireRunnerExit(t *testing.T, done chan struct{}) {
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunQueued did not exit after cancel")
	}
}

func waitForStatus(t *testing.T, ctx context.Context, id int64, status string) *Build {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if build := loadBuild(t, ctx, id); build.Status == status {
			return build
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("build %d never reached status %q", id, status)
	return nil
}

func TestRunQueuedRecordsSuccess(t *testing.T) {
	ctx := setupDB(t)
	queued := queueTestBuild(t, ctx, "app")
	stubPublishBuild(t, "ghcr.io/prod9/app:v1.2.3", "sha256:feed", nil)

	startRunner(t, ctx)

	build := waitForStatus(t, ctx, queued.ID, "succeeded")
	require.Equal(t, "ghcr.io/prod9/app:v1.2.3", build.Image)
	require.Equal(t, "sha256:feed", build.Digest)
	require.Equal(t, "", build.Error)
}

func TestRunQueuedRecordsFailure(t *testing.T) {
	ctx := setupDB(t)
	queued := queueTestBuild(t, ctx, "app")
	stubPublishBuild(t, "", "", errors.New("engine: build exploded"))

	startRunner(t, ctx)

	build := waitForStatus(t, ctx, queued.ID, "failed")
	require.Equal(t, "engine: build exploded", build.Error)
	require.Equal(t, "", build.Image)
	require.Equal(t, "", build.Digest)
}

func TestRecordOutcomeSurvivesShutdown(t *testing.T) {
	ctx := setupDB(t)
	queueTestBuild(t, ctx, "app")
	claimed, err := claimBuild(ctx)
	require.NoError(t, err)

	shutdown, cancel := context.WithCancel(ctx)
	cancel()
	recordOutcome(shutdown, &Finish{ID: claimed.ID, Image: "i", Digest: "d"})

	require.Equal(t, "succeeded", loadBuild(t, ctx, claimed.ID).Status)
}

func TestRunQueuedExitsOnCancel(t *testing.T) {
	ctx := setupDB(t)

	cancel, done := startRunner(t, ctx)
	cancel()
	requireRunnerExit(t, done)
}

