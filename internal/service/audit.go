package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
)

// actorContextKey carries the acting admin through the call chain. The audit
// entries are written deep inside the service methods, which have no access to
// the HTTP request, so the handler attaches the actor once and every write
// method below can read it.
type actorContextKey struct{}

// WithActor returns a context carrying the acting admin. Handlers call it before
// invoking a write use-case.
func WithActor(ctx context.Context, adminID string) context.Context {
	if adminID == "" {
		return ctx
	}
	return context.WithValue(ctx, actorContextKey{}, adminID)
}

// ActorFromContext returns the acting admin, if any.
func ActorFromContext(ctx context.Context) string {
	adminID, _ := ctx.Value(actorContextKey{}).(string)
	return adminID
}

// Audit action values written to audit_log.action for content changes.
const (
	AuditActionCreate = "CREATE"
	AuditActionUpdate = "UPDATE"
	AuditActionDelete = "DELETE"
)

// Content entity types written to audit_log.entity_type. They match the table
// each change belongs to, so the admin panel can group the trail.
const (
	auditEntityProfile      = "profile"
	auditEntitySkillCategory = "skill_category"
	auditEntitySkill        = "skill"
	auditEntityExperience   = "experience"
	auditEntityProject      = "project"
	auditEntityEducation    = "education"
	auditEntityExtra        = "extra"
	auditEntitySocialLink   = "social_link"
)

// Audit records content changes for the admin trail.
//
// Every method is safe to call on a zero value: with no store attached the
// recorder is a no-op, so a service can be built without an audit dependency in
// tests or in a deployment that does not keep the trail.
type Audit struct {
	store AuditStore
	now   func() time.Time
}

// NewAudit builds a recorder. A nil store disables recording.
func NewAudit(store AuditStore, now func() time.Time) *Audit {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Audit{store: store, now: now}
}

// Record appends one audit row for a content change. oldValue and newValue may be
// nil, which is how a create and a delete are represented.
//
// Write failures are logged and swallowed: a full audit table must not roll back
// a change the admin already made, and losing one trail row is preferable to
// losing the edit.
func (a *Audit) Record(ctx context.Context, entityType, entityID, action string, oldValue, newValue any) {
	if a == nil || a.store == nil {
		return
	}

	entry := &models.AuditEntry{
		ID:         ids.New(),
		AdminID:    ActorFromContext(ctx),
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		OldValue:   marshalAuditValue(oldValue),
		NewValue:   marshalAuditValue(newValue),
		CreatedAt:  a.now(),
	}
	if err := a.store.Insert(ctx, entry); err != nil {
		log.Printf("audit: record %s %s: %v", action, entityType, err)
	}
}

// marshalAuditValue renders a JSON snapshot. A nil value stays empty so a create
// or a delete leaves the unused side blank rather than writing the string "null".
func marshalAuditValue(value any) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		log.Printf("audit: marshal %T: %v", value, err)
		return ""
	}
	return string(raw)
}
