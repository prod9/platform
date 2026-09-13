package builds

import "time"

const (
	moduleColumns = `bm.id, bm.build_id, bm.manifest_id, bm.manifest_module_id, mm.name,
		COALESCE(bm.claimed_at, '0001-01-01 00:00:00+00'::timestamptz) AS claimed_at,
		bm.claimed_by, bm.engine_host,
		COALESCE(bm.engine_assigned_at, '0001-01-01 00:00:00+00'::timestamptz) AS engine_assigned_at,
		bm.created_at`
	moduleFrom = `build_modules bm JOIN repo_manifest_modules mm ON mm.id = bm.manifest_module_id`
)

type BuildModule struct {
	ID               int64     `db:"id"`
	BuildID          int64     `db:"build_id"`
	ManifestID       int64     `db:"manifest_id"`
	ManifestModuleID int64     `db:"manifest_module_id"`
	Name             string    `db:"name"`
	ClaimedAt        time.Time `db:"claimed_at"`
	ClaimedBy        string    `db:"claimed_by"`
	EngineHost       string    `db:"engine_host"`
	EngineAssignedAt time.Time `db:"engine_assigned_at"`
	CreatedAt        time.Time `db:"created_at"`
}
