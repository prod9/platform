package install

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/srvtest"
)

// Suffix invalidation (docs/spec/installation.md, §Redo and suffix invalidation):
// every save writes its own step all-or-none, then resets every later step in the
// same transaction. These tests seed a fully installed server, redo one step, and
// assert the suffix reads not-started again while the prefix survives.

func TestSaveOrgResetsSuffix(t *testing.T) {
	ctx := seedInstalled(t)

	require.NoError(t, (&SaveOrg{Org: "other-org"}).Execute(ctx, nil))

	org, err := github.LoadOrg(ctx)
	require.NoError(t, err)
	require.Equal(t, "other-org", org)

	_, err = github.LoadAppCreation(ctx)
	require.ErrorIs(t, err, github.ErrNoApp)
	_, err = github.LoadApp(ctx)
	require.ErrorIs(t, err, github.ErrNoApp)
	_, err = github.LoadRegistryToken(ctx, "ghcr.io")
	require.ErrorIs(t, err, github.ErrNoRegistryToken)
	_, err = Load(ctx)
	require.ErrorIs(t, err, ErrNotInstalled)
}

func TestSaveAppResetsSuffixKeepsPrefix(t *testing.T) {
	ctx := seedInstalled(t)

	action := &SaveApp{AppID: 43, Slug: "prodigy9-platform-new", ClientID: "Iv1.new", WebhookSecret: "whsec2"}
	require.NoError(t, action.Execute(ctx, nil))

	org, err := github.LoadOrg(ctx)
	require.NoError(t, err)
	require.Equal(t, "prod9", org)

	creation, err := github.LoadAppCreation(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(43), creation.AppID)
	require.Equal(t, "prodigy9-platform-new", creation.Slug)

	_, err = github.LoadApp(ctx)
	require.ErrorIs(t, err, github.ErrNoApp)
	_, err = github.LoadRegistryToken(ctx, "ghcr.io")
	require.ErrorIs(t, err, github.ErrNoRegistryToken)
	_, err = Load(ctx)
	require.ErrorIs(t, err, ErrNotInstalled)
}

func TestSaveCredentialsResetsSuffixKeepsPrefix(t *testing.T) {
	ctx := seedInstalled(t)

	action := &SaveCredentials{PrivateKey: "-----BEGIN RSA PRIVATE KEY-----", ClientSecret: "csec2"}
	require.NoError(t, action.Execute(ctx, nil))

	creation, err := github.LoadAppCreation(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(42), creation.AppID)

	app, err := github.LoadApp(ctx)
	require.NoError(t, err)
	require.Equal(t, "csec2", app.ClientSecret)

	_, err = github.LoadRegistryToken(ctx, "ghcr.io")
	require.ErrorIs(t, err, github.ErrNoRegistryToken)
	_, err = Load(ctx)
	require.ErrorIs(t, err, ErrNotInstalled)
}

func TestSaveRegistryTokenResetsSuffixKeepsPrefix(t *testing.T) {
	ctx := seedInstalled(t)

	require.NoError(t, (&SaveRegistryToken{Token: "ghp_new"}).Execute(ctx, nil))

	app, err := github.LoadApp(ctx)
	require.NoError(t, err)
	require.Equal(t, "csec", app.ClientSecret)

	token, err := github.LoadRegistryToken(ctx, "ghcr.io")
	require.NoError(t, err)
	require.Equal(t, "ghp_new", token)

	_, err = Load(ctx)
	require.ErrorIs(t, err, ErrNotInstalled)
}

// A reset install.* row reads as the not-yet-claimed state, so a fresh claim
// converges — the redo path back to completely installed.
func TestClaimAfterResetSucceeds(t *testing.T) {
	ctx := seedInstalled(t)

	require.NoError(t, (&SaveRegistryToken{Token: "ghp_new"}).Execute(ctx, nil))
	claimInstalled(t, ctx)

	record, err := Load(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(7), record.InstallationID)
}

// seedInstalled migrates a test database and walks every wizard write — the real
// writers, per the fixture-through-the-writer discipline — landing the server
// completely installed.
func seedInstalled(t *testing.T) context.Context {
	ctx := srvtest.SetupDB(t, Source)

	require.NoError(t, (&SaveOrg{Org: "prod9"}).Execute(ctx, nil))
	action := &SaveApp{AppID: 42, Slug: "prodigy9-platform", ClientID: "Iv1.abc", WebhookSecret: "whsec"}
	require.NoError(t, action.Execute(ctx, nil))
	keys := &SaveCredentials{PrivateKey: "-----BEGIN RSA PRIVATE KEY-----", ClientSecret: "csec"}
	require.NoError(t, keys.Execute(ctx, nil))
	require.NoError(t, (&SaveRegistryToken{Token: "ghp_token"}).Execute(ctx, nil))
	claimInstalled(t, ctx)

	return ctx
}
