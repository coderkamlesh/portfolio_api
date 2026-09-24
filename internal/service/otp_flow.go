package service

import (
	"context"
	"errors"
	"log"

	"github.com/coderkamlesh/portfolio_api/internal/email"
	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

// issueOTP invalidates any pending challenge of the same purpose, mails a new
// code and returns the client-facing view of the challenge.
//
// Guard rails: at most AUTH_OTP_MAX_PER_WINDOW codes per AUTH_OTP_WINDOW_MINUTES
// per admin, argon2id-hashed codes, and a hard expiry of AUTH_OTP_TTL_MINUTES.
func (s *AuthService) issueOTP(ctx context.Context, admin *models.AdminUser, purpose string, meta RequestMeta) (*ChallengeView, error) {
	now := s.now()

	sent, err := s.otps.CountCreatedSince(ctx, admin.ID, purpose, now.Add(-s.cfg.OTPWindow))
	if err != nil {
		return nil, err
	}
	if sent >= s.cfg.OTPMaxPerWindow {
		s.auditEvent(ctx, admin.ID, models.AuditRateLimited, "otp_window", map[string]any{"sent": sent})
		return nil, errOTPTooManyRequests()
	}

	// Only the newest code may be used.
	if err := s.otps.InvalidatePending(ctx, admin.ID, purpose, now); err != nil {
		return nil, err
	}

	code, err := security.GenerateOTP(s.cfg.OTPLength)
	if err != nil {
		return nil, wrapErr(err, 500, "internal_error", "Could not generate a verification code.")
	}
	otpHash, err := security.HashOTP(code)
	if err != nil {
		return nil, wrapErr(err, 500, "internal_error", "Could not generate a verification code.")
	}

	challenge := &models.OTPChallenge{
		ID:        ids.New(),
		AdminID:   admin.ID,
		OTPHash:   otpHash,
		Purpose:   purpose,
		ExpiresAt: now.Add(s.cfg.OTPTTL),
		CreatedAt: now,
	}
	if err := s.otps.Create(ctx, challenge); err != nil {
		return nil, err
	}

	msg := email.OTPMessage(admin.Email, email.OTPMail{
		Code:        code,
		Purpose:     purpose,
		TTL:         s.cfg.OTPTTL,
		RequestedAt: now,
		IPAddress:   meta.IPAddress,
		UserAgent:   sanitizeUserAgent(meta.UserAgent),
	})
	if err := s.mailer.Send(ctx, msg); err != nil {
		log.Printf("❌ auth: otp mail failed for admin %s: %v", admin.ID, err)
		if _, consumeErr := s.otps.Consume(ctx, challenge.ID, s.now()); consumeErr != nil {
			log.Printf("⚠️  auth: could not consume unsent challenge %s: %v", challenge.ID, consumeErr)
		}
		return nil, errEmailDeliveryFailed(err)
	}

	s.pruneExpired(ctx)
	return s.challengeView(challenge, admin.Email), nil
}

// loadChallenge fetches a challenge and validates its purpose and state,
// consuming nothing.
func (s *AuthService) loadChallenge(ctx context.Context, challengeID, purpose string) (*models.OTPChallenge, error) {
	if challengeID == "" {
		return nil, errInvalidChallenge()
	}
	challenge, err := s.otps.FindByID(ctx, challengeID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errInvalidChallenge()
		}
		return nil, err
	}
	if challenge.Purpose != purpose {
		return nil, errInvalidChallenge()
	}
	if challenge.IsConsumed() {
		return nil, errChallengeUsed()
	}
	if challenge.IsExpired(s.now()) {
		return nil, errChallengeExpired()
	}
	if challenge.AttemptCount >= s.cfg.OTPMaxAttempts {
		return nil, errOTPAttemptsExceeded()
	}
	return challenge, nil
}

// checkOTP verifies the submitted code, burns one attempt on failure and
// consumes the challenge on success. It returns the owning admin.
func (s *AuthService) checkOTP(ctx context.Context, challenge *models.OTPChallenge, code string) (*models.AdminUser, error) {
	admin, err := s.adminFor(ctx, challenge.AdminID)
	if err != nil {
		return nil, err
	}
	if !admin.IsActive {
		return nil, errAccountDisabled()
	}

	code = security.NormalizeOTP(code)
	if code == "" {
		return nil, errInvalidOTP(s.cfg.OTPMaxAttempts - challenge.AttemptCount)
	}

	ok, err := security.VerifyOTP(challenge.OTPHash, code)
	if err != nil {
		return nil, wrapErr(err, 500, "internal_error", "Stored verification code is unreadable.")
	}
	if !ok {
		attempts, incErr := s.otps.IncrementAttempts(ctx, challenge.ID)
		if incErr != nil {
			log.Printf("⚠️  auth: could not record otp attempt for %s: %v", challenge.ID, incErr)
			attempts = challenge.AttemptCount + 1
		}
		s.auditEvent(ctx, admin.ID, models.AuditOTPFailed,
			"invalid_code", map[string]any{"attempt": attempts, "purpose": challenge.Purpose})

		if remaining := s.cfg.OTPMaxAttempts - attempts; remaining > 0 {
			return nil, errInvalidOTP(remaining)
		}
		if _, consumeErr := s.otps.Consume(ctx, challenge.ID, s.now()); consumeErr != nil {
			log.Printf("⚠️  auth: could not consume locked challenge %s: %v", challenge.ID, consumeErr)
		}
		return nil, errOTPAttemptsExceeded()
	}

	// One-shot: a conditional UPDATE guarantees a single winner on races.
	consumed, err := s.otps.Consume(ctx, challenge.ID, s.now())
	if err != nil {
		return nil, err
	}
	if !consumed {
		return nil, errChallengeUsed()
	}
	return admin, nil
}

// challengeView projects a challenge for the API response.
func (s *AuthService) challengeView(c *models.OTPChallenge, emailAddress string) *ChallengeView {
	return &ChallengeView{
		ID:           c.ID,
		Purpose:      c.Purpose,
		Email:        security.MaskEmail(emailAddress),
		CodeLength:   s.cfg.OTPLength,
		ExpiresAt:    c.ExpiresAt,
		ExpiresIn:    maxInt(int(c.ExpiresAt.Sub(s.now()).Seconds()), 0),
		AttemptsLeft: maxInt(s.cfg.OTPMaxAttempts-c.AttemptCount, 0),
		ResendAfter:  int(s.cfg.OTPResendCooldown.Seconds()),
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
