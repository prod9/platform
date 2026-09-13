package builds

import (
	"context"
	"testing"

	"fx.prodigy9.co/data"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/repos"
	"platform.prodigy9.co/srv/srvtest"
)

const testManifest = `repository = "github.com/prod9/app"
[modules.api]
framework = "go/basic"
[modules.web]
framework = "go/basic"
`

func setupDB(t *testing.T) context.Context { return srvtest.SetupDB(t) }

func registerTestRepo(t *testing.T, ctx context.Context, owner, repo string) {
	userID, err := auth.SystemUserID(ctx)
	require.NoError(t, err)
	require.NoError(t, data.Exec(ctx, `INSERT INTO repos (owner, repo, registered_by)
  VALUES ($1,$2,$3) ON CONFLICT (owner, repo) DO NOTHING`, owner, repo, userID))
}

func prepareCreate(t *testing.T, ctx context.Context, create *Create) {
	registerTestRepo(t, ctx, create.Owner, create.Repo)
	model, err := repos.ParseManifest([]byte(testManifest), create.Repo)
	require.NoError(t, err)
	create.ManifestRaw, create.Manifest = testManifest, *model
}

func queueTestBuild(t *testing.T, ctx context.Context, repo string) *Build {
	userID, err := auth.SystemUserID(ctx)
	require.NoError(t, err)
	create := &Create{Trigger: TriggerGitHubPush, UserID: userID, Owner: "prod9", Repo: repo,
		CloneURL: "https://github.com/prod9/" + repo + ".git", Ref: "refs/tags/v1.2.3", SHA: "abc123"}
	prepareCreate(t, ctx, create)
	build := &Build{}
	require.NoError(t, create.Execute(ctx, build))
	return build
}

func modulesFor(t *testing.T, ctx context.Context, buildID int64) []BuildModule {
	modules := []BuildModule{}
	require.NoError(t, data.Select(ctx, &modules, `SELECT `+moduleColumns+` FROM `+moduleFrom+
		` WHERE bm.build_id = $1 ORDER BY bm.id`, buildID))
	return modules
}

func claimTestModule(t *testing.T, ctx context.Context, module BuildModule) BuildModule {
	claimed := BuildModule{}
	require.NoError(t, (&ClaimModule{ID: module.ID, Hostname: "worker-test"}).Execute(ctx, &claimed))
	return claimed
}

func TestCreateRecordsFullIntent(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "app")
	modules := modulesFor(t, ctx, build.ID)
	require.Len(t, modules, 2)
	require.Equal(t, []string{"api", "web"}, []string{modules[0].Name, modules[1].Name})
	require.NotZero(t, build.RepoID)
	require.NotZero(t, build.ManifestID)
	for _, module := range modules {
		require.Equal(t, build.ManifestID, module.ManifestID)
		require.True(t, module.ClaimedAt.IsZero())
	}
}

func TestCreateReusesWinningSnapshotAndSelection(t *testing.T) {
	ctx := setupDB(t)
	first := queueTestBuild(t, ctx, "app")
	userID, err := auth.SystemUserID(ctx)
	require.NoError(t, err)
	create := &Create{Trigger: TriggerWebUI, UserID: userID, Owner: "prod9", Repo: "app", Ref: "refs/heads/main", SHA: "abc123", Modules: ModuleSelection{"web"}}
	prepareCreate(t, ctx, create)
	delete(create.Manifest.Modules, "web")
	create.ManifestRaw = "a later observation cannot replace the winner"
	second := &Build{}
	require.NoError(t, create.Execute(ctx, second))
	require.Equal(t, first.ManifestID, second.ManifestID)
	selected := modulesFor(t, ctx, second.ID)
	require.Len(t, selected, 1)
	require.Equal(t, "web", selected[0].Name)
	var raw string
	require.NoError(t, data.Get(ctx, &raw, `SELECT raw FROM repo_manifests WHERE id = $1`, second.ManifestID))
	require.Equal(t, testManifest, raw)
}

func TestInvalidSelectionRollsBackWholeIntent(t *testing.T) {
	ctx := setupDB(t)
	userID, err := auth.SystemUserID(ctx)
	require.NoError(t, err)
	create := &Create{Trigger: TriggerWebUI, UserID: userID, Owner: "prod9", Repo: "app", Ref: "refs/heads/main", SHA: "abc123", Modules: ModuleSelection{"missing"}}
	prepareCreate(t, ctx, create)
	require.ErrorIs(t, create.Execute(ctx, nil), ErrInvalidModules)
	var builds, snapshots int
	require.NoError(t, data.Get(ctx, &builds, `SELECT count(*) FROM builds`))
	require.NoError(t, data.Get(ctx, &snapshots, `SELECT count(*) FROM repo_manifests`))
	require.Zero(t, builds)
	require.Zero(t, snapshots)
}

func TestConcurrentCreatesShareCompleteSnapshot(t *testing.T) {
	ctx := setupDB(t)
	userID, err := auth.SystemUserID(ctx)
	require.NoError(t, err)
	create := Create{Trigger: TriggerWebUI, UserID: userID, Owner: "prod9", Repo: "app",
		Ref: "refs/heads/main", SHA: "same-commit"}
	prepareCreate(t, ctx, &create)
	type outcome struct {
		build Build
		err   error
	}
	results := make(chan outcome, 2)
	for range 2 {
		go func() {
			var result outcome
			result.err = create.Execute(ctx, &result.build)
			results <- result
		}()
	}

	first, second := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, first.build.ManifestID, second.build.ManifestID)
	for _, build := range []Build{first.build, second.build} {
		require.Len(t, modulesFor(t, ctx, build.ID), 2)
	}
	var observations int
	require.NoError(t, data.Get(ctx, &observations, `SELECT count(*) FROM repo_manifests`))
	require.Equal(t, 1, observations)
}

func TestModuleClaimOnlySucceedsOnce(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "app")
	module := modulesFor(t, ctx, build.ID)[0]
	claimed := claimTestModule(t, ctx, module)
	require.NotZero(t, claimed.ClaimedAt)
	require.Equal(t, "worker-test", claimed.ClaimedBy)
	err := (&ClaimModule{ID: module.ID, Hostname: "duplicate"}).Execute(ctx, &BuildModule{})
	require.True(t, data.IsNoRows(err))
	require.NoError(t, (&BuildModuleJob{BuildModuleID: module.ID}).Run(ctx))
	require.Empty(t, eventsFor(t, ctx, build.ID))
}
