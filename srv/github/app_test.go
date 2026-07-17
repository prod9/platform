package github

import (
	"context"
	"testing"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
)

func appContext(t *testing.T) context.Context {
	t.Setenv("GITHUB_APP_ID", "42")
	t.Setenv("GITHUB_APP_SLUG", "platform-test")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "-----BEGIN RSA PRIVATE KEY-----")
	t.Setenv("GITHUB_APP_WEBHOOK_SECRET", "whsec")
	t.Setenv("GITHUB_APP_CLIENT_ID", "Iv1.abc")
	t.Setenv("GITHUB_APP_CLIENT_SECRET", "csec")
	return config.NewContext(context.Background(), fxtest.Configure())
}

func TestLoadAppFromConfig(t *testing.T) {
	app, err := loadApp(appContext(t))
	require.NoError(t, err)
	require.Equal(t, &App{
		AppID:         42,
		Slug:          "platform-test",
		PrivateKey:    "-----BEGIN RSA PRIVATE KEY-----",
		WebhookSecret: "whsec",
		ClientID:      "Iv1.abc",
		ClientSecret:  "csec",
	}, app)
}

func TestLoadAppMissingCredIsNoApp(t *testing.T) {
	t.Setenv("GITHUB_APP_ID", "42")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "-----BEGIN RSA PRIVATE KEY-----")
	t.Setenv("GITHUB_APP_WEBHOOK_SECRET", "whsec")
	t.Setenv("GITHUB_APP_CLIENT_ID", "Iv1.abc")
	// GITHUB_APP_CLIENT_SECRET deliberately absent.

	ctx := config.NewContext(context.Background(), fxtest.Configure())
	_, err := loadApp(ctx)
	require.ErrorIs(t, err, ErrNoApp)
}

func TestLoadAppSlugOptional(t *testing.T) {
	t.Setenv("GITHUB_APP_ID", "42")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "-----BEGIN RSA PRIVATE KEY-----")
	t.Setenv("GITHUB_APP_WEBHOOK_SECRET", "whsec")
	t.Setenv("GITHUB_APP_CLIENT_ID", "Iv1.abc")
	t.Setenv("GITHUB_APP_CLIENT_SECRET", "csec")
	// GITHUB_APP_SLUG deliberately absent — slug is optional.

	app, err := loadApp(config.NewContext(context.Background(), fxtest.Configure()))
	require.NoError(t, err)
	require.Equal(t, "", app.Slug)
}
