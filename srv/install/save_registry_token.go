package install

import (
	"context"

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
// that write. The step is convergent: re-posting overwrites.
func (c *SaveRegistryToken) Execute(ctx context.Context, out any) error {
	return github.SaveRegistryToken(ctx, ghcrHost, c.Token)
}
