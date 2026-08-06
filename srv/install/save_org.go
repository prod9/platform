package install

import (
	"context"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/github"
)

// SaveOrg is the wizard's name-the-org step: one ungated POST saving the primary-org
// slug every later panel's GitHub links are built from
// (docs/spec/installation.md, the state surface).
type SaveOrg struct {
	Org string `json:"org"`
}

var _ controllers.Validator = (*SaveOrg)(nil)

func (c *SaveOrg) Validate() error {
	return validate.Required("org", c.Org)
}

// Execute writes the slug through github.SaveOrg — the one owner of that write —
// and suffix-resets every later step in the same transaction
// (docs/spec/installation.md, §Redo and suffix invalidation).
func (c *SaveOrg) Execute(ctx context.Context, out any) error {
	return data.Run(ctx, func(s data.Scope) error {
		if err := github.SaveOrg(s.Context(), c.Org); err != nil {
			return err
		}
		return resetSuffix(s.Context(), stepOrg)
	})
}
