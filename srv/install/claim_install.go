package install

import (
	"context"
	"errors"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrAlreadyInstalled reports a claim against a server that already holds its
// singleton install record — re-org is delete + re-install, never an overwrite.
var ErrAlreadyInstalled = errors.New("install: already installed")

// ClaimInstall writes the singleton install record — the org-owner claim's one
// mutation. InstallationID arrives from the request; every other field is derived
// server-side (the App API and the session) by the controller.
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

func (c *ClaimInstall) Execute(ctx context.Context, out any) error {
	err := data.Exec(ctx, `
		INSERT INTO installations
			(org_id, org_login, installation_id, installed_by_user_id, installed_by_login)
		VALUES ($1, $2, $3, $4, $5)`,
		c.OrgID, c.OrgLogin, c.InstallationID, c.UserID, c.UserLogin)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrAlreadyInstalled
	}
	return err
}
