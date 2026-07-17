// Package pgerr classifies postgres driver errors for the srv fragments.
package pgerr

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func IsUniqueViolation(err error) bool {
	var pgerr *pgconn.PgError
	return errors.As(err, &pgerr) && pgerr.Code == "23505"
}
