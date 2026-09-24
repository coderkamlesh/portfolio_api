package service

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

// ChangePassword rotates the password of the signed-in admin. Every other
// session is revoked and a fresh pair is returned so the caller stays signed
// in on the current device.
func (s *AuthService) ChangePassword(ctx context.Context, adminID string, in ChangePasswordInput, meta RequestMeta) (*TokenPair, error) {
	if err := s.validatePassword(in.NewPassword); err != nil {
		return nil, err
	}

	admin, err := s.verifyAdminPassword(ctx, adminID, in.CurrentPassword)
	if err != nil {
		return nil, err
	}

	same, err := security.VerifyPassword(admin.PasswordHash, in.NewPassword)
	if err != nil {
		return nil, wrapErr(err, 500, "internal_error", "Stored credentials are unreadable.")
	}
	if same {
		return nil, errSamePassword()
	}

	hash, err := security.HashPassword(in.NewPassword)
	if err != nil {
		return nil, wrapErr(err, 500, "internal_error", "Could not store the new password.")
	}
	if err := s.admins.UpdatePassword(ctx, admin.ID, hash, s.now()); err != nil {
		return nil, err
	}

	// Kill every existing session, then start a new one for this device.
	if _, err := s.refresh.RevokeAllForAdmin(ctx, admin.ID, s.now()); err != nil {
		log.Printf("⚠️  auth: could not revoke sessions after password change: %v", err)
	}
	tokens, err := s.issueSession(ctx, admin, meta)
	if err != nil {
		return nil, err
	}

	s.auditEvent(ctx, admin.ID, models.AuditPasswordChanged, "", nil)
	return tokens, nil
}

// ForgotPassword mails a reset code. It always reports success so the endpoint
// cannot be used to enumerate admin email addresses.
func (s *AuthService) ForgotPassword(ctx context.Context, in ForgotPasswordInput, meta RequestMeta) error {
	emailAddress := strings.ToLower(strings.TrimSpace(in.Email))
	if emailAddress == "" {
		return nil
	}

	admin, err := s.admins.FindByIdentifier(ctx, emailAddress)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			log.Printf("🔐 auth: password reset requested for unknown address %s", emailAddress)
			return nil
		}
		return err
	}
	if !admin.IsActive {
		return nil
	}

	if _, err := s.issueOTP(ctx, admin, models.OTPPurposePasswordReset, meta); err != nil {
		// Rate limiting is the only failure worth surfacing; mail problems are
		// reported as success to keep the response indistinguishable.
		var apiErr *apierr.Error
		if errors.As(err, &apiErr) && apiErr.Status == 429 {
			return err
		}
		log.Printf("⚠️  auth: reset mail failed for admin %s: %v", admin.ID, err)
		return nil
	}
	s.auditEvent(ctx, admin.ID, models.AuditPasswordResetAsked, "", nil)
	return nil
}

// ResetPassword completes the reset flow and signs the admin in.
func (s *AuthService) ResetPassword(ctx context.Context, in ResetPasswordInput, meta RequestMeta) (*LoginResult, error) {
	if err := s.validatePassword(in.NewPassword); err != nil {
		return nil, err
	}

	challenge, err := s.loadChallenge(ctx, in.ChallengeID, models.OTPPurposePasswordReset)
	if err != nil {
		return nil, err
	}

	admin, err := s.checkOTP(ctx, challenge, in.OTP)
	if err != nil {
		return nil, err
	}

	hash, err := security.HashPassword(in.NewPassword)
	if err != nil {
		return nil, wrapErr(err, 500, "internal_error", "Could not store the new password.")
	}
	if err := s.admins.UpdatePassword(ctx, admin.ID, hash, s.now()); err != nil {
		return nil, err
	}
	if _, err := s.refresh.RevokeAllForAdmin(ctx, admin.ID, s.now()); err != nil {
		log.Printf("⚠️  auth: could not revoke sessions after password reset: %v", err)
	}
	if err := s.admins.TouchLastLogin(ctx, admin.ID, s.now()); err != nil {
		log.Printf("⚠️  auth: could not record last login for admin %s: %v", admin.ID, err)
	}

	tokens, err := s.issueSession(ctx, admin, meta)
	if err != nil {
		return nil, err
	}
	s.auditEvent(ctx, admin.ID, models.AuditPasswordReset, "", nil)
	return &LoginResult{Tokens: tokens, Admin: adminView(admin)}, nil
}

// validatePassword enforces the password policy (AUTH_MIN_PASSWORD_LENGTH plus
// the shared mix rule) and maps violations to the API error.
func (s *AuthService) validatePassword(password string) error {
	if err := security.ValidatePassword(password, s.cfg.MinPasswordLength); err != nil {
		return errWeakPassword("Password must " + err.Error() + ".")
	}
	return nil
}
