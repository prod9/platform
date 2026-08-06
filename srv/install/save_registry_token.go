package install

import (
	"context"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/github"
)

// ghcrHost is the one registry the wizard covers; more registries are the punted
// multi-registry UI (docs/spec/installation.md, "The registry token").
const ghcrHost = "ghcr.io"

// SaveRegistryToken is the wizard's registry step: one ungated POST saving the
// operator-created classic PAT — ghcr accepts no App-derived credential
// (docs/vendor/ghcr-auth.md).
type SaveRegistryToken struct {
	Token string `json:"token"`
}

var _ controllers.Validator = (*SaveRegistryToken)(nil)

func (c *SaveRegistryToken) Validate() error {
	return validate.Required("token", c.Token)
}

// Execute writes the token through github.SaveRegistryToken — the one owner of
// that write — and suffix-resets every later step in the same transaction. The
// step is convergent: re-posting overwrites
// (docs/spec/installation.md, §Redo and suffix invalidation).
func (c *SaveRegistryToken) Execute(ctx context.Context, out any) error {
	return data.Run(ctx, func(s data.Scope) error {
		if err := github.SaveRegistryToken(s.Context(), ghcrHost, c.Token); err != nil {
			return err
		}
		return resetSuffix(s.Context(), stepRegistryToken)
	})
}
