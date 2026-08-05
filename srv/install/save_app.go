package install

import (
	"context"

	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/github"
)

// SaveApp is the wizard's create-the-App step: one ungated POST saving what
// GitHub's creation form yields — the App id, the client id, and the webhook
// secret the form was given — all required
// (docs/spec/installation.md, "The install settings").
type SaveApp struct {
	AppID         int64  `json:"app_id"`
	ClientID      string `json:"client_id"`
	WebhookSecret string `json:"webhook_secret"`
}

var _ controllers.Validator = (*SaveApp)(nil)

func (c *SaveApp) Validate() error {
	return validate.Multi(
		validate.Positive("app_id", c.AppID),
		validate.Required("client_id", c.ClientID),
		validate.Required("webhook_secret", c.WebhookSecret),
	)
}

// Execute writes the trio through github.SaveAppCreation — the one owner of that
// write. The step is convergent: re-posting overwrites.
func (c *SaveApp) Execute(ctx context.Context, out any) error {
	return github.SaveAppCreation(ctx, &github.AppCreation{
		AppID:         c.AppID,
		ClientID:      c.ClientID,
		WebhookSecret: c.WebhookSecret,
	})
}
