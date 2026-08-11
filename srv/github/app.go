// Package github owns the server's GitHub integration: the GitHub App credential set
// and the registry push tokens (settings.go), the repo-name whitelist every fragment
// taking owner/repo input shares, and the shared API-error summary.
package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

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

	values, err := loadAppSettings(ctx, keyAppID, keyPrivateKey, keyWebhookSecret, keyClientID, keyClientSecret)
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
