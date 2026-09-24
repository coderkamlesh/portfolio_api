// Package repository holds every SQL statement of the API. Repositories only
// translate rows <-> models; business rules live in the service layer.
package repository

import (
	"database/sql"
	"errors"

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
