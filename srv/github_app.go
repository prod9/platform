package srv

import (
	"context"
	"errors"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/secret"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNoGitHubApp     = errors.New("srv: no github app configured")
	ErrGitHubAppExists = errors.New("srv: a github app is already configured")
)

// GitHubApp is the server's GitHub App credential set in decrypted, in-memory form.
// At rest (the single-row github_app table) private_key, webhook_secret, and
// client_secret are encrypted with fx's secret package (SECRET config var).
type GitHubApp struct {
	AppID         int64
	Slug          string
	PrivateKey    string
	WebhookSecret string
	ClientID      string
	ClientSecret  string
}

func LoadGitHubApp(ctx context.Context) (*GitHubApp, error) {
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
		return nil, ErrNoGitHubApp
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

	return &GitHubApp{
		AppID:         row.AppID,
		Slug:          row.Slug,
		PrivateKey:    privateKey,
		WebhookSecret: webhookSecret,
		ClientID:      row.ClientID,
		ClientSecret:  clientSecret,
	}, nil
}

// SaveGitHubApp records the App credentials received from the manifest exchange. The
// App is created once — a second save is a hard error, never an upsert.
type SaveGitHubApp struct {
	AppID         int64
	Slug          string
	PrivateKey    string
	WebhookSecret string
	ClientID      string
	ClientSecret  string
}

func (s *SaveGitHubApp) Execute(ctx context.Context, out any) error {
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
	if isUniqueViolation(err) {
		return ErrGitHubAppExists
	}
	return err
}

func isUniqueViolation(err error) bool {
	var pgerr *pgconn.PgError
	return errors.As(err, &pgerr) && pgerr.Code == "23505"
}
