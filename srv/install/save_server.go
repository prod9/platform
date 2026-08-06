package install

import (
	"context"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/github"
)

// SaveServer is the wizard's name-the-server step: one ungated POST saving the
// public URL every later panel's server-side URL renders from
// (docs/spec/installation.md, the state surface).
type SaveServer struct {
	PublicURL string `json:"public_url"`
}

var _ controllers.Validator = (*SaveServer)(nil)

func (c *SaveServer) Validate() error {
	return validate.Required("public_url", c.PublicURL)
}

// Execute writes the URL through github.SavePublicURL — the one owner of that
// write — and suffix-resets every later step in the same transaction
// (docs/spec/installation.md, §Redo and suffix invalidation).
func (c *SaveServer) Execute(ctx context.Context, out any) error {
	return data.Run(ctx, func(s data.Scope) error {
		if err := github.SavePublicURL(s.Context(), c.PublicURL); err != nil {
			return err
		}
		return resetSuffix(s.Context(), stepServer)
	})
}
