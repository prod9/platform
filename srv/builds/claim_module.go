package builds

import (
	"context"

	"fx.prodigy9.co/data"
)

// ClaimModule admits exactly one delivery of a module into execution.
type ClaimModule struct {
	ID       int64
	Hostname string
}

func (c *ClaimModule) Execute(ctx context.Context, out any) error {
	return data.Get(ctx, out, `WITH claimed AS (
		UPDATE build_modules SET claimed_at = now(), claimed_by = $2
		WHERE id = $1 AND claimed_at IS NULL RETURNING *
	) SELECT `+moduleColumns+` FROM claimed bm
		JOIN repo_manifest_modules mm ON mm.id = bm.manifest_module_id`, c.ID, c.Hostname)
}
