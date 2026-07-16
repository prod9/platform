package srv

import (
	"context"
	"sync"
	"testing"

	"fx.prodigy9.co/data"
	"github.com/stretchr/testify/require"
)

func queueTestBuild(t *testing.T, ctx context.Context, repo string) *Build {
	create := &CreateBuild{
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

func loadBuild(t *testing.T, ctx context.Context, id int64) *Build {
	build := &Build{}
	require.NoError(t, data.Get(ctx, build, `SELECT * FROM builds WHERE id = $1`, id))
	return build
}

func claimBuild(ctx context.Context) (*Build, error) {
	build := &Build{}
	if err := (&ClaimBuild{}).Execute(ctx, build); err != nil {
		return nil, err
	}
	return build, nil
}

func TestClaimBuildClaimsOldestQueued(t *testing.T) {
	ctx := setupDB(t)
	first := queueTestBuild(t, ctx, "app")
	queueTestBuild(t, ctx, "later-app")

	build, err := claimBuild(ctx)
	require.NoError(t, err)
	require.Equal(t, first.ID, build.ID)
	require.Equal(t, "prod9", build.Owner)
	require.Equal(t, "app", build.Repo)
	require.Equal(t, "https://github.com/prod9/app.git", build.CloneURL)
	require.Equal(t, "v1.2.3", build.Tag)
	require.Equal(t, "abc123", build.SHA)
	require.Equal(t, "running", build.Status)
	require.True(t, build.UpdatedAt.After(first.UpdatedAt))

	require.Equal(t, "running", loadBuild(t, ctx, first.ID).Status)
}

func TestClaimBuildEmptyQueue(t *testing.T) {
	ctx := setupDB(t)

	build, err := claimBuild(ctx)
	require.ErrorIs(t, err, ErrNoQueuedBuild)
	require.Nil(t, build)
}

func TestClaimBuildConcurrentClaimsOneWinner(t *testing.T) {
	ctx := setupDB(t)
	queueTestBuild(t, ctx, "app")

	builds := make([]*Build, 2)
	errs := make([]error, 2)
	wg := sync.WaitGroup{}
	for i := range builds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			builds[i], errs[i] = claimBuild(ctx)
		}()
	}
	wg.Wait()

	claimed, missed := 0, 0
	for i := range builds {
		switch {
		case errs[i] == nil && builds[i] != nil:
			claimed++
		case errs[i] == ErrNoQueuedBuild:
			missed++
		default:
			t.Fatalf("claim %d: unexpected result: %v, %v", i, builds[i], errs[i])
		}
	}
	require.Equal(t, 1, claimed)
	require.Equal(t, 1, missed)
}

func TestRequeueOrphanBuildsRequeuesRunning(t *testing.T) {
	ctx := setupDB(t)
	queueTestBuild(t, ctx, "app")
	orphan, err := claimBuild(ctx)
	require.NoError(t, err)

	queueTestBuild(t, ctx, "done-app")
	finished, err := claimBuild(ctx)
	require.NoError(t, err)
	require.NoError(t, (&FinishBuild{ID: finished.ID, Image: "i", Digest: "d"}).Execute(ctx, nil))
	queued := queueTestBuild(t, ctx, "queued-app")

	require.NoError(t, (&RequeueOrphanBuilds{}).Execute(ctx, nil))

	requeued := loadBuild(t, ctx, orphan.ID)
	require.Equal(t, "queued", requeued.Status)
	require.True(t, requeued.UpdatedAt.After(orphan.UpdatedAt))

	require.Equal(t, "queued", loadBuild(t, ctx, queued.ID).Status)
	require.Equal(t, "succeeded", loadBuild(t, ctx, finished.ID).Status)
}

func TestFinishBuild(t *testing.T) {
	ctx := setupDB(t)
	queueTestBuild(t, ctx, "app")
	claimed, err := claimBuild(ctx)
	require.NoError(t, err)

	finish := &FinishBuild{
		ID:     claimed.ID,
		Image:  "ghcr.io/prod9/app:v1.2.3\nghcr.io/prod9/app-web:v1.2.3",
		Digest: "sha256:feed\nsha256:f00d",
	}
	require.NoError(t, finish.Execute(ctx, nil))

	build := loadBuild(t, ctx, claimed.ID)
	require.Equal(t, "succeeded", build.Status)
	require.Equal(t, finish.Image, build.Image)
	require.Equal(t, finish.Digest, build.Digest)
	require.Equal(t, "", build.Error)
	require.True(t, build.UpdatedAt.After(claimed.UpdatedAt))
}

func TestFailBuild(t *testing.T) {
	ctx := setupDB(t)
	queueTestBuild(t, ctx, "app")
	claimed, err := claimBuild(ctx)
	require.NoError(t, err)

	fail := &FailBuild{ID: claimed.ID, Error: "engine: build exploded"}
	require.NoError(t, fail.Execute(ctx, nil))

	build := loadBuild(t, ctx, claimed.ID)
	require.Equal(t, "failed", build.Status)
	require.Equal(t, "engine: build exploded", build.Error)
	require.Equal(t, "", build.Image)
	require.Equal(t, "", build.Digest)
	require.True(t, build.UpdatedAt.After(claimed.UpdatedAt))
}
