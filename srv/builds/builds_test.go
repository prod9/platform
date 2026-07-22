package builds

import (
	"context"
	"testing"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/srvtest"
)

// setupDB migrates the auth schema too: the /api/builds endpoint tests seed sessions.
func setupDB(t *testing.T) context.Context {
	return srvtest.SetupDB(t,
		migrator.FromFS(Migrations),
		migrator.FromFS(auth.Migrations))
}

func queueTestBuild(t *testing.T, ctx context.Context, repo string) *Build {
	create := &Create{
		Owner:    "prod9",
		Repo:     repo,
		CloneURL: "https://github.com/prod9/" + repo + ".git",
		Tag:      "v1.2.3",
		SHA:      "abc123",
	}
	require.NoError(t, create.Execute(ctx, nil))

	build := &Build{}
	require.NoError(t, data.Get(ctx, build,
		`SELECT * FROM builds ORDER BY id DESC LIMIT 1`))
	return build
}

func TestCreateQueuesBuild(t *testing.T) {
	ctx := setupDB(t)

	build := queueTestBuild(t, ctx, "app")

	require.Equal(t, "prod9", build.Owner)
	require.Equal(t, "app", build.Repo)
	require.Equal(t, "https://github.com/prod9/app.git", build.CloneURL)
	require.Equal(t, "v1.2.3", build.Tag)
	require.Equal(t, "abc123", build.SHA)
	require.Equal(t, "queued", build.Status)
}
