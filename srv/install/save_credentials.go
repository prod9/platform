package install

import (
	"context"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/github"
)

// SaveCredentials is the wizard's generated-keys step: one ungated POST saving the
// pair GitHub generates on the created App's settings page, both required
// (docs/spec/installation.md, "The install settings").
type SaveCredentials struct {
	PrivateKey   string `json:"private_key"`
	ClientSecret string `json:"client_secret"`
}

var _ controllers.Validator = (*SaveCredentials)(nil)

func (c *SaveCredentials) Validate() error {
	return validate.Multi(
		validate.Required("private_key", c.PrivateKey),
		validate.Required("client_secret", c.ClientSecret),
	)
}

// Execute writes the pair through github.SaveAppKeys — the one owner of that
// write — and suffix-resets every later step in the same transaction. The step is
// convergent: re-posting overwrites
// (docs/spec/installation.md, §Redo and suffix invalidation).
func (c *SaveCredentials) Execute(ctx context.Context, out any) error {
	return data.Run(ctx, func(s data.Scope) error {
		err := github.SaveAppKeys(s.Context(), &github.AppKeys{
			PrivateKey:   c.PrivateKey,
			ClientSecret: c.ClientSecret,
		})
		if err != nil {
			return err
		}
		return resetSuffix(s.Context(), stepAppCredentials)
	})
}
