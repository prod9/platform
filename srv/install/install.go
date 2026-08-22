// Package install is the platform server's installer fragment: the install.* settings
// binding the server to one org, the ordered install-state surface (GET /api/install),
// the migrations remediation, and the org-owner claim (POST /api/install/claim). The
// fragment remains mounted after claim and its routes are gated by install state.
package install

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"fx.prodigy9.co/app"
	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/errutil"
	"github.com/jmoiron/sqlx"
	"platform.prodigy9.co/srv/github"
)

var (
	App = app.Build().
		Name("install").
		Mount(settings.App).
		Middlewares(InstallerGate).
		Controllers(InstallCtr{})

	ErrAlreadyInstalled     = errors.New("install: already installed")
	ErrInstallationRequired = errutil.NewCoded(
		"installation_required", "platform installation is incomplete", nil,
	)
	// ErrNotInstalled reports that the server is not bound to an org yet — the install.*
	// settings are absent or still empty.
	ErrNotInstalled = errors.New("install: not installed")

	errNoDB        = errors.New("install: no database configured")
	errNotOrgOwner = errors.New("install: session user is not an owner of the installation's org")
	installKeys    = []string{keyOrgID, keyOrgLogin, keyInstallationID,
		keyInstalledByUserID, keyInstalledByLogin, keyInstalledAt}
)

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
		value, err := github.LoadSetting(ctx, field.key, ErrNotInstalled)
		if err != nil {
			return nil, err
		}
		if err := field.read(value); err != nil {
			return nil, fmt.Errorf("install: bad %s: %w", field.key, err)
		}
	}
	return record, nil
}

// IsInstalled reads the durable claim alone. It probes the settings schema before
// loading install.* so a fresh database is a normal uninstalled state.
func IsInstalled(ctx context.Context, db *sqlx.DB) (bool, error) {
	if db == nil {
		return false, errNoDB
	}

	ready, err := settingsSchemaReady(ctx, db)
	if err != nil || !ready {
		return false, err
	}

	_, err = Load(data.NewContext(ctx, db))
	if errors.Is(err, ErrNotInstalled) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
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
