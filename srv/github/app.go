// Package github owns the server's GitHub integration: the GitHub App credential set
// (supplied via fx config), the repo-name whitelist every fragment taking owner/repo
// input shares, and the shared API-error summary.
package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"fx.prodigy9.co/config"
)

// ErrNoApp reports that the GitHub App credentials are absent from config — how the
// installer detects "not yet configured".
var ErrNoApp = errors.New("github: no github app configured")

// LoadApp seams loadApp so fragment tests can stub the App without config plumbing.
var LoadApp = loadApp

// App is the server's GitHub App credential set. It is injected via fx config (a k8s
// Secret at rest, provided by the operator), never stored in the DB.
type App struct {
	AppID         int64
	PrivateKey    string
	WebhookSecret string
	ClientID      string
	ClientSecret  string
}

func loadApp(ctx context.Context) (*App, error) {
	cfg := config.FromContext(ctx)
	app := &App{
		AppID:         config.Get(cfg, AppIDConfig),
		PrivateKey:    config.Get(cfg, PrivateKeyConfig),
		WebhookSecret: config.Get(cfg, WebhookSecretConfig),
		ClientID:      config.Get(cfg, ClientIDConfig),
		ClientSecret:  config.Get(cfg, ClientSecretConfig),
	}

	if app.AppID == 0 ||
		app.PrivateKey == "" ||
		app.WebhookSecret == "" ||
		app.ClientID == "" ||
		app.ClientSecret == "" {
		return nil, ErrNoApp
	}

	return app, nil
}

// RespError summarizes a failed GitHub API response: op, status line, and up to 1KB
// of body. The body read is best-effort — GitHub's status already carries the
// verdict, so a read failure only shortens the message, never masks it.
func RespError(op string, resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	if err != nil {
		body = []byte("(unreadable body: " + err.Error() + ")")
	}
	return fmt.Errorf("github: %s failed: %d %s: %s",
		op, resp.StatusCode, resp.Status, body)
}
