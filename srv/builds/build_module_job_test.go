package builds

import (
	"context"
	"path/filepath"
	"testing"

	"fx.prodigy9.co/config"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/srvtest"
)

// Publication follows admitted policy, independently of release-name conventions.
func TestModulePublicationSelection(t *testing.T) {
	for _, test := range []struct {
		name    string
		policy  conf.PublishPolicy
		ref     string
		tag     string
		invalid bool
	}{
		{"always branch", conf.PublishPolicyAlways, "refs/heads/main", "latest", false},
		{"always tag", conf.PublishPolicyAlways, "refs/tags/v0.10.0-alpha.1", "latest", false},
		{"tags branch", conf.PublishPolicyTags, "refs/heads/main", "", false},
		{"tags alpha", conf.PublishPolicyTags, "refs/tags/v0.10.0-alpha.1", "v0.10.0-alpha.1", false},
		{"tags arbitrary", conf.PublishPolicyTags, "refs/tags/release-1", "release-1", false},
		{"tags empty", conf.PublishPolicyTags, "refs/tags/", "", false},
		{"never branch", conf.PublishPolicyNever, "refs/heads/main", "", false},
		{"never tag", conf.PublishPolicyNever, "refs/tags/v0.10.0-alpha.1", "", false},
		{"unknown", "sometimes", "refs/tags/v1.0.0", "", true},
		{"empty", "", "refs/tags/v1.0.0", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			tag, err := (moduleRequest{PublishPolicy: test.policy, Ref: test.ref}).publicationTag()
			if test.invalid {
				require.ErrorContains(t, err, "publish policy")
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.tag, tag)
		})
	}
}

func TestModuleRequestUsesWinningManifestPolicy(t *testing.T) {
	ctx := setupDB(t)
	first := queueTestBuild(t, ctx, "infra")
	create := &Create{Trigger: TriggerWebUI, UserID: first.UserID, Owner: first.Owner,
		Repo: first.Repo, Ref: "refs/heads/main", SHA: first.SHA, CloneURL: first.CloneURL}
	prepareCreate(t, ctx, create)
	create.Manifest.Server.Publish = conf.PublishPolicyNever
	second := &Build{}
	require.NoError(t, create.Execute(ctx, second))
	create.SHA = "later-commit"
	third := &Build{}
	require.NoError(t, create.Execute(ctx, third))

	for _, build := range []*Build{first, second, third} {
		request, err := readModuleRequest(ctx, modulesFor(t, ctx, build.ID)[0].ID)
		require.NoError(t, err)
		expected := conf.PublishPolicyAlways
		if build.ID == third.ID {
			expected = conf.PublishPolicyNever
		}
		require.Equal(t, expected, request.PublishPolicy)
		tag, err := request.publicationTag()
		require.NoError(t, err)
		if expected == conf.PublishPolicyAlways {
			require.Equal(t, "latest", tag)
		} else {
			require.Empty(t, tag)
		}
	}
}

func TestBuildOnlyTagSkipsRegistryPrerequisites(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	config.Set(cfg, engine.CheckoutCacheConfig, t.TempDir())
	ctx = config.NewContext(ctx, cfg)
	record, err := install.Load(ctx)
	require.NoError(t, err)
	ctx = install.NewContext(ctx, record)
	// Source-token access has its bound record, but publication would need a fresh
	// installation read and fail: build-only execution must not load that prerequisite.
	require.NoError(t, srvtest.SeedSettings(ctx, map[string]string{"install.installed_by_login": ""}))
	_, err = publicationContext(ctx, "ghcr.io/prod9/app")
	require.Error(t, err)
	create := &Create{Trigger: TriggerWebUI, UserID: 1, Owner: "prod9", Repo: "app",
		Ref: "refs/tags/v0.10.0-alpha.1", SHA: "abc123", Modules: ModuleSelection{"api"},
		CloneURL: filepath.Join(t.TempDir(), "missing.git")}
	prepareCreate(t, ctx, create)
	create.Manifest.Server.Publish = conf.PublishPolicyNever
	build := &Build{}
	require.NoError(t, create.Execute(ctx, build))
	module := modulesFor(t, ctx, build.ID)[0]
	require.NoError(t, (&BuildModuleJob{BuildModuleID: module.ID}).Run(ctx))
	require.Equal(t, []EventKind{EventRunStart, EventCloneStart, EventCloneDone, EventRunDone},
		kindsOf(eventsFor(t, ctx, build.ID)))
}

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
