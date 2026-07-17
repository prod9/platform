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

// IsUndefinedTable reports a query against a table that does not exist — how the
// installer reads a singleton whose migration has not run yet as "not installed".
func IsUndefinedTable(err error) bool {
	var pgerr *pgconn.PgError
	return errors.As(err, &pgerr) && pgerr.Code == "42P01"
}
