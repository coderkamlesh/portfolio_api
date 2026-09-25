package service

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
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

// ForgotPassword mails a reset code and returns a client-facing challenge.
// Unknown and inactive addresses receive a non-persisted challenge with the same
// response shape so response content cannot be used to enumerate admin accounts.
func (s *AuthService) ForgotPassword(ctx context.Context, in ForgotPasswordInput, meta RequestMeta) (*ChallengeView, error) {
	emailAddress := strings.ToLower(strings.TrimSpace(in.Email))
	if emailAddress == "" {
		return s.syntheticResetChallenge(emailAddress), nil
	}

	admin, err := s.admins.FindByIdentifier(ctx, emailAddress)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			log.Printf("🔐 auth: password reset requested for unknown address %s", emailAddress)
			return s.syntheticResetChallenge(emailAddress), nil
		}
		return nil, err
	}
	if !admin.IsActive {
		return s.syntheticResetChallenge(emailAddress), nil
	}

	challenge, err := s.issueOTP(ctx, admin, models.OTPPurposePasswordReset, meta)
	if err != nil {
		var apiErr *apierr.Error
		if !errors.As(err, &apiErr) ||
			(apiErr.Code != CodeOTPTooMany && apiErr.Code != CodeEmailDeliveryFailed) {
			return nil, err
		}
		// Rate limiting and mail failures use the same generic response as an
		// unknown account so the endpoint cannot reveal whether email exists.
		log.Printf("⚠️  auth: reset request not completed for admin %s: %v", admin.ID, err)
		return s.syntheticResetChallenge(emailAddress), nil
	}
	s.auditEvent(ctx, admin.ID, models.AuditPasswordResetAsked, "", nil)
	return challenge, nil
}

func (s *AuthService) syntheticResetChallenge(emailAddress string) *ChallengeView {
	now := s.now()
	return &ChallengeView{
		ID:           ids.New(),
		Purpose:      models.OTPPurposePasswordReset,
		Email:        security.MaskEmail(emailAddress),
		CodeLength:   s.cfg.OTPLength,
		ExpiresAt:    now.Add(s.cfg.OTPTTL),
		ExpiresIn:    int(s.cfg.OTPTTL.Seconds()),
		AttemptsLeft: s.cfg.OTPMaxAttempts,
		ResendAfter:  int(s.cfg.OTPResendCooldown.Seconds()),
	}
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
