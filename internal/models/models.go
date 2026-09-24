// Package models holds the domain structs of the portfolio CMS. Every struct
// mirrors a table in db_schema.sql one-to-one so repositories stay dumb.
package models

import (
	"errors"
	"time"
)

// ErrNotFound is returned by repositories when a row does not exist. The
// service layer translates it into the right API error.
var ErrNotFound = errors.New("models: not found")

// ErrConflict is returned by repositories when a UNIQUE constraint rejects a
// write. The service layer translates it into the 409 the admin panel expects,
// which is the safety net for a race between the service's duplicate check and
// the insert.
var ErrConflict = errors.New("models: conflict")

// 2FA methods (admin_2fa.method).
const (
	TwoFAMethodEmailOTP = "EMAIL_OTP"
	TwoFAMethodTOTP     = "TOTP"
)

// OTP purposes (otp_challenges.purpose).
const (
	OTPPurposeLogin2FA      = "LOGIN_2FA"
	OTPPurposePasswordReset = "PASSWORD_RESET"
)

// Audit actions written to audit_log.action.
const (
	AuditLoginSucceeded     = "LOGIN_SUCCEEDED"
	AuditLoginFailed        = "LOGIN_FAILED"
	AuditLoginChallenged    = "LOGIN_CHALLENGED"
	AuditOTPFailed          = "OTP_FAILED"
	AuditOTPResent          = "OTP_RESENT"
	AuditTokenRefreshed     = "TOKEN_REFRESHED"
	AuditTokenReuseDetected = "TOKEN_REUSE_DETECTED"
	AuditLoggedOut          = "LOGOUT"
	AuditPasswordChanged    = "PASSWORD_CHANGED"
	AuditPasswordReset      = "PASSWORD_RESET"
	AuditPasswordResetAsked = "PASSWORD_RESET_REQUESTED"
	AuditTwoFAEnabled       = "TWO_FA_ENABLED"
	AuditRateLimited        = "RATE_LIMITED"
)

// Audit entity types used by the auth module.
const (
	EntityAdmin = "admin_users"
	EntityAuth  = "auth"
)

// AdminUser maps admin_users.
type AdminUser struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	IsActive     bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TwoFactorConfig maps admin_2fa.
type TwoFactorConfig struct {
	ID          string
	AdminID     string
	Method      string
	TOTPSecret  string
	IsEnabled   bool
	ConfirmedAt *time.Time
	CreatedAt   time.Time
}

// OTPChallenge maps otp_challenges.
type OTPChallenge struct {
	ID           string
	AdminID      string
	OTPHash      string
	Purpose      string
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	AttemptCount int
	CreatedAt    time.Time
}

// IsExpired reports whether the challenge can no longer be used.
func (c *OTPChallenge) IsExpired(at time.Time) bool { return !at.Before(c.ExpiresAt) }

// IsConsumed reports whether the challenge was already used (or invalidated).
func (c *OTPChallenge) IsConsumed() bool { return c.ConsumedAt != nil }

// RefreshToken maps refresh_tokens. TokenHash is the SHA-256 of the opaque
// token handed to the client; the raw value is never stored.
type RefreshToken struct {
	ID        string
	AdminID   string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	UserAgent string
	IPAddress string
	CreatedAt time.Time
}

// IsUsable reports whether the token can still be exchanged for a new pair.
func (t *RefreshToken) IsUsable(at time.Time) bool {
	return t.RevokedAt == nil && at.Before(t.ExpiresAt)
}

// AuditEntry maps audit_log.
type AuditEntry struct {
	ID         string
	AdminID    string
	EntityType string
	EntityID   string
	Action     string
	OldValue   string
	NewValue   string
	CreatedAt  time.Time
}
