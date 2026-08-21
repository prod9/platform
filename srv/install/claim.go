package install

import (
	"context"
	"strconv"
	"time"

	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/github"
)

// ClaimInstall fills the install.* settings — the org-owner claim's one mutation.
// InstallationID arrives from the request; every other field is derived server-side
// (the App API and the session) by the controller.
type Claim struct {
	GitHubInstallationID int64  `json:"installation_id"`
	GitHubOrgID          int64  `json:"-"`
	GitHubOrgLogin       string `json:"-"`

	PlatformUserID    int64  `json:"-"`
	PlatformUserLogin string `json:"-"`
}

var _ controllers.Validator = (*Claim)(nil)

func (c *Claim) Validate() error {
	return validate.Positive("installation_id", c.GitHubInstallationID)
}

// Execute writes every install.* value in one transaction. Concurrent claims
// serialize on the install.installation_id row: an empty-valued row is put in place if
// absent, locked with SELECT … FOR UPDATE, and a value already filled is
// ErrAlreadyInstalled — the second claim blocks on the lock and then finds the first
// one's write. The locking read is raw SQL; fx's settings API has no equivalent.
func (c *Claim) Execute(ctx context.Context, out any) error {
	if err := c.claimGithub(ctx); err != nil {
		return err
	}
	return c.claimInstall(ctx)
}

func (c *Claim) claimGithub(ctx context.Context) error {
	client, err := github.NewClient(ctx)
	if err != nil {
		return err
	}
	org, owner, err := claimedOrgOwner(ctx, client, c.GitHubInstallationID, c.PlatformUserLogin)
	if err != nil {
		return err
	}
	if !owner {
		return errNotOrgOwner
	}
	c.GitHubOrgID, c.GitHubOrgLogin = org.ID, org.Login
	return nil
}

func (c *Claim) claimInstall(ctx context.Context) error {
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
			keyOrgID:             strconv.FormatInt(c.GitHubOrgID, 10),
			keyOrgLogin:          c.GitHubOrgLogin,
			keyInstallationID:    strconv.FormatInt(c.GitHubInstallationID, 10),
			keyInstalledByUserID: strconv.FormatInt(c.PlatformUserID, 10),
			keyInstalledByLogin:  c.PlatformUserLogin,
			keyInstalledAt:       time.Now().UTC().Format(time.RFC3339),
		}
		for key, value := range values {
			upsert := &settings.Upsert{Key: key, Value: value}
			if err := upsert.Execute(s.Context(), &settings.Settings{}); err != nil {
				return err
			}
		}
		return nil
	})
}

func claimedOrgOwner(ctx context.Context, client *github.Client, installationID int64, user string) (*github.Org, bool, error) {
	org, err := client.InstallationOrg(ctx, installationID)
	if err != nil {
		return nil, false, err
	}
	token, err := client.InstallationToken(ctx, installationID)
	if err != nil {
		return nil, false, err
	}
	owner, err := client.IsOrgOwner(ctx, token, org.Login, user)
	if err != nil {
		return nil, false, err
	}
	return org, owner, nil
}
