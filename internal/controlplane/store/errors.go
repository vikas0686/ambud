// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres SQLSTATE codes this package translates into sentinel
// errors instead of leaking the raw driver error. See
// https://www.postgresql.org/docs/current/errcodes-appendix.html.
const (
	postgresUniqueViolation     = "23505"
	postgresForeignKeyViolation = "23503"
)

// isUniqueViolation reports whether err is a Postgres unique
// constraint violation, so callers can translate it into
// ErrAlreadyExists instead of a raw driver error.
func isUniqueViolation(err error) bool {
	return pgErrorCode(err) == postgresUniqueViolation
}

// isForeignKeyViolation reports whether err is a Postgres foreign key
// violation — e.g. creating a workload against a node ID that doesn't
// exist — so callers can translate it into ErrNotFound.
func isForeignKeyViolation(err error) bool {
	return pgErrorCode(err) == postgresForeignKeyViolation
}

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
