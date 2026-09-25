// Package repository holds every SQL statement of the API. Repositories only
// translate rows <-> models; business rules live in the service layer.
package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// base gives every repository access to the Turso connection.
type base struct {
	db *database.DB
}

// rowScanner covers both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// notFound maps the driver's sentinel to the domain sentinel so services never
// have to import database/sql.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return models.ErrNotFound
	}
	return err
}

// affectedOrNotFound reports models.ErrNotFound when an UPDATE or DELETE
// matched no row, so services can map a missing row to a 404.
func affectedOrNotFound(res sql.Result, op string) error {
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if affected == 0 {
		return models.ErrNotFound
	}
	return nil
}

// conflictOr maps a UNIQUE-constraint failure to models.ErrConflict so the
// service layer can answer 409 instead of a generic 500. The driver error stays
// in the chain for the logs.
func conflictOr(op string, err error) error {
	if database.IsUniqueViolation(err) {
		return fmt.Errorf("%s: %w", op, errors.Join(models.ErrConflict, err))
	}
	return fmt.Errorf("%s: %w", op, err)
}

// boolToInt converts a Go bool for an INTEGER column.
func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// boolFromInt converts an INTEGER column (0/1, possibly NULL) into a bool.
func boolFromInt(v sql.NullInt64) bool {
	return v.Valid && v.Int64 != 0
}

// nullString turns an empty string into a SQL NULL so optional columns stay
// nullable.
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
