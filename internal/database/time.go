package database

import (
	"database/sql"
	"strings"
	"time"
)

// SQLiteTimeLayout is the text format SQLite's datetime('now') produces —
// which is what every DEFAULT in db_schema.sql uses. All timestamps written
// by the API use the same fixed-width UTC layout so that lexicographic
// string comparison in SQL is also chronological comparison.
const SQLiteTimeLayout = "2006-01-02 15:04:05"

// FormatTime renders t as a UTC SQLite datetime string.
func FormatTime(t time.Time) string {
	return t.UTC().Format(SQLiteTimeLayout)
}

// FormatTimePtr renders a nullable timestamp as a driver argument
// (string or nil).
func FormatTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return FormatTime(*t)
}

// ParseTime parses a SQLite datetime string, tolerating the "T" separator,
// fractional seconds and a trailing "Z" so values written by other tools work.
func ParseTime(s string) (time.Time, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(s), "Z")
	layouts := []string{
		SQLiteTimeLayout + ".999999999",
		"2006-01-02T15:04:05.999999999",
		SQLiteTimeLayout,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	var err error
	for _, layout := range layouts {
		var t time.Time
		t, err = time.ParseInLocation(layout, trimmed, time.UTC)
		if err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, err
}

// ParseTimePtr parses a nullable datetime column into a *time.Time.
func ParseTimePtr(v sql.NullString) (*time.Time, error) {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil, nil
	}
	t, err := ParseTime(v.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ParseTimeValue parses a required datetime column.
func ParseTimeValue(v string) (time.Time, error) {
	return ParseTime(v)
}
