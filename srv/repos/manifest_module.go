package repos

import (
	"context"

	"fx.prodigy9.co/data"
)

type ManifestModule struct {
	ID         int64  `db:"id"`
	ManifestID int64  `db:"manifest_id"`
	Name       string `db:"name"`
	ImageName  string `db:"image_name"`
}

func ManifestModules(ctx context.Context, manifestID int64) ([]ManifestModule, error) {
	modules := []ManifestModule{}
	err := data.Select(ctx, &modules, `SELECT id, manifest_id, name, image_name
		FROM repo_manifest_modules WHERE manifest_id = $1 ORDER BY id`, manifestID)
	return modules, err
}
