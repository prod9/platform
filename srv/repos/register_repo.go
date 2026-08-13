package repos

import (
	"context"
	"errors"

	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/validate"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrAlreadyRegistered reports a duplicate registration — the (owner, repo) row
// already exists, answered as the caller's conflict rather than a server fault.
var ErrAlreadyRegistered = errors.New("repos: repo is already registered")

// RegisterRepo records a registration — the one write the repos table takes;
// deregistration is not in this surface yet (spec §Repos are registered, visibility
// is live). Reachability by the installation is the controller's boundary check, not
// the action's: the action records, the transport layer authorizes.
type RegisterRepo struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	UserID int64  `json:"-"`
}

var _ controllers.Validator = (*RegisterRepo)(nil)

func (r *RegisterRepo) Validate() error {
	return validate.Multi(
		validate.Required("owner", r.Owner),
		validate.Required("repo", r.Repo),
	)
}

func (r *RegisterRepo) Execute(ctx context.Context, out any) error {
	err := data.Get(ctx, out, `
		INSERT INTO repos (owner, repo, registered_by)
		VALUES ($1, $2, $3)
		RETURNING *`,
		r.Owner, r.Repo, r.UserID)

	// 23505 is unique_violation on (owner, repo): registered already, by anyone.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrAlreadyRegistered
	}
	return err
}
