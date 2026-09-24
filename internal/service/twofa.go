package service

import (
	"context"
	"errors"
	"log"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

// emailTwoFAConfig resolves whether email OTP must be used for this admin.
//
// Rules:
//   - a row with is_enabled = 1 means 2FA is on;
//   - when AUTH_2FA_REQUIRED=true the second factor can never be skipped: a
//     missing row is auto-provisioned (no enrollment is needed for email OTP)
//     and a disabled row is re-enabled;
//   - when AUTH_2FA_REQUIRED=false the admin's own setting decides.
//
// The bool return value reports whether an OTP must be sent.
func (s *AuthService) emailTwoFAConfig(ctx context.Context, admin *models.AdminUser) (*models.TwoFactorConfig, bool, error) {
	cfg, err := s.twoFA.FindByAdminAndMethod(ctx, admin.ID, models.TwoFAMethodEmailOTP)
	switch {
	case err == nil:
		if cfg.IsEnabled {
			return cfg, true, nil
		}
		if !s.cfg.TwoFARequired {
			return cfg, false, nil
		}

		now := s.now()
		cfg.IsEnabled = true
		cfg.ConfirmedAt = &now
		if err := s.twoFA.SetEnabled(ctx, admin.ID, cfg.Method, true, now); err != nil {
			return nil, false, err
		}
		s.auditEvent(ctx, admin.ID, models.AuditTwoFAEnabled, "enforced_by_config", nil)
		return cfg, true, nil

	case errors.Is(err, models.ErrNotFound):
		if !s.cfg.TwoFARequired {
			return nil, false, nil
		}
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
			return nil, false, err
		}
		s.auditEvent(ctx, admin.ID, models.AuditTwoFAEnabled, "auto_provisioned", nil)
		return cfg, true, nil

	default:
		return nil, false, err
	}
}

// TwoFAStatus reports the effective 2FA state: Enabled is true when the
// second factor is required either by server policy or by the admin setting.
func (s *AuthService) TwoFAStatus(ctx context.Context, admin *models.AdminUser) (*TwoFAStatus, error) {
	status := &TwoFAStatus{
		Required: s.cfg.TwoFARequired,
		Method:   models.TwoFAMethodEmailOTP,
		Email:    security.MaskEmail(admin.Email),
	}

	cfg, err := s.twoFA.FindByAdminAndMethod(ctx, admin.ID, models.TwoFAMethodEmailOTP)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			status.Enabled = s.cfg.TwoFARequired
			return status, nil
		}
		return nil, err
	}

	status.Enabled = cfg.IsEnabled || s.cfg.TwoFARequired
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

// DisableEmailOTP turns the second factor off. It refuses when the server
// enforces 2FA (AUTH_2FA_REQUIRED=true) so a compromised session cannot strip
// the protection.
func (s *AuthService) DisableEmailOTP(ctx context.Context, adminID string, in PasswordConfirmationInput) error {
	if s.cfg.TwoFARequired {
		return errTwoFAEnforced()
	}

	admin, err := s.verifyAdminPassword(ctx, adminID, in.Password)
	if err != nil {
		return err
	}

	if err := s.twoFA.SetEnabled(ctx, admin.ID, models.TwoFAMethodEmailOTP, false, s.now()); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return errTwoFANotConfigured()
		}
		return err
	}
	s.auditEvent(ctx, admin.ID, models.AuditTwoFADisabled, "admin_setting", nil)
	return nil
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
