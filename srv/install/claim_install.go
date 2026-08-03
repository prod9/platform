package install

import (
	"context"
	"errors"
	"strconv"
	"time"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
)

// ErrAlreadyInstalled reports a claim against a server whose install.* settings already
// carry values — re-org is clearing them + re-install, never an overwrite.
var ErrAlreadyInstalled = errors.New("install: already installed")

// ClaimInstall fills the install.* settings — the org-owner claim's one mutation.
// InstallationID arrives from the request; every other field is derived server-side
// (the App API and the session) by the controller.
type ClaimInstall struct {
	InstallationID int64 `json:"installation_id"`

	OrgID     int64  `json:"-"`
	OrgLogin  string `json:"-"`
	UserID    int64  `json:"-"`
	UserLogin string `json:"-"`
}

var _ controllers.Validator = (*ClaimInstall)(nil)

func (c *ClaimInstall) Validate() error {
	return validate.Positive("installation_id", c.InstallationID)
}

// Execute writes every install.* value in one transaction. Concurrent claims
// serialize on the install.installation_id row: an empty-valued row is put in place if
// absent, locked with SELECT … FOR UPDATE, and a value already filled is
// ErrAlreadyInstalled — the second claim blocks on the lock and then finds the first
// one's write. The locking read is raw SQL; fx's settings API has no equivalent.
func (c *ClaimInstall) Execute(ctx context.Context, out any) error {
	return data.Run(ctx, func(s data.Scope) error {
		err := s.Exec(`INSERT INTO settings (key, value) VALUES ($1, '')
			ON CONFLICT (key) DO NOTHING`, keyInstallationID)
		if err != nil {
			return err
		}

		var current string
		err = s.Get(&current, `SELECT value FROM settings WHERE key = $1 FOR UPDATE`,
			keyInstallationID)
		if err != nil {
			return err
		}
		if current != "" {
			return ErrAlreadyInstalled
		}

		values := map[string]string{
			keyOrgID:             strconv.FormatInt(c.OrgID, 10),
			keyOrgLogin:          c.OrgLogin,
			keyInstallationID:    strconv.FormatInt(c.InstallationID, 10),
			keyInstalledByUserID: strconv.FormatInt(c.UserID, 10),
			keyInstalledByLogin:  c.UserLogin,
			keyInstalledAt:       time.Now().UTC().Format(time.RFC3339),
		}
		for key, value := range values {
			if err := setValue(s, key, value); err != nil {
				return err
			}
		}
		return nil
	})
}

// setValue upserts one settings row inside the claim's transaction. Temporary seam:
// fx's settings.Set updates only (no insert) as of v0.8.6 — once the upstream upsert
// lands and fx is upgraded, this collapses to settings.Set on the scope's context.
func setValue(s data.Scope, key, value string) error {
	return s.Exec(`INSERT INTO settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		key, value)
}
