package install

import (
	"context"
	"errors"
	"net/url"

	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/github"
)

// keyReceiverURL is the flux-setup step's one setting: the cluster Receiver URL the
// org delivery webhook targets. Its presence is the step's fully-ready
// (docs/spec/installation.md, the flux-setup step).
const keyReceiverURL = "flux.receiver_url"

var errNoReceiver = errors.New("install: no flux receiver configured")

// SetupFlux is the wizard's delivery step: a session-gated POST that converges the
// org-wide registry_package webhook onto the cluster's Flux Receiver through the
// App, then saves the URL as the step's record.
type SetupFlux struct {
	ReceiverURL string `json:"receiver_url"`
}

var _ controllers.Validator = (*SetupFlux)(nil)

func (c *SetupFlux) Validate() error {
	if err := validate.Required("receiver_url", c.ReceiverURL); err != nil {
		return err
	}

	parsed, err := url.Parse(c.ReceiverURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("receiver_url: must be an absolute https URL")
	}
	return nil
}

// Execute wires the webhook first and records the URL only after GitHub accepted it,
// so a saved receiver always means a wired org. Convergent: re-posting with the same
// URL finds the hook and only rewrites the setting.
func (c *SetupFlux) Execute(ctx context.Context, out any) error {
	record, err := Load(ctx)
	if err != nil {
		return err
	}
	client, err := github.NewClient(ctx)
	if err != nil {
		return err
	}
	token, err := client.InstallationToken(ctx, record.InstallationID)
	if err != nil {
		return err
	}

	if err := client.EnsureOrgWebhook(ctx, token, record.OrgLogin, c.ReceiverURL); err != nil {
		return err
	}

	upsert := &settings.Upsert{Key: keyReceiverURL, Value: c.ReceiverURL}
	return upsert.Execute(ctx, &settings.Settings{})
}
