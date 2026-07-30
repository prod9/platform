package builds

import (
	"context"
	"testing"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/migrate"
	"platform.prodigy9.co/srv/srvtest"
)

// setupDB migrates the auth schema too: builds reference the user who asked for them, and
// the /api/builds endpoint tests seed sessions. The jobs table comes along because the scan
// schedules into it.
func setupDB(t *testing.T) context.Context {
	return srvtest.SetupDB(t,
		migrate.JobsTable,
		migrator.FromFS(Migrations),
		migrator.FromFS(auth.Migrations))
}

func queueTestBuild(t *testing.T, ctx context.Context, repo string) *Build {
	userID, err := auth.SystemUserID(ctx)
	require.NoError(t, err)

	create := &Create{
		Trigger:  TriggerGitHubPush,
		UserID:   userID,
		Owner:    "prod9",
		Repo:     repo,
		CloneURL: "https://github.com/prod9/" + repo + ".git",
		Ref:      "refs/tags/v1.2.3",
		SHA:      "abc123",
	}
	require.NoError(t, create.Execute(ctx, nil))

	build := &Build{}
	require.NoError(t, data.Get(ctx, build,
		`SELECT `+buildColumns+` FROM builds ORDER BY id DESC LIMIT 1`))
	return build
}

func TestCreateRecordsFullIntent(t *testing.T) {
	ctx := setupDB(t)
	systemUserID, err := auth.SystemUserID(ctx)
	require.NoError(t, err)

	build := queueTestBuild(t, ctx, "app")

	require.Equal(t, TriggerGitHubPush, build.Trigger)
	require.Equal(t, systemUserID, build.UserID)
	require.Equal(t, "prod9", build.Owner)
	require.Equal(t, "app", build.Repo)
	require.Equal(t, "https://github.com/prod9/app.git", build.CloneURL)
	require.Equal(t, "refs/tags/v1.2.3", build.Ref)
	require.Equal(t, "abc123", build.SHA)
}

// A build with no parent reads as zero rather than as a pointer nobody wants to check.
func TestCreateWithoutRetryOfReadsAsZero(t *testing.T) {
	ctx := setupDB(t)

	build := queueTestBuild(t, ctx, "app")

	require.Zero(t, build.RetryOf)
}

func TestCreateLinksARetryToItsParent(t *testing.T) {
	ctx := setupDB(t)
	parent := queueTestBuild(t, ctx, "app")

	create := &Create{
		Trigger:  TriggerRetry,
		RetryOf:  parent.ID,
		UserID:   parent.UserID,
		Owner:    parent.Owner,
		Repo:     parent.Repo,
		CloneURL: parent.CloneURL,
		Ref:      parent.Ref,
		SHA:      parent.SHA,
	}
	require.NoError(t, create.Execute(ctx, nil))

	retry := &Build{}
	require.NoError(t, data.Get(ctx, retry,
		`SELECT `+buildColumns+` FROM builds ORDER BY id DESC LIMIT 1`))
	require.Equal(t, TriggerRetry, retry.Trigger)
	require.Equal(t, parent.ID, retry.RetryOf)
	require.Equal(t, parent.SHA, retry.SHA)
}

func TestAppendEventRecordsTheStream(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "app")

	record := &AppendEvent{
		BuildID: build.ID,
		Kind:    EventStepDone,
		Unit:    "api",
		Step:    "test",
		At:      at(1),
		Stdout:  "ok",
	}
	require.NoError(t, record.Execute(ctx, nil))

	events := eventsFor(t, ctx, build.ID)
	require.Len(t, events, 1)
	require.Equal(t, EventStepDone, events[0].Kind)
	require.Equal(t, "api", events[0].Unit)
	require.Equal(t, "test", events[0].Step)
	require.Equal(t, at(1), events[0].At.UTC())
	require.Equal(t, "ok", events[0].Stdout)
}
