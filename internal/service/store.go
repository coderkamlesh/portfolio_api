package service

import (
	"context"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// AdminStore is the persistence contract for admin_users.
type AdminStore interface {
	FindByID(ctx context.Context, id string) (*models.AdminUser, error)
	FindByIdentifier(ctx context.Context, identifier string) (*models.AdminUser, error)
	TouchLastLogin(ctx context.Context, id string, at time.Time) error
	UpdatePassword(ctx context.Context, id, passwordHash string, at time.Time) error
}

// TwoFAStore is the persistence contract for admin_2fa.
type TwoFAStore interface {
	FindByAdminAndMethod(ctx context.Context, adminID, method string) (*models.TwoFactorConfig, error)
	ListByAdmin(ctx context.Context, adminID string) ([]models.TwoFactorConfig, error)
	Upsert(ctx context.Context, cfg *models.TwoFactorConfig) error
	SetEnabled(ctx context.Context, adminID, method string, enabled bool, at time.Time) error
}

// OTPStore is the persistence contract for otp_challenges.
type OTPStore interface {
	Create(ctx context.Context, c *models.OTPChallenge) error
	FindByID(ctx context.Context, id string) (*models.OTPChallenge, error)
	FindLatestPending(ctx context.Context, adminID, purpose string, at time.Time) (*models.OTPChallenge, error)
	CountCreatedSince(ctx context.Context, adminID, purpose string, since time.Time) (int, error)
	InvalidatePending(ctx context.Context, adminID, purpose string, at time.Time) error
	IncrementAttempts(ctx context.Context, id string) (int, error)
	Consume(ctx context.Context, id string, at time.Time) (bool, error)
	DeleteOlderThan(ctx context.Context, before time.Time) error
}

// RefreshStore is the persistence contract for refresh_tokens.
type RefreshStore interface {
	Create(ctx context.Context, t *models.RefreshToken) error
	FindByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	Revoke(ctx context.Context, id string, at time.Time) (bool, error)
	RevokeAllForAdmin(ctx context.Context, adminID string, at time.Time) (int64, error)
	CountActive(ctx context.Context, adminID string, at time.Time) (int, error)
	DeleteExpired(ctx context.Context, before time.Time) error
}

// AuditStore is the persistence contract for audit_log.
type AuditStore interface {
	Insert(ctx context.Context, e *models.AuditEntry) error
}
