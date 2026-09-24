package service

import (
	"context"
	"errors"
	"log"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

// emailTwoFAConfig resolves the mandatory email OTP configuration for this
// admin. A missing row is auto-provisioned and a stale disabled row is
// re-enabled, so password verification always leads to a second-factor
// challenge.
func (s *AuthService) emailTwoFAConfig(ctx context.Context, admin *models.AdminUser) (*models.TwoFactorConfig, error) {
	cfg, err := s.twoFA.FindByAdminAndMethod(ctx, admin.ID, models.TwoFAMethodEmailOTP)
	switch {
	case err == nil:
		if cfg.IsEnabled {
			return cfg, nil
		}

		now := s.now()
		cfg.IsEnabled = true
		cfg.ConfirmedAt = &now
		if err := s.twoFA.SetEnabled(ctx, admin.ID, cfg.Method, true, now); err != nil {
			return nil, err
		}
		s.auditEvent(ctx, admin.ID, models.AuditTwoFAEnabled, "mandatory_policy", nil)
		return cfg, nil

	case errors.Is(err, models.ErrNotFound):
		now := s.now()
		cfg = &models.TwoFactorConfig{
			ID:          ids.New(),
			AdminID:     admin.ID,
			Method:      models.TwoFAMethodEmailOTP,
			IsEnabled:   true,
			ConfirmedAt: &now,
			CreatedAt:   now,
		}
		if err := s.twoFA.Upsert(ctx, cfg); err != nil {
			return nil, err
		}
		s.auditEvent(ctx, admin.ID, models.AuditTwoFAEnabled, "auto_provisioned", nil)
		return cfg, nil

	default:
		return nil, err
	}
}

// TwoFAStatus reports the mandatory email OTP state. Enabled and Required are
// always true; a missing stored row is provisioned during the next login.
func (s *AuthService) TwoFAStatus(ctx context.Context, admin *models.AdminUser) (*TwoFAStatus, error) {
	status := &TwoFAStatus{
		Required: true,
		Method:   models.TwoFAMethodEmailOTP,
		Enabled:  true,
		Email:    security.MaskEmail(admin.Email),
	}

	cfg, err := s.twoFA.FindByAdminAndMethod(ctx, admin.ID, models.TwoFAMethodEmailOTP)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return status, nil
		}
		return nil, err
	}

	status.ConfirmedAt = cfg.ConfirmedAt
	created := cfg.CreatedAt
	status.UpdatedAt = &created
	return status, nil
}

// TwoFAStatusByAdminID is the handler-friendly variant of TwoFAStatus.
func (s *AuthService) TwoFAStatusByAdminID(ctx context.Context, adminID string) (*TwoFAStatus, error) {
	admin, err := s.adminFor(ctx, adminID)
	if err != nil {
		return nil, err
	}
	return s.TwoFAStatus(ctx, admin)
}

// EnableEmailOTP turns the email second factor on (password re-confirmation
// required) and returns the fresh status.
func (s *AuthService) EnableEmailOTP(ctx context.Context, adminID string, in PasswordConfirmationInput) (*TwoFAStatus, error) {
	admin, err := s.verifyAdminPassword(ctx, adminID, in.Password)
	if err != nil {
		return nil, err
	}

	now := s.now()
	cfg := &models.TwoFactorConfig{
		ID:          ids.New(),
		AdminID:     admin.ID,
		Method:      models.TwoFAMethodEmailOTP,
		IsEnabled:   true,
		ConfirmedAt: &now,
		CreatedAt:   now,
	}
	if err := s.twoFA.Upsert(ctx, cfg); err != nil {
		return nil, err
	}
	s.auditEvent(ctx, admin.ID, models.AuditTwoFAEnabled, "admin_setting", nil)
	return s.TwoFAStatus(ctx, admin)
}

// verifyAdminPassword re-authenticates a signed-in admin for sensitive actions.
func (s *AuthService) verifyAdminPassword(ctx context.Context, adminID, password string) (*models.AdminUser, error) {
	admin, err := s.admins.FindByID(ctx, adminID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errInvalidCredentials()
		}
		return nil, err
	}
	if !admin.IsActive {
		return nil, errAccountDisabled()
	}
	if password == "" {
		return nil, errInvalidPassword()
	}

	ok, err := security.VerifyPassword(admin.PasswordHash, password)
	if err != nil {
		return nil, wrapErr(err, 500, "internal_error", "Stored credentials are unreadable.")
	}
	if !ok {
		log.Printf("⚠️  auth: password re-confirmation failed for admin %s", admin.ID)
		return nil, errInvalidPassword()
	}
	return admin, nil
}
