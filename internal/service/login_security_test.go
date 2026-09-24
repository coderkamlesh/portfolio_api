package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

func TestLoginRejectsWrongPassword(t *testing.T) {
	h := newHarness(t)

	_, err := h.svc.Login(context.Background(), LoginInput{
		Identifier: testAdminUser,
		Password:   "definitely-not-the-password-1",
	}, h.meta())
	if got := errorCode(err); got != CodeInvalidCredentials {
		t.Fatalf("wrong password returned %q, want %q", got, CodeInvalidCredentials)
	}
	if h.mailer.count() != 0 {
		t.Error("no mail may be sent for a failed password step")
	}
	if !h.audit.hasAction(models.AuditLoginFailed) {
		t.Error("expected a LOGIN_FAILED audit row")
	}
}

func TestLoginDoesNotRevealUnknownIdentifier(t *testing.T) {
	h := newHarness(t)

	_, err := h.svc.Login(context.Background(), LoginInput{
		Identifier: "ghost@example.com",
		Password:   testPassword,
	}, h.meta())
	if got := errorCode(err); got != CodeInvalidCredentials {
		t.Fatalf("unknown identifier returned %q, want %q (no enumeration)", got, CodeInvalidCredentials)
	}
}

func TestLoginRejectsDisabledAccount(t *testing.T) {
	h := newHarness(t)
	h.admin.IsActive = false

	_, err := h.svc.Login(context.Background(), LoginInput{
		Identifier: testAdminEmail,
		Password:   testPassword,
	}, h.meta())
	if got := errorCode(err); got != CodeAccountDisabled {
		t.Fatalf("disabled account returned %q, want %q", got, CodeAccountDisabled)
	}
}

func TestLoginIsRateLimitedAfterMaxFailures(t *testing.T) {
	h := newHarness(t)

	for i := 0; i < h.cfg.LoginMaxAttempts; i++ {
		_, err := h.svc.Login(context.Background(), LoginInput{
			Identifier: testAdminUser,
			Password:   "wrong-password-attempt",
		}, h.meta())
		if got := errorCode(err); got != CodeInvalidCredentials {
			t.Fatalf("attempt %d returned %q, want %q", i+1, got, CodeInvalidCredentials)
		}
	}

	// Even the correct password is refused while the limiter is hot.
	_, err := h.svc.Login(context.Background(), LoginInput{
		Identifier: testAdminUser,
		Password:   testPassword,
	}, h.meta())
	if got := errorCode(err); got != CodeRateLimited {
		t.Fatalf("rate limited login returned %q, want %q", got, CodeRateLimited)
	}
	if !h.audit.hasAction(models.AuditRateLimited) {
		t.Error("expected a RATE_LIMITED audit row")
	}
}

func TestLoginUsesEmailCaseInsensitively(t *testing.T) {
	h := newHarness(t)

	if _, err := h.svc.Login(context.Background(), LoginInput{
		Identifier: "KAMLESH@EXAMPLE.COM",
		Password:   testPassword,
	}, h.meta()); err != nil {
		t.Fatalf("email login should be case-insensitive: %v", err)
	}
}

func TestLoginSkipsOTPWhenNotRequired(t *testing.T) {
	h := newHarness(t)
	h.cfg.TwoFARequired = false

	result := h.login(t)
	if result.TwoFactorRequired || result.Challenge != nil {
		t.Fatalf("expected no 2FA challenge, got %+v", result)
	}
	if result.Tokens == nil {
		t.Fatal("expected a token pair when 2FA is off")
	}
	if h.mailer.count() != 0 {
		t.Error("no OTP mail may be sent when 2FA is off")
	}
	if len(h.twoFA.rows) != 0 {
		t.Errorf("2FA row must not be auto-provisioned when not required: %+v", h.twoFA.rows)
	}
}

func TestLoginSurfacesMailFailuresAndBurnsTheChallenge(t *testing.T) {
	h := newHarness(t)
	h.mailer.err = errors.New("ses: message rejected")

	_, err := h.svc.Login(context.Background(), LoginInput{
		Identifier: testAdminUser,
		Password:   testPassword,
	}, h.meta())
	if got := errorCode(err); got != CodeEmailDeliveryFailed {
		t.Fatalf("mail failure returned %q, want %q", got, CodeEmailDeliveryFailed)
	}
	// The unusable challenge must be consumed so a leaked id is worthless.
	for _, r := range h.otps.rows {
		if r.Purpose == models.OTPPurposeLogin2FA && r.ConsumedAt == nil {
			t.Fatal("challenge left pending after the mail failed")
		}
	}
}

func TestResendOTPRespectsCooldownAndInvalidatesTheOldCode(t *testing.T) {
	h := newHarness(t)

	first := h.login(t)
	firstCode := h.mailer.lastCode(t)

	_, err := h.svc.ResendLoginOTP(context.Background(), ResendOTPInput{
		ChallengeID: first.Challenge.ID,
	}, h.meta())
	if got := errorCode(err); got != CodeOTPCooldown {
		t.Fatalf("immediate resend returned %q, want %q", got, CodeOTPCooldown)
	}

	h.advance(h.cfg.OTPResendCooldown + time.Second)

	second, err := h.svc.ResendLoginOTP(context.Background(), ResendOTPInput{
		ChallengeID: first.Challenge.ID,
	}, h.meta())
	if err != nil {
		t.Fatalf("ResendLoginOTP: %v", err)
	}
	if second.ID == first.Challenge.ID {
		t.Fatal("resend must create a new challenge id")
	}
	if h.mailer.count() != 2 {
		t.Fatalf("mails sent = %d, want 2", h.mailer.count())
	}

	// The previous code is dead once a newer one is issued.
	_, err = h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: first.Challenge.ID, OTP: firstCode,
	}, h.meta())
	if got := errorCode(err); got != CodeChallengeUsed {
		t.Fatalf("old challenge returned %q, want %q", got, CodeChallengeUsed)
	}

	if _, err := h.svc.VerifyLoginOTP(context.Background(), VerifyOTPInput{
		ChallengeID: second.ID, OTP: h.mailer.lastCode(t),
	}, h.meta()); err != nil {
		t.Fatalf("new code must work: %v", err)
	}
	if !h.audit.hasAction(models.AuditOTPResent) {
		t.Error("expected an OTP_RESENT audit row")
	}
}

func TestResendOTPIsCappedPerWindow(t *testing.T) {
	h := newHarness(t)

	challenge := h.login(t)
	issued := 1

	for i := 1; i < h.cfg.OTPMaxPerWindow; i++ {
		h.advance(h.cfg.OTPResendCooldown + time.Second)
		next, err := h.svc.ResendLoginOTP(context.Background(), ResendOTPInput{
			ChallengeID: challenge.Challenge.ID,
		}, h.meta())
		if err != nil {
			t.Fatalf("resend %d failed: %v", i, err)
		}
		challenge = &LoginResult{TwoFactorRequired: true, Challenge: next}
		issued++
	}

	if issued != h.cfg.OTPMaxPerWindow {
		t.Fatalf("issued %d challenges, want %d", issued, h.cfg.OTPMaxPerWindow)
	}

	h.advance(h.cfg.OTPResendCooldown + time.Second)
	_, err := h.svc.ResendLoginOTP(context.Background(), ResendOTPInput{
		ChallengeID: challenge.Challenge.ID,
	}, h.meta())
	if got := errorCode(err); got != CodeOTPTooMany {
		t.Fatalf("window cap returned %q, want %q", got, CodeOTPTooMany)
	}
}
