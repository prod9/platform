package install

import (
	"context"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/github"
)

// SaveApp is the wizard's create-the-App step: one ungated POST saving what
// GitHub's creation form yields — the App id, its URL slug, the client id, and
// the webhook secret the form was given — all required
// (docs/spec/installation.md, "The install settings").
type SaveApp struct {
	AppID         int64  `json:"app_id"`
	Slug          string `json:"app_slug"`
	ClientID      string `json:"client_id"`
	WebhookSecret string `json:"webhook_secret"`
}

var _ controllers.Validator = (*SaveApp)(nil)

func (c *SaveApp) Validate() error {
	return validate.Multi(
		validate.Positive("app_id", c.AppID),
		validate.Required("app_slug", c.Slug),
		validate.Required("client_id", c.ClientID),
		validate.Required("webhook_secret", c.WebhookSecret),
	)
}

// Execute writes the quartet through github.SaveAppCreation — the one owner of that
// write — and suffix-resets every later step in the same transaction. The step is
// convergent: re-posting overwrites
// (docs/spec/installation.md, §Redo and suffix invalidation).
func (c *SaveApp) Execute(ctx context.Context, out any) error {
	return data.Run(ctx, func(s data.Scope) error {
		err := github.SaveAppCreation(s.Context(), &github.AppCreation{
			AppID:         c.AppID,
			Slug:          c.Slug,
			ClientID:      c.ClientID,
			WebhookSecret: c.WebhookSecret,
		})
		if err != nil {
			return err
		}
		return resetSuffix(s.Context(), stepAppCreated)
	})
}
