package github

// All wizard-written settings live here: the github.app_* credential keys, the
// registry.<host>.token push credentials, the server.public_url binding, and
// the transactional load/save helpers every accessor shares. No
// migration defines the keys — an absent key reads as empty; the install wizard
// writes them (docs/spec/installation.md, "The install settings").

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/data"
)

var (
	// ErrNoOrg reports that no primary-org slug is saved — how the installer's org
	// check reaches not-started.
	ErrNoOrg = errors.New("github: no org configured")

	// ErrNoRegistryToken reports that no push token is saved for the registry —
	// how the installer's registry-token check reaches not-started, and what stops
	// a build from attempting an unauthenticated push.
	ErrNoRegistryToken = errors.New("github: no registry token configured")

	// ErrNoPublicURL reports that no public URL is saved — how the installer's
	// server check reaches not-started, and what makes login refuse to run.
	ErrNoPublicURL = errors.New("github: no public URL configured")

	// LoadPublicURL reads the server's public URL, ErrNoPublicURL when unset. A
	// seam like LoadApp: auth's login tests stub it, everything else uses the
	// settings read.
	LoadPublicURL = func(ctx context.Context) (string, error) {
		return LoadSetting(ctx, keyPublicURL, ErrNoPublicURL)
	}
)

const (
	keyOrg           = "github.org"
	keyAppID         = "github.app_id"
	keyAppSlug       = "github.app_slug"
	keyPrivateKey    = "github.app_private_key"
	keyWebhookSecret = "github.app_webhook_secret"
	keyClientID      = "github.app_client_id"
	keyClientSecret  = "github.app_client_secret"

	keyPublicURL = "server.public_url"
)

// AppCreation is what GitHub's creation form yields: the App's id, its URL slug
// (what the wizard's direct App links are built from), and client id, plus the
// webhook secret the wizard minted and the form was given. The generated keys
// (private key, client secret) arrive in a later wizard step
// (docs/spec/installation.md, "App creation").
type AppCreation struct {
	AppID         int64
	Slug          string
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
		keyAppSlug:       creation.Slug,
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
	values, err := loadSettings(ctx, keyAppID, keyAppSlug, keyClientID, keyWebhookSecret)
	if err != nil {
		return nil, err
	}

	appID, err := parseAppID(values[keyAppID])
	if err != nil {
		return nil, err
	}

	return &AppCreation{
		AppID:         appID,
		Slug:          values[keyAppSlug],
		ClientID:      values[keyClientID],
		WebhookSecret: values[keyWebhookSecret],
	}, nil
}

// SavePublicURL writes the server's public URL — the server wizard step is its
// caller; re-posting overwrites.
func SavePublicURL(ctx context.Context, publicURL string) error {
	return saveSettings(ctx, map[string]string{keyPublicURL: publicURL})
}

// SaveOrg writes the primary-org slug — the org wizard step is its caller;
// re-posting overwrites.
func SaveOrg(ctx context.Context, org string) error {
	return saveSettings(ctx, map[string]string{keyOrg: org})
}

// LoadOrg reads the primary-org slug, ErrNoOrg when unset.
func LoadOrg(ctx context.Context) (string, error) {
	return LoadSetting(ctx, keyOrg, ErrNoOrg)
}

// The Clear* writers empty a step's settings — suffix invalidation's mechanism: an
// empty value already reads as unset, so states flip with the plain upsert and
// nothing needs a delete verb (docs/spec/installation.md, §Redo and suffix
// invalidation). Each clears exactly the keys its wizard step owns.

func ClearOrg(ctx context.Context) error {
	return saveSettings(ctx, map[string]string{keyOrg: ""})
}

func ClearPublicURL(ctx context.Context) error {
	return saveSettings(ctx, map[string]string{keyPublicURL: ""})
}

func ClearAppCreation(ctx context.Context) error {
	return saveSettings(ctx, map[string]string{
		keyAppID:         "",
		keyAppSlug:       "",
		keyClientID:      "",
		keyWebhookSecret: "",
	})
}

func ClearAppKeys(ctx context.Context) error {
	return saveSettings(ctx, map[string]string{
		keyPrivateKey:   "",
		keyClientSecret: "",
	})
}

func ClearRegistryToken(ctx context.Context, host string) error {
	return saveSettings(ctx, map[string]string{registryTokenKey(host): ""})
}

// SaveRegistryToken writes the push credential for one registry host — the
// registry-token wizard step is its caller; re-posting overwrites. Only the token
// is stored: the publish path pairs it with the installation record's login
// (docs/spec/installation.md, "The registry token").
func SaveRegistryToken(ctx context.Context, host, token string) error {
	return saveSettings(ctx, map[string]string{registryTokenKey(host): token})
}

// LoadRegistryToken reads the push credential for one registry host,
// ErrNoRegistryToken when unset.
func LoadRegistryToken(ctx context.Context, host string) (string, error) {
	return LoadSetting(ctx, registryTokenKey(host), ErrNoRegistryToken)
}

func registryTokenKey(host string) string {
	return "registry." + host + ".token"
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

func parseAppID(value string) (int64, error) {
	appID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("github: bad %s: %w", keyAppID, err)
	}
	return appID, nil
}
