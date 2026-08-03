// Package install is the platform server's installer fragment: the install.* settings
// binding the server to one org, the ordered install-state surface (GET /api/install),
// the migrations remediation, and the org-owner claim (POST /api/install/claim). Boot
// mounts this fragment only while the server is not completely installed; product
// fragments have zero install awareness.
package install

import (
	"context"
	"errors"
	"strconv"
	"time"

	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/data/migrator"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrNotInstalled reports that the server is not bound to an org yet — the install.*
// settings are absent or still empty.
var ErrNotInstalled = errors.New("install: not installed")

// The hard-coded settings keys install state lives under (docs/spec/installation.md,
// "The install settings"). No migration defines them — an absent key reads as empty;
// the claim writes every value or none.
const (
	keyOrgID             = "install.org_id"
	keyOrgLogin          = "install.org_login"
	keyInstallationID    = "install.installation_id"
	keyInstalledByUserID = "install.installed_by_user_id"
	keyInstalledByLogin  = "install.installed_by_login"
	keyInstalledAt       = "install.installed_at"
)

// Record is the decoded install.* settings binding the server to one GitHub org. App
// credentials are deliberately absent — they are the github.app_* settings, owned by
// srv/github.
type Record struct {
	OrgID             int64
	OrgLogin          string
	InstallationID    int64
	InstalledByUserID int64
	InstalledByLogin  string
	InstalledAt       time.Time
}

// Load decodes the install.* settings. A missing settings table, absent keys, and
// empty values all mean "not installed" — the keys carry values only after the
// org-owner claim, and the claim writes all of them or none.
func Load(ctx context.Context) (*Record, error) {
	record := &Record{}
	fields := []struct {
		key  string
		read func(value string) error
	}{
		{keyOrgID, intField(&record.OrgID)},
		{keyOrgLogin, strField(&record.OrgLogin)},
		{keyInstallationID, intField(&record.InstallationID)},
		{keyInstalledByUserID, intField(&record.InstalledByUserID)},
		{keyInstalledByLogin, strField(&record.InstalledByLogin)},
		{keyInstalledAt, timeField(&record.InstalledAt)},
	}

	for _, field := range fields {
		value, err := loadValue(ctx, field.key)
		if err != nil {
			return nil, err
		}
		if err := field.read(value); err != nil {
			return nil, err
		}
	}
	return record, nil
}

// loadValue reads one install.* value, folding every not-installed shape into
// ErrNotInstalled: a missing settings table (42P01 — its migration has not run, a valid
// pre-install state), and an empty value — settings.Get folds an absent row into the
// fallback, so absent and empty (both: not yet claimed) arrive as "".
func loadValue(ctx context.Context, key string) (string, error) {
	value, err := settings.Get(ctx, key, "")

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return "", ErrNotInstalled
	} else if err != nil {
		return "", err
	} else if value == "" {
		return "", ErrNotInstalled
	}
	return value, nil
}

func intField(out *int64) func(string) error {
	return func(value string) (err error) {
		*out, err = strconv.ParseInt(value, 10, 64)
		return err
	}
}

func strField(out *string) func(string) error {
	return func(value string) error {
		*out = value
		return nil
	}
}

func timeField(out *time.Time) func(string) error {
	return func(value string) (err error) {
		*out, err = time.Parse(time.RFC3339, value)
		return err
	}
}

// Source is the install fragment's migration set: fx's settings schema, the storage
// the install.* keys live in. The fragment carries no SQL of its own — no migration
// defines the keys; they are hard-coded here and written by the claim
// (docs/spec/installation.md, "The install settings").
var Source = migrator.FromFS(*settings.App.EmbeddedMigrations())
