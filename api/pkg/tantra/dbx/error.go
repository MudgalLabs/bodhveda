package dbx

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const pgErrUniqueViolation = "23505" // PostgreSQL error code for unique constraint violation

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgErrUniqueViolation
	}
	return false
}
