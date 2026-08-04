package github

import (
	"context"
	"testing"

	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
)

// connectDB connects a fresh test database. This package cannot use srvtest (srvtest
// stubs LoadApp, so it imports this package back), so the postgres gate and setup are
// inlined.
func connectDB(t *testing.T) context.Context {
	if config.Get(fxtest.Configure(), data.DatabaseURLConfig) == "" {
		t.Skip("DATABASE_URL unset; skipping postgres-backed test")
	}
	return fxtest.ConnectTestDatabase(t)
}

// migrateSettings applies fx's settings schema — the storage the github.app_* keys
// live in.
func migrateSettings(t *testing.T, ctx context.Context) {
	m := migrator.New(data.FromContext(ctx),
		migrator.FromFS(*settings.App.EmbeddedMigrations()))
	plans, dirty, err := m.Plan(ctx, migrator.IntentMigrate)
	require.NoError(t, err)
	require.False(t, dirty)

	for _, plan := range plans {
		require.NoError(t, m.Apply(ctx, plan))
	}
}

func seedApp(t *testing.T, ctx context.Context, values map[string]string) {
	for key, value := range values {
		upsert := &settings.Upsert{Key: key, Value: value}
		require.NoError(t, upsert.Execute(ctx, &settings.Settings{}))
	}
}

func TestLoadAppFromSettings(t *testing.T) {
	ctx := connectDB(t)
	migrateSettings(t, ctx)
	seedApp(t, ctx, map[string]string{
		"github.app_id":             "42",
		"github.app_private_key":    "-----BEGIN RSA PRIVATE KEY-----",
		"github.app_webhook_secret": "whsec",
		"github.app_client_id":      "Iv1.abc",
		"github.app_client_secret":  "csec",
	})

	app, err := loadApp(ctx)
	require.NoError(t, err)
	require.Equal(t, &App{
		AppID:         42,
		PrivateKey:    "-----BEGIN RSA PRIVATE KEY-----",
		WebhookSecret: "whsec",
		ClientID:      "Iv1.abc",
		ClientSecret:  "csec",
	}, app)
}

func TestLoadAppMissingCredIsNoApp(t *testing.T) {
	ctx := connectDB(t)
	migrateSettings(t, ctx)
	seedApp(t, ctx, map[string]string{
		"github.app_id":             "42",
		"github.app_private_key":    "-----BEGIN RSA PRIVATE KEY-----",
		"github.app_webhook_secret": "whsec",
		"github.app_client_id":      "Iv1.abc",
		// github.app_client_secret deliberately absent.
	})

	_, err := loadApp(ctx)
	require.ErrorIs(t, err, ErrNoApp)
}

// A missing settings table surfaces as the database error it is: this package
// assumes the schema exists — tolerating a pre-install world is the install
// fragment's concern alone (docs/spec/installation.md, install-safe checks).
func TestLoadAppMissingSettingsTableSurfaces(t *testing.T) {
	ctx := connectDB(t)

	_, err := loadApp(ctx)
	require.NotErrorIs(t, err, ErrNoApp)
	require.ErrorContains(t, err, "settings")
}

// A context with no database at all (nil boot DB — the installer composition before a
// DATABASE_URL exists) reads as "no app configured", never a panic: auth mounts in that
// composition and its handlers reach for the App.
func TestLoadAppNoDatabaseIsNoApp(t *testing.T) {
	_, err := loadApp(context.Background())
	require.ErrorIs(t, err, ErrNoApp)
}
