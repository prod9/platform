package srvtest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/github"
)

// StubApp overrides github.LoadApp for the test's duration, so any fragment's
// GitHub calls see the given credentials (or failure) instead of real config.
func StubApp(t *testing.T, app *github.App, err error) {
	orig := github.LoadApp
	github.LoadApp = func(ctx context.Context) (*github.App, error) { return app, err }
	t.Cleanup(func() { github.LoadApp = orig })
}

// TestApp is a full set of App credentials with a real signing key, so App-JWT
// calls against a fake GitHub authenticate for real.
func TestApp(t *testing.T) *github.App {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return &github.App{
		AppID:         42,
		PrivateKey:    string(keyPEM),
		WebhookSecret: "whsec",
		ClientID:      "Iv1.abc",
		ClientSecret:  "csec",
	}
}
