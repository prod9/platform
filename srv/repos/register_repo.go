package repos

import (
	"context"
	"errors"
	"maps"
	"slices"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"github.com/jackc/pgx/v5/pgconn"
	"platform.prodigy9.co/conf"
)

// ErrAlreadyRegistered reports a duplicate registration — the (owner, repo) row
// already exists, answered as the caller's conflict rather than a server fault.
var ErrAlreadyRegistered = errors.New("repos: repo is already registered")

// RegisterRepo records a registration — the one write the repos table takes;
// deregistration is not in this surface yet (spec §Repos are registered, visibility
// is live). Reachability by the installation is the controller's boundary check, not
// the action's: the action records, the transport layer authorizes.
type RegisterRepo struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	ManifestSHA string `json:"manifest_sha"`

	UserID      int64      `json:"-"`
	ManifestRaw string     `json:"-"`
	Manifest    conf.Model `json:"-"`
}

var _ controllers.Validator = (*RegisterRepo)(nil)

func (r *RegisterRepo) Validate() error {
	return validate.Multi(
		validate.Required("owner", r.Owner),
		validate.Required("repo", r.Repo),
		validate.Required("manifest_sha", r.ManifestSHA),
	)
}

func (r *RegisterRepo) Execute(ctx context.Context, out any) error {
	policy, err := r.Manifest.ResolveServerPublish(r.Repo)
	if err != nil {
		return err
	}
	r.Manifest.Server.Publish = policy

	err = data.Run(ctx, func(scope data.Scope) error {
		repo := &Repo{}
		if err := scope.Get(repo, `
			INSERT INTO repos (owner, repo, registered_by)
			VALUES ($1, $2, $3)
			RETURNING *`, r.Owner, r.Repo, r.UserID); err != nil {
			return err
		}

		snapshotID, err := insertManifestSnapshot(
			scope, repo.ID, r.ManifestSHA, r.ManifestRaw, r.Manifest)
		if err != nil {
			return err
		}
		for _, name := range slices.Sorted(maps.Keys(r.Manifest.Modules)) {
			if err := insertManifestModule(scope, snapshotID, name, *r.Manifest.Modules[name]); err != nil {
				return err
			}
		}

		if out == nil {
			return nil
		}
		return scope.Get(out, `SELECT * FROM repos WHERE id = $1`, repo.ID)
	})

	// 23505 is unique_violation on (owner, repo): registered already, by anyone.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrAlreadyRegistered
	}
	return err
}
