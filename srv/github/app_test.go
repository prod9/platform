package github

import (
	"context"
	"testing"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/srvtest"
)

func setupDB(t *testing.T) context.Context {
	return srvtest.SetupDB(t, migrator.FromFS(Migrations))
}

func TestAppSaveLoadRoundtrip(t *testing.T) {
	ctx := setupDB(t)

	save := &SaveApp{
		AppID:         42,
		Slug:          "platform-test",
		PrivateKey:    "-----BEGIN RSA PRIVATE KEY-----",
		WebhookSecret: "whsec",
		ClientID:      "Iv1.abc",
		ClientSecret:  "csec",
	}
	require.NoError(t, save.Execute(ctx, nil))

	var raw struct {
		PrivateKey    string `db:"private_key"`
		WebhookSecret string `db:"webhook_secret"`
		ClientSecret  string `db:"client_secret"`
	}
	require.NoError(t, data.Get(ctx, &raw, `
		SELECT private_key, webhook_secret, client_secret
		FROM github_app WHERE id = 1`))
	require.NotEqual(t, save.PrivateKey, raw.PrivateKey)
	require.NotEqual(t, save.WebhookSecret, raw.WebhookSecret)
	require.NotEqual(t, save.ClientSecret, raw.ClientSecret)

	app, err := loadApp(ctx)
	require.NoError(t, err)
	require.Equal(t, &App{
		AppID:         42,
		Slug:          "platform-test",
		PrivateKey:    "-----BEGIN RSA PRIVATE KEY-----",
		WebhookSecret: "whsec",
		ClientID:      "Iv1.abc",
		ClientSecret:  "csec",
	}, app)

	require.ErrorIs(t, save.Execute(ctx, nil), ErrAppExists)
}

func TestLoadAppWithoutRow(t *testing.T) {
	ctx := setupDB(t)

	_, err := loadApp(ctx)
	require.ErrorIs(t, err, ErrNoApp)
}
