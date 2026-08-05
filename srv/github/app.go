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

	values, err := loadSettings(ctx, keyAppID, keyPrivateKey, keyWebhookSecret, keyClientID, keyClientSecret)
	if err != nil {
		return nil, err
	}

	appID, err := parseAppID(values[keyAppID])
	if err != nil {
		return nil, err
	}

	return &App{
		AppID:         appID,
		PrivateKey:    values[keyPrivateKey],
		WebhookSecret: values[keyWebhookSecret],
		ClientID:      values[keyClientID],
		ClientSecret:  values[keyClientSecret],
	}, nil
}

// AppCreation is what GitHub's creation form yields: the App's id and client id,
// plus the webhook secret the wizard minted and the form was given. The generated
// keys (private key, client secret) arrive in a later wizard step
// (docs/spec/installation.md, "App creation").
type AppCreation struct {
	AppID         int64
	ClientID      string
	WebhookSecret string
}

// AppKeys is the pair generated on the created App's settings page.
type AppKeys struct {
	PrivateKey   string
	ClientSecret string
}

// SaveAppCreation writes the creation-time settings in one transaction — the
// app-created wizard step is its caller, and the step is convergent: re-posting
// overwrites.
func SaveAppCreation(ctx context.Context, creation *AppCreation) error {
	return saveSettings(ctx, map[string]string{
		keyAppID:         strconv.FormatInt(creation.AppID, 10),
		keyClientID:      creation.ClientID,
		keyWebhookSecret: creation.WebhookSecret,
	})
}

// SaveAppKeys writes the generated pair in one transaction — the app-credentials
// wizard step is its caller; same convergence.
func SaveAppKeys(ctx context.Context, keys *AppKeys) error {
	return saveSettings(ctx, map[string]string{
		keyPrivateKey:   keys.PrivateKey,
		keyClientSecret: keys.ClientSecret,
	})
}

// LoadAppCreation reads the creation-time trio, ErrNoApp when any is unset — how
// the app-created check reaches its verdict.
func LoadAppCreation(ctx context.Context) (*AppCreation, error) {
	values, err := loadSettings(ctx, keyAppID, keyClientID, keyWebhookSecret)
	if err != nil {
		return nil, err
	}

	appID, err := parseAppID(values[keyAppID])
	if err != nil {
		return nil, err
	}

	return &AppCreation{
		AppID:         appID,
		ClientID:      values[keyClientID],
		WebhookSecret: values[keyWebhookSecret],
	}, nil
}

// loadSettings reads the given keys inside one transaction — the writers are
// transactional, so reading inside one snapshot is what keeps a credential set from
// arriving torn. Any unset key is ErrNoApp.
func loadSettings(ctx context.Context, keys ...string) (map[string]string, error) {
	values := map[string]string{}
	err := data.Run(ctx, func(s data.Scope) error {
		for _, key := range keys {
			value, err := LoadSetting(s.Context(), key, ErrNoApp)
			if err != nil {
				return err
			}
			values[key] = value
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return values, nil
}

func parseAppID(value string) (int64, error) {
	appID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("github: bad %s: %w", keyAppID, err)
	}
	return appID, nil
}

func saveSettings(ctx context.Context, values map[string]string) error {
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

// requiredPermissions is the set the install wizard verifies against GET /app, in
// display order. Labels read as GitHub's UI names the permission, not the API slug
// (docs/spec/installation.md, the credentials check).
var requiredPermissions = []struct{ slug, label, level string }{
	{"contents", "contents", "write"},
	{"metadata", "metadata", "read"},
	{"members", "members", "read"},
}

// MissingPermissions names every required permission the App's map lacks, as
// "label: level" strings; write satisfies a read requirement. Empty means the App
// is fully scoped.
func MissingPermissions(perms map[string]string) []string {
	var missing []string
	for _, required := range requiredPermissions {
		held := perms[required.slug]
		if held == "write" || held == required.level {
			continue
		}
		missing = append(missing, required.label+": "+required.level)
	}
	return missing
}

// LoadSetting reads one settings value, folding an unset one into absent —
// settings.Get folds an absent row into the fallback, so absent and empty both
// arrive as "". It assumes the settings schema exists: tolerating a pre-install
// world is the install fragment's concern alone, guarded there by its schema probe
// (docs/spec/installation.md, install-safe checks). The App reads use it with
// ErrNoApp; the install fragment reads its install.* keys through it with its own
// sentinel.
func LoadSetting(ctx context.Context, key string, absent error) (string, error) {
	value, err := settings.Get(ctx, key, "")
	if err != nil {
		return "", err
	} else if value == "" {
		return "", absent
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
