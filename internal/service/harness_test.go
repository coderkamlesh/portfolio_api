package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/config"
	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

const (
	testPassword   = "correct-horse-battery-1"
	testAdminID    = "22222222-2222-4222-8222-222222222222"
	testAdminEmail = "kamlesh@example.com"
	testAdminUser  = "kamlesh"
	testJWTSecret  = "unit-test-secret-key-that-is-long-enough-1234"
)

// harness bundles the service under test with its fake collaborators and a
// controllable clock.
type harness struct {
	svc     *AuthService
	admins  *fakeAdminStore
	twoFA   *fakeTwoFAStore
	otps    *fakeOTPStore
	refresh *fakeRefreshStore
	audit   *fakeAuditStore
	mailer  *fakeMailer
	cfg     *config.Config
	now     time.Time
	admin   *models.AdminUser
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	hash, err := security.HashPassword(testPassword)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	start := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	h := &harness{
		admins:  newFakeAdminStore(),
		twoFA:   &fakeTwoFAStore{},
		otps:    &fakeOTPStore{},
		refresh: &fakeRefreshStore{},
		audit:   &fakeAuditStore{},
		mailer:  &fakeMailer{},
		now:     start,
		admin: &models.AdminUser{
			ID:           testAdminID,
			Username:     testAdminUser,
			Email:        testAdminEmail,
			PasswordHash: hash,
			IsActive:     true,
			CreatedAt:    start,
			UpdatedAt:    start,
		},
		cfg: &config.Config{
			JWTSecret:         testJWTSecret,
			JWTIssuer:         "portfolio-api",
			AccessTokenTTL:    15 * time.Minute,
			RefreshTokenTTL:   30 * 24 * time.Hour,
			OTPLength:         6,
			OTPTTL:            10 * time.Minute,
			OTPMaxAttempts:    3,
			OTPMaxPerWindow:   5,
			OTPWindow:         time.Hour,
			OTPResendCooldown: 30 * time.Second,
			MinPasswordLength: 12,
			LoginMaxAttempts:  3,
			LoginWindow:       15 * time.Minute,
			EmailProvider:     config.EmailProviderLog,
		},
	}
	h.admins.admins[h.admin.ID] = h.admin

	h.svc = NewAuthService(Deps{
		Admins:  h.admins,
		TwoFA:   h.twoFA,
		OTPs:    h.otps,
		Refresh: h.refresh,
		Audit:   h.audit,
		Mailer:  h.mailer,
		Tokens:  security.NewTokenManager(h.cfg.JWTSecret, h.cfg.JWTIssuer, h.cfg.AccessTokenTTL),
		Cfg:     h.cfg,
		Now:     func() time.Time { return h.now },
	})
	return h
}

func (h *harness) advance(d time.Duration) { h.now = h.now.Add(d) }

func (h *harness) meta() RequestMeta {
	return RequestMeta{IPAddress: "203.0.113.7", UserAgent: "unit-test/1.0"}
}

// login runs the password step with the correct credentials.
func (h *harness) login(t *testing.T) *LoginResult {
	t.Helper()

	result, err := h.svc.Login(context.Background(), LoginInput{
		Identifier: testAdminUser,
		Password:   testPassword,
	}, h.meta())
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	return result
}

// loginWithOTP walks the full two-step flow and returns the signed-in session.
func (h *harness) loginWithOTP(t *testing.T) *LoginResult {
	t.Helper()

	challenge := h.login(t)
	if !challenge.TwoFactorRequired || challenge.Challenge == nil {
		t.Fatalf("expected a 2FA challenge, got %+v", challenge)
	}

	result, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: challenge.Challenge.ID,
		OTP:         h.mailer.lastCode(t),
	}, h.meta())
	if err != nil {
		t.Fatalf("VerifyLoginOTP: %v", err)
	}
	return result
}

// errorCode returns the machine-readable API code of err ("" for nil).
func errorCode(err error) string {
	if err == nil {
		return ""
	}
	return apierr.From(err).Code
}

func lower(v string) string      { return strings.ToLower(strings.TrimSpace(v)) }
func hasPrefix(v, p string) bool { return strings.HasPrefix(v, p) }
