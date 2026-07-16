package srv

import (
	"testing"

	"fx.prodigy9.co/data"
	"github.com/stretchr/testify/require"
)

func TestGitHubAppSaveLoadRoundtrip(t *testing.T) {
	t.Setenv("SECRET", "the cake is a lie")
	ctx := setupDB(t)

	save := &SaveGitHubApp{
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

	app, err := LoadGitHubApp(ctx)
	require.NoError(t, err)
	require.Equal(t, &GitHubApp{
		AppID:         42,
		Slug:          "platform-test",
		PrivateKey:    "-----BEGIN RSA PRIVATE KEY-----",
		WebhookSecret: "whsec",
		ClientID:      "Iv1.abc",
		ClientSecret:  "csec",
	}, app)

	require.ErrorIs(t, save.Execute(ctx, nil), ErrGitHubAppExists)
}

func TestLoadGitHubAppWithoutRow(t *testing.T) {
	ctx := setupDB(t)

	_, err := LoadGitHubApp(ctx)
	require.ErrorIs(t, err, ErrNoGitHubApp)
}
