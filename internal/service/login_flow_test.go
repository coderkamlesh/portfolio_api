package service

import (
	"context"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

func TestLoginChallengesWithEmailOTPAndVerifyIssuesSession(t *testing.T) {
	h := newHarness(t)

	result := h.login(t)
	if !result.TwoFactorRequired {
		t.Fatal("expected two_factor_required=true when AUTH_2FA_REQUIRED is on")
	}
	if result.Tokens != nil {
		t.Fatal("no tokens may be issued before the OTP is verified")
	}
	if result.Challenge == nil || result.Challenge.ID == "" {
		t.Fatalf("challenge missing: %+v", result)
	}
	if result.Challenge.Purpose != models.OTPPurposeLogin2FA {
		t.Errorf("purpose = %q, want %q", result.Challenge.Purpose, models.OTPPurposeLogin2FA)
	}
	if result.Challenge.Email == testAdminEmail {
		t.Error("the challenge response must expose the masked email only")
	}
	if result.Challenge.CodeLength != h.cfg.OTPLength {
		t.Errorf("code_length = %d, want %d", result.Challenge.CodeLength, h.cfg.OTPLength)
	}
	if h.mailer.count() != 1 {
		t.Fatalf("mails sent = %d, want 1", h.mailer.count())
	}

	// The 2FA row is auto-provisioned because email OTP needs no enrollment.
	if len(h.twoFA.rows) != 1 || !h.twoFA.rows[0].IsEnabled {
		t.Fatalf("expected an enabled admin_2fa row, got %+v", h.twoFA.rows)
	}

	verified, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: result.Challenge.ID,
		OTP:         h.mailer.lastCode(t),
	}, h.meta())
	if err != nil {
		t.Fatalf("VerifyLoginOTP: %v", err)
	}
	if verified.Tokens == nil {
		t.Fatal("expected a token pair after a correct OTP")
	}
	if verified.Tokens.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want Bearer", verified.Tokens.TokenType)
	}
	if verified.Admin == nil || verified.Admin.ID != testAdminID {
		t.Fatalf("unexpected admin payload: %+v", verified.Admin)
	}

	// The access token must be a valid JWT for the admin.
	manager := security.NewTokenManager(testJWTSecret, h.cfg.JWTIssuer, h.cfg.AccessTokenTTL)
	claims, err := manager.ParseAccessToken(verified.Tokens.AccessToken)
	if err != nil {
		t.Fatalf("issued access token is not parseable: %v", err)
	}
	if claims.Subject != testAdminID {
		t.Errorf("token subject = %q, want %q", claims.Subject, testAdminID)
	}

	// The refresh token is stored hashed, never in clear text.
	if h.refresh.storedCount() != 1 {
		t.Fatalf("stored refresh tokens = %d, want 1", h.refresh.storedCount())
	}
	stored := h.refresh.rows[0]
	if stored.TokenHash == verified.Tokens.RefreshToken {
		t.Fatal("the refresh token was stored in clear text")
	}
	if stored.TokenHash != security.HashToken(verified.Tokens.RefreshToken) {
		t.Fatal("stored hash does not match the issued refresh token")
	}
	if stored.UserAgent != "unit-test/1.0" || stored.IPAddress != "203.0.113.7" {
		t.Errorf("session metadata missing: %+v", stored)
	}
	if h.admin.LastLoginAt == nil || !h.admin.LastLoginAt.Equal(h.now) {
		t.Errorf("last_login_at = %v, want %v", h.admin.LastLoginAt, h.now)
	}
	if !h.audit.hasAction(models.AuditLoginChallenged) || !h.audit.hasAction(models.AuditLoginSucceeded) {
		t.Error("expected LOGIN_CHALLENGED and LOGIN_SUCCEEDED audit rows")
	}
}

func TestVerifyLoginOTPIsOneShot(t *testing.T) {
	h := newHarness(t)
	challenge := h.login(t)
	code := h.mailer.lastCode(t)

	if _, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: challenge.Challenge.ID, OTP: code,
	}, h.meta()); err != nil {
		t.Fatalf("first verify failed: %v", err)
	}

	_, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: challenge.Challenge.ID, OTP: code,
	}, h.meta())
	if got := errorCode(err); got != CodeChallengeUsed {
		t.Fatalf("replaying a used code returned %q, want %q", got, CodeChallengeUsed)
	}
}

func TestVerifyLoginOTPRejectsWrongCodeAndCountsAttempts(t *testing.T) {
	h := newHarness(t)
	challenge := h.login(t)

	_, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: challenge.Challenge.ID, OTP: "000000",
	}, h.meta())
	if got := errorCode(err); got != CodeInvalidOTP {
		t.Fatalf("wrong code returned %q, want %q", got, CodeInvalidOTP)
	}

	stored, err := h.otps.FindByID(context.Background(), challenge.Challenge.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if stored.AttemptCount != 1 {
		t.Errorf("attempt_count = %d, want 1", stored.AttemptCount)
	}
	if !h.audit.hasAction(models.AuditOTPFailed) {
		t.Error("expected an OTP_FAILED audit row")
	}
}

func TestVerifyLoginOTPLocksChallengeAfterMaxAttempts(t *testing.T) {
	h := newHarness(t)
	challenge := h.login(t)
	code := h.mailer.lastCode(t)

	for i := 0; i < h.cfg.OTPMaxAttempts; i++ {
		_, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
			ChallengeID: challenge.Challenge.ID, OTP: "000000",
		}, h.meta())

		last := i == h.cfg.OTPMaxAttempts-1
		want := CodeInvalidOTP
		if last {
			want = CodeOTPAttempts
		}
		if got := errorCode(err); got != want {
			t.Fatalf("attempt %d returned %q, want %q", i+1, got, want)
		}
	}

	// Even the correct code is refused once the challenge is locked.
	_, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: challenge.Challenge.ID, OTP: code,
	}, h.meta())
	if got := errorCode(err); got != CodeChallengeUsed {
		t.Fatalf("locked challenge returned %q, want %q", got, CodeChallengeUsed)
	}
}

func TestVerifyLoginOTPRejectsExpiredChallenge(t *testing.T) {
	h := newHarness(t)
	challenge := h.login(t)
	code := h.mailer.lastCode(t)

	h.advance(h.cfg.OTPTTL + time.Minute)

	_, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: challenge.Challenge.ID, OTP: code,
	}, h.meta())
	if got := errorCode(err); got != CodeChallengeExpired {
		t.Fatalf("expired challenge returned %q, want %q", got, CodeChallengeExpired)
	}
}

func TestVerifyLoginOTPRejectsUnknownChallenge(t *testing.T) {
	h := newHarness(t)

	_, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: "does-not-exist", OTP: "123456",
	}, h.meta())
	if got := errorCode(err); got != CodeInvalidChallenge {
		t.Fatalf("unknown challenge returned %q, want %q", got, CodeInvalidChallenge)
	}
}

func TestVerifyLoginOTPRejectsPasswordResetChallenge(t *testing.T) {
	h := newHarness(t)

	if err := h.svc.ForgotPassword(context.Background(), ForgotPasswordInput{Email: testAdminEmail}, h.meta()); err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}
	code := h.mailer.lastCode(t)

	var resetChallenge *models.OTPChallenge
	for _, r := range h.otps.rows {
		if r.Purpose == models.OTPPurposePasswordReset {
			resetChallenge = r
		}
	}
	if resetChallenge == nil {
		t.Fatal("no PASSWORD_RESET challenge was created")
	}

	_, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: resetChallenge.ID, OTP: code,
	}, h.meta())
	if got := errorCode(err); got != CodeInvalidChallenge {
		t.Fatalf("cross-purpose verification returned %q, want %q", got, CodeInvalidChallenge)
	}
}
