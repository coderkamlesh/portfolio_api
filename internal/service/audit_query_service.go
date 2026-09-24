package service

import (
	"context"
	"strings"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// AuditQueryDeps wires the audit read side.
type AuditQueryDeps struct {
	Audit AuditStore
}

// AuditQueryService serves the admin audit trail.
type AuditQueryService struct {
	audit AuditStore
}

// NewAuditQueryService builds the service.
func NewAuditQueryService(deps AuditQueryDeps) *AuditQueryService {
	return &AuditQueryService{audit: deps.Audit}
}

// Audit pagination bounds. A trail grows without limit, so the page size is
// capped and the offset is rejected past a ceiling that would make the database
// walk a huge number of rows.
const (
	defaultAuditLimit = 50
	maxAuditLimit     = 200
	maxAuditOffset    = 10000
)

// AuditEntryView is one row of the trail as the admin panel needs it. The raw
// JSON snapshots are passed through as strings because the panel decides whether
// to pretty-print or diff them.
type AuditEntryView struct {
	ID         string `json:"id"`
	AdminID    string `json:"admin_id,omitempty"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id,omitempty"`
	Action     string `json:"action"`
	OldValue   string `json:"old_value,omitempty"`
	NewValue   string `json:"new_value,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// AuditPage is the paginated response.
type AuditPage struct {
	Entries []AuditEntryView `json:"entries"`
	Total   int64            `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

// Entries returns a filtered page of the trail, newest first.
func (s *AuditQueryService) Entries(ctx context.Context, entityType, action string, limit, offset int) (*AuditPage, error) {
	entityType = strings.ToLower(strings.TrimSpace(entityType))
	action = strings.ToUpper(strings.TrimSpace(action))

	if action != "" && !isAuditAction(action) {
		return nil, errAuditValidation("action must be one of CREATE, UPDATE, DELETE.")
	}

	limit = clampAuditLimit(limit)
	offset = clampAuditOffset(offset)

	entries, total, err := s.audit.ListAuditEntries(ctx, entityType, action, limit, offset)
	if err != nil {
		return nil, err
	}

	views := make([]AuditEntryView, 0, len(entries))
	for i := range entries {
		views = append(views, auditEntryView(&entries[i]))
	}
	return &AuditPage{Entries: views, Total: total, Limit: limit, Offset: offset}, nil
}

func auditEntryView(entry *models.AuditEntry) AuditEntryView {
	return AuditEntryView{
		ID:         entry.ID,
		AdminID:    entry.AdminID,
		EntityType: entry.EntityType,
		EntityID:   entry.EntityID,
		Action:     entry.Action,
		OldValue:   entry.OldValue,
		NewValue:   entry.NewValue,
		CreatedAt:  entry.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func isAuditAction(action string) bool {
	switch action {
	case AuditActionCreate, AuditActionUpdate, AuditActionDelete:
		return true
	default:
		return false
	}
}

func clampAuditLimit(limit int) int {
	if limit <= 0 {
		return defaultAuditLimit
	}
	if limit > maxAuditLimit {
		return maxAuditLimit
	}
	return limit
}

func clampAuditOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > maxAuditOffset {
		return maxAuditOffset
	}
	return offset
}
