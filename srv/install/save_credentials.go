package install

import (
	"context"

	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/github"
)

// SaveCredentials is the wizard's credential step: one ungated POST saving every
// github.app_* setting, all required (docs/spec/installation.md, "The install
// settings").
type SaveCredentials struct {
	AppID         int64  `json:"app_id"`
	PrivateKey    string `json:"private_key"`
	WebhookSecret string `json:"webhook_secret"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
}

var _ controllers.Validator = (*SaveCredentials)(nil)

func (c *SaveCredentials) Validate() error {
	return validate.Multi(
		validate.Positive("app_id", c.AppID),
		validate.Required("private_key", c.PrivateKey),
		validate.Required("webhook_secret", c.WebhookSecret),
		validate.Required("client_id", c.ClientID),
		validate.Required("client_secret", c.ClientSecret),
	)
}

// Execute writes the credentials through github.SaveApp — the one owner of the
// github.app_* write. The step is convergent: re-posting overwrites.
func (c *SaveCredentials) Execute(ctx context.Context, out any) error {
	return github.SaveApp(ctx, &github.App{
		AppID:         c.AppID,
		PrivateKey:    c.PrivateKey,
		WebhookSecret: c.WebhookSecret,
		ClientID:      c.ClientID,
		ClientSecret:  c.ClientSecret,
	})
}
