package install

import (
	"context"
	"errors"
	"strconv"
	"time"

	"fx.prodigy9.co/app/settings"
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

// Execute writes every install.* value in one transaction, keyed off a locked read of
// install.installation_id so two concurrent claims serialize: the second finds the
// value filled and gets ErrAlreadyInstalled. The lock is a raw SELECT … FOR UPDATE —
// fx's settings API has no locking read — while the writes go through settings.Set,
// which joins the transaction via the scope's context.
func (c *ClaimInstall) Execute(ctx context.Context, out any) error {
	return data.Run(ctx, func(s data.Scope) error {
		var current string
		err := s.Get(&current, `SELECT value FROM settings WHERE key = $1 FOR UPDATE`,
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
			if _, err := settings.Set(s.Context(), key, value); err != nil {
				return err
			}
		}
		return nil
	})
}
