package install

import (
	"context"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"platform.prodigy9.co/srv/github"
)

// SaveEngine is the wizard's bind-the-engine step: one ungated POST locking the
// infra-provided DAGGER_ENGINE seed into the engine.hosts setting — runtime reads
// the setting from then on (docs/spec/installation.md, the engine step).
type SaveEngine struct {
	Hosts string `json:"hosts"`
}

var _ controllers.Validator = (*SaveEngine)(nil)

func (c *SaveEngine) Validate() error {
	return validate.Required("hosts", c.Hosts)
}

// Execute writes the binding through github.SaveEngineHosts — the one owner of
// that write — and suffix-resets every later step in the same transaction
// (docs/spec/installation.md, §Redo and suffix invalidation).
func (c *SaveEngine) Execute(ctx context.Context, out any) error {
	return data.Run(ctx, func(s data.Scope) error {
		if err := github.SaveEngineHosts(s.Context(), c.Hosts); err != nil {
			return err
		}
		return resetSuffix(s.Context(), stepEngine)
	})
}
