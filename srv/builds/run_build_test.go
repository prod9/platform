package builds

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/srvtest"
)

func TestCloneTokenMintsFromInstallRecord(t *testing.T) {
	ctx := setupCloneToken(t)

	token, err := cloneToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "ghs_clone_tok", token)
}

func TestCloneTokenFailsWhenNotInstalled(t *testing.T) {
	ctx := setupCloneToken(t)
	require.NoError(t, data.Exec(ctx, `DELETE FROM installations`))

	_, err := cloneToken(ctx)
	require.ErrorIs(t, err, install.ErrNotInstalled)
}

// setupCloneToken is an installed server minus HTTP: migrated DB with the singleton
// install record, App credentials with a real signing key, and a fake GitHub minting
// installation tokens for installation 7.
func setupCloneToken(t *testing.T) context.Context {
	ctx := srvtest.SetupDB(t, migrator.FromFS(install.Migrations))
	require.NoError(t, data.Exec(ctx, `
		INSERT INTO installations
			(id, org_id, org_login, installation_id, installed_by_user_id, installed_by_login)
		VALUES (1, 9, 'prodigy9', 7, 1, 'chakrit')`))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /app/installations/7/access_tokens", func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(201)
		fmt.Fprint(resp, `{"token":"ghs_clone_tok"}`)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	origLoad := github.LoadApp
	github.LoadApp = func(ctx context.Context) (*github.App, error) {
		return &github.App{
			AppID:         42,
			PrivateKey:    string(keyPEM),
			WebhookSecret: "whsec",
			ClientID:      "Iv1.abc",
			ClientSecret:  "csec",
		}, nil
	}
	t.Cleanup(func() { github.LoadApp = origLoad })

	cfg := config.Configure()
	config.Set(cfg, github.APIURLConfig, server.URL)
	return config.NewContext(ctx, cfg)
}
