package database

import (
	"errors"
	"strings"

	turso "turso.tech/database/tursogo-serverless"
)

// IsUniqueViolation reports whether err is a UNIQUE or PRIMARY KEY constraint
// failure reported by Turso.
//
// The server either sends a code (SQLITE_CONSTRAINT_UNIQUE) or, when codes are
// omitted, only a message — the driver's own tests cover both shapes — so the
// message is used as fallback. Foreign-key failures are deliberately left
// unmatched: they never carry "UNIQUE" in the message.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var tursoErr *turso.Error
	if !errors.As(err, &tursoErr) {
		return false
	}

	code := strings.ToUpper(tursoErr.Code + " " + tursoErr.ExtendedCode)
	if strings.Contains(code, "CONSTRAINT_UNIQUE") || strings.Contains(code, "CONSTRAINT_PRIMARYKEY") {
		return true
	}
	return strings.Contains(strings.ToUpper(tursoErr.Message), "UNIQUE")
}
