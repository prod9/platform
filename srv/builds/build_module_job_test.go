package builds

import (
	"context"
	"path/filepath"
	"testing"

	"fx.prodigy9.co/config"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/srv/github"
)

func TestPublicationContextIsolatesRegistryCredentials(t *testing.T) {
	ctx, _ := setupInstalled(t)
	require.NoError(t, github.SaveRegistryToken(ctx, "ghcr.io", "saved-token"))
	ambient := config.NewSource(&config.MemProvider{}, config.FromContext(ctx).Vars())
	config.Set(ambient, engine.RegistryConfig, "ambient.example")
	config.Set(ambient, engine.RegistryPasswordConfig, "ambient-token")
	ctx = config.NewContext(ctx, ambient)
	child, err := publicationContext(ctx, "ghcr.io/prod9/app")
	require.NoError(t, err)
	require.Equal(t, "saved-token", config.Get(config.FromContext(child), engine.RegistryPasswordConfig))
	require.Equal(t, "ambient-token", config.Get(ambient, engine.RegistryPasswordConfig))
	require.Equal(t, "ambient.example", config.Get(ambient, engine.RegistryConfig))
	missing, err := publicationContext(ctx, "registry.example/prod9/app")
	require.NoError(t, err)
	require.Empty(t, config.Get(config.FromContext(missing), engine.RegistryPasswordConfig))
}

func TestJobPrerequisiteFailureRetainsClaimWithoutFabricatedEvents(t *testing.T) {
	ctx := setupDB(t)
	build := queueTestBuild(t, ctx, "app")
	module := modulesFor(t, ctx, build.ID)[0]
	require.Error(t, (&BuildModuleJob{BuildModuleID: module.ID}).Run(ctx))
	require.False(t, modulesFor(t, ctx, build.ID)[0].ClaimedAt.IsZero())
	require.Empty(t, eventsFor(t, ctx, build.ID))
	require.NoError(t, (&BuildModuleJob{BuildModuleID: module.ID}).Run(context.WithoutCancel(ctx)))
}

func TestJobRecordsCloneFailureAndDuplicateDoesNoWork(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	config.Set(cfg, engine.CheckoutCacheConfig, t.TempDir())
	ctx = config.NewContext(ctx, cfg)
	create := &Create{
		Trigger: TriggerWebUI, UserID: 1, Owner: "prod9", Repo: "app",
		Ref: "refs/heads/main", SHA: "abc123", Modules: ModuleSelection{"api"},
		CloneURL: filepath.Join(t.TempDir(), "missing.git"),
	}
	prepareCreate(t, ctx, create)
	build := &Build{}
	require.NoError(t, create.Execute(ctx, build))
	module := modulesFor(t, ctx, build.ID)[0]
	job := &BuildModuleJob{BuildModuleID: module.ID}

	require.NoError(t, job.Run(ctx), "a recorded build failure is a successful job")
	events := eventsFor(t, ctx, build.ID)
	require.Equal(t, []EventKind{EventRunStart, EventCloneStart, EventCloneDone, EventRunDone}, kindsOf(events))
	require.NotEmpty(t, events[2].Error)
	require.Equal(t, events[2].Error, events[3].Error)
	require.Empty(t, modulesFor(t, ctx, build.ID)[0].EngineHost)

	require.NoError(t, job.Run(ctx))
	require.Equal(t, events, eventsFor(t, ctx, build.ID), "a duplicate appends no execution events")
}
