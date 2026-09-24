package repository

import (
	"context"
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
