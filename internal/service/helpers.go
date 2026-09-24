package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
)

// pruneOlderThan is the retention window for expired OTP challenges and
// revoked/expired refresh tokens (housekeeping only — they are never valid
// after their own expiry).
const pruneOlderThan = 7 * 24 * time.Hour

// adminFor loads an admin and maps "missing row" to invalid credentials, so an
// endpoint never confirms that an account was deleted.
func (s *AuthService) adminFor(ctx context.Context, adminID string) (*models.AdminUser, error) {
	admin, err := s.admins.FindByID(ctx, adminID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errInvalidCredentials()
		}
		return nil, err
	}
	return admin, nil
}

// adminView projects the account to its public shape.
func adminView(a *models.AdminUser) *AdminView {
	if a == nil {
		return nil
	}
	return &AdminView{
		ID:          a.ID,
		Username:    a.Username,
		Email:       a.Email,
		IsActive:    a.IsActive,
		LastLoginAt: a.LastLoginAt,
	}
}

// auditEvent writes an audit_log row. Failures are logged but never bubble up:
// authentication must not depend on the audit table being writable.
func (s *AuthService) auditEvent(ctx context.Context, adminID, action, reason string, extra map[string]any) {
	payload := map[string]any{}
	if reason != "" {
		payload["reason"] = reason
	}
	for k, v := range extra {
		payload[k] = v
	}

	newValue := ""
	if len(payload) > 0 {
		if encoded, err := json.Marshal(payload); err == nil {
			newValue = string(encoded)
		}
	}

	entry := &models.AuditEntry{
		ID:         ids.New(),
		AdminID:    adminID,
		EntityType: models.EntityAuth,
		Action:     action,
		NewValue:   newValue,
		CreatedAt:  s.now(),
	}
	if err := s.audit.Insert(ctx, entry); err != nil {
		log.Printf("⚠️  auth: audit insert failed (%s): %v", action, err)
	}
}

// pruneExpired deletes stale rows in the background. It intentionally runs
// detached from the request so a cleanup failure can never fail a login.
func (s *AuthService) pruneExpired(ctx context.Context) {
	cutoff := s.now().Add(-pruneOlderThan)
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)

	go func() {
		defer cancel()
		if err := s.otps.DeleteOlderThan(cleanupCtx, cutoff); err != nil {
			log.Printf("⚠️  auth: otp prune failed: %v", err)
		}
		if err := s.refresh.DeleteExpired(cleanupCtx, cutoff); err != nil {
			log.Printf("⚠️  auth: refresh prune failed: %v", err)
		}
	}()
}

// sanitizeUserAgent keeps audit/session rows tidy and bounded.
func sanitizeUserAgent(ua string) string {
	ua = strings.TrimSpace(ua)
	if len(ua) > 255 {
		ua = ua[:255]
	}
	return ua
}
