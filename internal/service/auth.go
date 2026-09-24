package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/config"
	"github.com/coderkamlesh/portfolio_api/internal/email"
	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

// Deps wires the service to its collaborators. Every field is an interface so
// the auth flows can be unit-tested with in-memory doubles.
type Deps struct {
	Admins  AdminStore
	TwoFA   TwoFAStore
	OTPs    OTPStore
	Refresh RefreshStore
	Audit   AuditStore
	Mailer  email.Sender
	Tokens  *security.TokenManager
	Cfg     *config.Config
	// Now is injectable for deterministic tests; defaults to time.Now (UTC).
	Now func() time.Time
}

// AuthService owns every admin authentication flow.
type AuthService struct {
	admins       AdminStore
	twoFA        TwoFAStore
	otps         OTPStore
	refresh      RefreshStore
	audit        AuditStore
	mailer       email.Sender
	tokens       *security.TokenManager
	cfg          *config.Config
	now          func() time.Time
	loginLimiter *security.Limiter
}

// NewAuthService builds the service.
func NewAuthService(deps Deps) *AuthService {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &AuthService{
		admins:       deps.Admins,
		twoFA:        deps.TwoFA,
		otps:         deps.OTPs,
		refresh:      deps.Refresh,
		audit:        deps.Audit,
		mailer:       deps.Mailer,
		tokens:       deps.Tokens,
		cfg:          deps.Cfg,
		now:          now,
		loginLimiter: security.NewLimiter(deps.Cfg.LoginMaxAttempts, deps.Cfg.LoginWindow),
	}
}

// Login validates the password, ensures mandatory email OTP is active, mails a
// code, and returns the challenge that must be completed before a session is
// issued.
func (s *AuthService) Login(ctx context.Context, in LoginInput, meta RequestMeta) (*LoginResult, error) {
	identifier := strings.TrimSpace(in.Identifier)
	if identifier == "" || in.Password == "" {
		return nil, errInvalidCredentials()
	}

	limitKey := security.ClientKey(identifier, meta.IPAddress)
	if !s.loginLimiter.Allow(limitKey) {
		s.auditEvent(ctx, "", models.AuditRateLimited, "login", map[string]any{"identifier": identifier})
		return nil, errRateLimited(int(s.loginLimiter.RetryAfter(limitKey).Seconds()) + 1)
	}

	admin, err := s.admins.FindByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			// Burn the same CPU as a real verification so the response time
			// does not disclose whether the account exists.
			_, _ = security.VerifyPassword(security.DummyPasswordHash, in.Password)
			s.loginLimiter.RegisterFailure(limitKey)
			s.auditEvent(ctx, "", models.AuditLoginFailed, "unknown_identifier", map[string]any{"identifier": identifier})
			return nil, errInvalidCredentials()
		}
		return nil, err
	}

	if !admin.IsActive {
		s.loginLimiter.RegisterFailure(limitKey)
		s.auditEvent(ctx, admin.ID, models.AuditLoginFailed, "inactive", nil)
		return nil, errAccountDisabled()
	}

	ok, err := security.VerifyPassword(admin.PasswordHash, in.Password)
	if err != nil {
		return nil, wrapErr(err, 500, "internal_error", "Stored credentials are unreadable.")
	}
	if !ok {
		s.loginLimiter.RegisterFailure(limitKey)
		s.auditEvent(ctx, admin.ID, models.AuditLoginFailed, "bad_password", nil)
		return nil, errInvalidCredentials()
	}
	s.loginLimiter.Reset(limitKey)

	// Opportunistically upgrade hashes produced with weaker parameters.
	if security.NeedsRehash(admin.PasswordHash) {
		if hash, err := security.HashPassword(in.Password); err == nil {
			if err := s.admins.UpdatePassword(ctx, admin.ID, hash, s.now()); err != nil {
				log.Printf("⚠️  auth: password rehash failed for admin %s: %v", admin.ID, err)
			}
		}
	}

	if _, err := s.emailTwoFAConfig(ctx, admin); err != nil {
		return nil, err
	}

	challenge, err := s.issueOTP(ctx, admin, models.OTPPurposeLogin2FA, meta)
	if err != nil {
		return nil, err
	}
	s.auditEvent(ctx, admin.ID, models.AuditLoginChallenged, "email_otp", nil)
	return &LoginResult{TwoFactorRequired: true, Challenge: challenge}, nil
}

// VerifyLoginOTP completes the second factor and starts a session.
func (s *AuthService) VerifyLoginOTP(ctx context.Context, in VerifyOTPInput, meta RequestMeta) (*LoginResult, error) {
	challenge, err := s.loadChallenge(ctx, in.ChallengeID, models.OTPPurposeLogin2FA)
	if err != nil {
		return nil, err
	}

	admin, err := s.checkOTP(ctx, challenge, in.OTP)
	if err != nil {
		return nil, err
	}

	if err := s.admins.TouchLastLogin(ctx, admin.ID, s.now()); err != nil {
		log.Printf("⚠️  auth: could not record last login for admin %s: %v", admin.ID, err)
	}

	tokens, err := s.issueSession(ctx, admin, meta)
	if err != nil {
		return nil, err
	}
	s.auditEvent(ctx, admin.ID, models.AuditLoginSucceeded, "email_otp", nil)
	return &LoginResult{Tokens: tokens, Admin: adminView(admin)}, nil
}

// ResendLoginOTP invalidates the previous code and mails a fresh one, subject
// to the resend cooldown so the endpoint cannot be used to spam the admin.
func (s *AuthService) ResendLoginOTP(ctx context.Context, in ResendOTPInput, meta RequestMeta) (*ChallengeView, error) {
	challenge, err := s.otps.FindByID(ctx, in.ChallengeID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errInvalidChallenge()
		}
		return nil, err
	}
	if challenge.Purpose != models.OTPPurposeLogin2FA || challenge.IsConsumed() {
		return nil, errChallengeUsed()
	}

	admin, err := s.adminFor(ctx, challenge.AdminID)
	if err != nil {
		return nil, err
	}
	if !admin.IsActive {
		return nil, errAccountDisabled()
	}

	if pending, err := s.otps.FindLatestPending(ctx, admin.ID, models.OTPPurposeLogin2FA, s.now()); err == nil {
		availableAt := pending.CreatedAt.Add(s.cfg.OTPResendCooldown)
		if s.now().Before(availableAt) {
			return nil, errOTPCooldown(int(availableAt.Sub(s.now()).Seconds()) + 1)
		}
	} else if !errors.Is(err, models.ErrNotFound) {
		return nil, err
	}

	view, err := s.issueOTP(ctx, admin, models.OTPPurposeLogin2FA, meta)
	if err != nil {
		return nil, err
	}
	s.auditEvent(ctx, admin.ID, models.AuditOTPResent, "email_otp", nil)
	return view, nil
}
