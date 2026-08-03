// Package github owns the server's GitHub integration: the GitHub App credential set
// (the github.app_* settings), the repo-name whitelist every fragment taking owner/repo
// input shares, and the shared API-error summary.
package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/data"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNoApp reports that the GitHub App credentials are absent or incomplete — how
	// the installer detects "not yet configured".
	ErrNoApp = errors.New("github: no github app configured")

	// LoadApp seams loadApp so fragment tests can stub the App without settings plumbing.
	LoadApp = loadApp

	repoNamePattern = regexp.MustCompile(`^[A-Za-z0-9-][A-Za-z0-9._-]*$`)
)

// The hard-coded settings keys the App credentials live under
// (docs/spec/installation.md, "The install settings"). No migration defines them — an
// absent key reads as empty; the wizard's credential step writes them all.
const (
	keyAppID         = "github.app_id"
	keyPrivateKey    = "github.app_private_key"
	keyWebhookSecret = "github.app_webhook_secret"
	keyClientID      = "github.app_client_id"
	keyClientSecret  = "github.app_client_secret"
)

// App is the server's GitHub App credential set. It lives in the github.app_* settings
// — the database, not fx config — so srv and the worker read the same rows
// (docs/spec/platform-server.md, "Auth mechanism").
type App struct {
	AppID         int64
	PrivateKey    string
	WebhookSecret string
	ClientID      string
	ClientSecret  string
}

func loadApp(ctx context.Context) (*App, error) {
	if _, ok := data.LookupFromContext(ctx); !ok {
		return nil, ErrNoApp
	}

	values := map[string]string{}
	for _, key := range []string{keyAppID, keyPrivateKey, keyWebhookSecret, keyClientID, keyClientSecret} {
		value, err := loadValue(ctx, key)
		if err != nil {
			return nil, err
		}
		values[key] = value
	}

	appID, err := strconv.ParseInt(values[keyAppID], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("github: bad %s: %w", keyAppID, err)
	}

	return &App{
		AppID:         appID,
		PrivateKey:    values[keyPrivateKey],
		WebhookSecret: values[keyWebhookSecret],
		ClientID:      values[keyClientID],
		ClientSecret:  values[keyClientSecret],
	}, nil
}

// SaveApp writes every github.app_* setting in one transaction — the wizard's
// credential step is its caller, and the step is convergent: re-posting overwrites.
func SaveApp(ctx context.Context, app *App) error {
	values := map[string]string{
		keyAppID:         strconv.FormatInt(app.AppID, 10),
		keyPrivateKey:    app.PrivateKey,
		keyWebhookSecret: app.WebhookSecret,
		keyClientID:      app.ClientID,
		keyClientSecret:  app.ClientSecret,
	}

	return data.Run(ctx, func(s data.Scope) error {
		for key, value := range values {
			upsert := &settings.Upsert{Key: key, Value: value}
			if err := upsert.Execute(s.Context(), &settings.Settings{}); err != nil {
				return err
			}
		}
		return nil
	})
}

// loadValue reads one github.app_* value, folding every not-configured shape into
// ErrNoApp: a missing settings table (42P01 — its migration has not run, a valid
// pre-install state) and an empty value — settings.Get folds an absent row into the
// fallback, so absent and empty both arrive as "".
func loadValue(ctx context.Context, key string) (string, error) {
	value, err := settings.Get(ctx, key, "")

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return "", ErrNoApp
	} else if err != nil {
		return "", err
	} else if value == "" {
		return "", ErrNoApp
	}
	return value, nil
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

// CheckRepoPath admits only names GitHub itself allows (letters, digits, '-', plus
// '._' in repo names, never leading '.') — owner/repo land in filesystem paths and
// API URLs, so the whitelist is what keeps a hostile payload from escaping them.
func CheckRepoPath(owner, repo string) error {
	if !repoNamePattern.MatchString(owner) || !repoNamePattern.MatchString(repo) {
		return fmt.Errorf("github: invalid repo path: %q/%q", owner, repo)
	}
	return nil
}
