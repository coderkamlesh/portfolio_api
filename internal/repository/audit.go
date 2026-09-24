package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// AuditRepository appends to audit_log.
type AuditRepository struct {
	base
}

// NewAuditRepository binds the repository to the database connection.
func NewAuditRepository(db *database.DB) *AuditRepository {
	return &AuditRepository{base{db: db}}
}

// Insert records one audit entry. Callers treat failures as non-fatal: an
// audit hiccup must never block a login.
func (r *AuditRepository) Insert(ctx context.Context, e *models.AuditEntry) error {
	createdAt := e.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	const q = `INSERT INTO audit_log (id, admin_id, entity_type, entity_id, action, old_value, new_value, created_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, e.ID, nullString(e.AdminID), e.EntityType, nullString(e.EntityID),
		e.Action, nullString(e.OldValue), nullString(e.NewValue), database.FormatTime(createdAt))
	if err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}

// ListAuditEntries returns a page of the trail, newest first, with the total
// number of rows matching the same filters.
//
// The filters are applied as bound parameters with an empty-string fallback
// rather than by concatenating SQL, so a caller cannot inject through the
// query string.
func (r *AuditRepository) ListAuditEntries(ctx context.Context, entityType, action string, limit, offset int) ([]models.AuditEntry, int64, error) {
	total, err := r.CountAuditEntries(ctx, entityType, action)
	if err != nil {
		return nil, 0, err
	}

	const q = `SELECT id, admin_id, entity_type, entity_id, action, old_value, new_value, created_at
	           FROM audit_log
	          WHERE (? = '' OR entity_type = ?) AND (? = '' OR action = ?)
	          ORDER BY created_at DESC, id DESC
	          LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, q, entityType, entityType, action, action, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("audit: list: %w", err)
	}
	defer rows.Close()

	entries := make([]models.AuditEntry, 0, limit)
	for rows.Next() {
		var (
			entry     models.AuditEntry
			adminID   sql.NullString
			entityID  sql.NullString
			oldValue  sql.NullString
			newValue  sql.NullString
			createdAt string
		)
		if err := rows.Scan(&entry.ID, &adminID, &entry.EntityType, &entityID, &entry.Action,
			&oldValue, &newValue, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("audit: scan: %w", err)
		}
		entry.AdminID = adminID.String
		entry.EntityID = entityID.String
		entry.OldValue = oldValue.String
		entry.NewValue = newValue.String
		if entry.CreatedAt, err = database.ParseTime(createdAt); err != nil {
			return nil, 0, fmt.Errorf("audit: parse created_at: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("audit: rows: %w", err)
	}
	return entries, total, nil
}

// CountAuditEntries reports how many rows match the filters.
func (r *AuditRepository) CountAuditEntries(ctx context.Context, entityType, action string) (int64, error) {
	const q = `SELECT COUNT(*) FROM audit_log
	           WHERE (? = '' OR entity_type = ?) AND (? = '' OR action = ?)`
	var total int64
	if err := r.db.QueryRowContext(ctx, q, entityType, entityType, action, action).Scan(&total); err != nil {
		return 0, fmt.Errorf("audit: count: %w", err)
	}
	return total, nil
}
