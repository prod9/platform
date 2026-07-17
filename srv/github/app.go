// Package github owns the server's GitHub integration: the GitHub App credential set
// (supplied via fx config), App-authenticated token minting, and the repo-name
// whitelist every fragment taking owner/repo input shares.
package github

import (
	"context"
	"errors"

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
	Slug          string
	PrivateKey    string
	WebhookSecret string
	ClientID      string
	ClientSecret  string
}

func loadApp(ctx context.Context) (*App, error) {
	cfg := config.FromContext(ctx)
	app := &App{
		AppID:         config.Get(cfg, AppIDConfig),
		Slug:          config.Get(cfg, SlugConfig),
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
