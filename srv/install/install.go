// Package install is the platform server's installer fragment: the install.* settings
// binding the server to one org, the ordered install-state surface (GET /api/install),
// the migrations remediation, and the org-owner claim (POST /api/install/claim). Boot
// mounts this fragment only while the server is not completely installed; product
// fragments have zero install awareness.
package install

import (
	"context"
	"embed"
	"errors"
	"strconv"
	"time"

	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"github.com/jackc/pgx/v5/pgconn"
	"platform.prodigy9.co/srv/migrate"
)

// ErrNotInstalled reports that the server is not bound to an org yet — the install.*
// settings are absent, unseeded, or still empty.
var ErrNotInstalled = errors.New("install: not installed")

// The pre-defined settings keys install state lives under (docs/spec/installation.md,
// "The install settings"). Seeded empty by this fragment's migration; the claim fills
// every value or none.
const (
	keyOrgID             = "install.org_id"
	keyOrgLogin          = "install.org_login"
	keyInstallationID    = "install.installation_id"
	keyInstalledByUserID = "install.installed_by_user_id"
	keyInstalledByLogin  = "install.installed_by_login"
	keyInstalledAt       = "install.installed_at"
)

// Record is the decoded install.* settings binding the server to one GitHub org. App
// credentials are deliberately absent — they live in fx config, never in the DB.
type Record struct {
	OrgID             int64
	OrgLogin          string
	InstallationID    int64
	InstalledByUserID int64
	InstalledByLogin  string
	InstalledAt       time.Time
}

// Load decodes the install.* settings. A missing settings table, unseeded keys, and
// seeded-but-empty values all mean "not installed" — the keys carry values only after
// the org-owner claim, and the claim writes all of them or none.
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
// pre-install state), a missing row (unseeded), and an empty value (not yet claimed).
func loadValue(ctx context.Context, key string) (string, error) {
	setting, err := settings.Get(ctx, key)

	var pgErr *pgconn.PgError
	undefinedTable := errors.As(err, &pgErr) && pgErr.Code == "42P01"
	if data.IsNoRows(err) || undefinedTable {
		return "", ErrNotInstalled
	} else if err != nil {
		return "", err
	} else if setting.Value == "" {
		return "", ErrNotInstalled
	}
	return setting.Value, nil
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

// Migrations seeds the install.* settings rows; srv aggregates every fragment's SQL at
// boot via Source, which carries the settings table these rows live in.
//
//go:embed *.sql
var Migrations embed.FS

// Source is the install fragment's whole migration set: fx's settings schema (the
// storage the install.* keys live in) plus this fragment's seed rows, interleaved by
// timestamp like any fragment's own SQL.
var Source = migrate.Merged(
	migrator.FromFS(*settings.App.EmbeddedMigrations()),
	migrator.FromFS(Migrations),
)
