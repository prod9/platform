// Package github owns the server's GitHub integration: the stored GitHub App
// credential set and its manifest-flow bootstrap, App-authenticated token minting,
// and the repo-name whitelist every fragment taking owner/repo input shares.
package github

import (
	"context"
	"errors"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/secret"
	"platform.prodigy9.co/srv/pgerr"
)

var (
	ErrNoApp     = errors.New("github: no github app configured")
	ErrAppExists = errors.New("github: a github app is already configured")
)

// LoadApp seams loadApp so fragment tests run without postgres.
var LoadApp = loadApp

// App is the server's GitHub App credential set in decrypted, in-memory form. At
// rest (the single-row github_app table) private_key, webhook_secret, and
// client_secret are encrypted with fx's secret package (SECRET config var).
type App struct {
	AppID         int64
	Slug          string
	PrivateKey    string
	WebhookSecret string
	ClientID      string
	ClientSecret  string
}

func loadApp(ctx context.Context) (*App, error) {
	var row struct {
		AppID         int64  `db:"app_id"`
		Slug          string `db:"slug"`
		PrivateKey    string `db:"private_key"`
		WebhookSecret string `db:"webhook_secret"`
		ClientID      string `db:"client_id"`
		ClientSecret  string `db:"client_secret"`
	}
	err := data.Get(ctx, &row, `
		SELECT app_id, slug, private_key, webhook_secret, client_id, client_secret
		FROM github_app WHERE id = 1`)
	if data.IsNoRows(err) {
		return nil, ErrNoApp
	} else if err != nil {
		return nil, err
	}

	cfg := config.FromContext(ctx)
	privateKey, err := secret.Reveal(cfg, row.PrivateKey)
	if err != nil {
		return nil, err
	}
	webhookSecret, err := secret.Reveal(cfg, row.WebhookSecret)
	if err != nil {
		return nil, err
	}
	clientSecret, err := secret.Reveal(cfg, row.ClientSecret)
	if err != nil {
		return nil, err
	}

	return &App{
		AppID:         row.AppID,
		Slug:          row.Slug,
		PrivateKey:    privateKey,
		WebhookSecret: webhookSecret,
		ClientID:      row.ClientID,
		ClientSecret:  clientSecret,
	}, nil
}

// SaveApp records the App credentials received from the manifest exchange. The App
// is created once — a second save is a hard error, never an upsert.
type SaveApp struct {
	AppID         int64
	Slug          string
	PrivateKey    string
	WebhookSecret string
	ClientID      string
	ClientSecret  string
}

func (s *SaveApp) Execute(ctx context.Context, out any) error {
	cfg := config.FromContext(ctx)
	privateKey, err := secret.Hide(cfg, s.PrivateKey)
	if err != nil {
		return err
	}
	webhookSecret, err := secret.Hide(cfg, s.WebhookSecret)
	if err != nil {
		return err
	}
	clientSecret, err := secret.Hide(cfg, s.ClientSecret)
	if err != nil {
		return err
	}

	err = data.Exec(ctx, `
		INSERT INTO github_app (app_id, slug, private_key, webhook_secret, client_id, client_secret)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		s.AppID, s.Slug, privateKey, webhookSecret, s.ClientID, clientSecret)
	if pgerr.IsUniqueViolation(err) {
		return ErrAppExists
	}
	return err
}
