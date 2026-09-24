package service

import (
	"context"
	"testing"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

// resetChallengeID returns the id of the pending PASSWORD_RESET challenge.
func resetChallengeID(t *testing.T, h *harness) string {
	t.Helper()

	for _, r := range h.otps.rows {
		if r.Purpose == models.OTPPurposePasswordReset && r.ConsumedAt == nil {
			return r.ID
		}
	}
	t.Fatal("no pending PASSWORD_RESET challenge found")
	return ""
}

func TestChangePasswordRotatesHashAndKeepsCurrentDeviceSignedIn(t *testing.T) {
	h := newHarness(t)
	session := h.loginWithOTP(t)

	_, err := h.svc.ChangePassword(context.Background(), testAdminID, ChangePasswordInput{
		CurrentPassword: "not-the-current-password",
		NewPassword:     "brand-new-passphrase-42",
	}, h.meta())
	if got := errorCode(err); got != CodeInvalidPassword {
		t.Fatalf("wrong current password returned %q, want %q", got, CodeInvalidPassword)
	}

	oldHash := h.admin.PasswordHash
	tokens, err := h.svc.ChangePassword(context.Background(), testAdminID, ChangePasswordInput{
		CurrentPassword: testPassword,
		NewPassword:     "brand-new-passphrase-42",
	}, h.meta())
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if h.admin.PasswordHash == oldHash {
		t.Fatal("the stored hash was not rotated")
	}

	// Only the fresh session survives; the old one was revoked.
	if h.refresh.activeCount() != 1 {
		t.Fatalf("active sessions = %d, want 1", h.refresh.activeCount())
	}
	var surviving *models.RefreshToken
	for _, r := range h.refresh.rows {
		if r.RevokedAt == nil {
			surviving = r
		}
	}
	if surviving == nil || surviving.TokenHash != security.HashToken(tokens.RefreshToken) {
		t.Fatal("the returned token does not match the surviving session")
	}

	// The old password no longer works, the new one does.
	if _, err := h.svc.Login(context.Background(), LoginInput{
		Identifier: testAdminUser, Password: testPassword,
	}, h.meta()); errorCode(err) != CodeInvalidCredentials {
		t.Fatalf("old password still works (got %q)", errorCode(err))
	}
	if _, err := h.svc.Login(context.Background(), LoginInput{
		Identifier: testAdminUser, Password: "brand-new-passphrase-42",
	}, h.meta()); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}

	_ = session
}

func TestChangePasswordEnforcesPolicy(t *testing.T) {
	h := newHarness(t)
	h.loginWithOTP(t)

	cases := []struct {
		name     string
		password string
	}{
		{name: "too short", password: "aB3!"},
		{name: "letters only", password: "abcdefghijklmnopqrst"},
		{name: "digits only", password: "123456789012345"},
	}

	for _, tc := range cases {
		_, err := h.svc.ChangePassword(context.Background(), testAdminID, ChangePasswordInput{
			CurrentPassword: testPassword,
			NewPassword:     tc.password,
		}, h.meta())
		if got := errorCode(err); got != CodeWeakPassword {
			t.Errorf("%s: returned %q, want %q", tc.name, got, CodeWeakPassword)
		}
	}
}

func TestChangePasswordRejectsReuse(t *testing.T) {
	h := newHarness(t)
	h.loginWithOTP(t)

	_, err := h.svc.ChangePassword(context.Background(), testAdminID, ChangePasswordInput{
		CurrentPassword: testPassword,
		NewPassword:     testPassword,
	}, h.meta())
	if got := errorCode(err); got != CodeSamePassword {
		t.Fatalf("password reuse returned %q, want %q", got, CodeSamePassword)
	}
}

func TestForgotPasswordIsSilentForUnknownAddresses(t *testing.T) {
	h := newHarness(t)

	challenge, err := h.svc.ForgotPassword(context.Background(), ForgotPasswordInput{
		Email: "nobody@example.com",
	}, h.meta())
	if err != nil {
		t.Fatalf("unknown address must not error: %v", err)
	}
	if challenge == nil || challenge.Purpose != models.OTPPurposePasswordReset {
		t.Fatalf("unknown address challenge = %+v, want a password-reset challenge shape", challenge)
	}
	if h.mailer.count() != 0 {
		t.Error("no mail may be sent to an unknown address")
	}
	if _, err := h.svc.ResetPassword(context.Background(), ResetPasswordInput{
		ChallengeID: challenge.ID, OTP: "123456", NewPassword: "reset-passphrase-99",
	}, h.meta()); errorCode(err) != CodeInvalidChallenge {
		t.Fatalf("synthetic challenge returned %q, want %q", errorCode(err), CodeInvalidChallenge)
	}
}

func TestForgotPasswordRateLimitLooksLikeUnknownAddress(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < h.cfg.OTPMaxPerWindow; i++ {
		h.otps.rows = append(h.otps.rows, &models.OTPChallenge{
			ID:        "rate-limit-challenge",
			AdminID:   testAdminID,
			Purpose:   models.OTPPurposePasswordReset,
			CreatedAt: h.now,
			ExpiresAt: h.now.Add(h.cfg.OTPTTL),
		})
	}

	challenge, err := h.svc.ForgotPassword(context.Background(), ForgotPasswordInput{
		Email: testAdminEmail,
	}, h.meta())
	if err != nil {
		t.Fatalf("rate-limited address must not expose an error: %v", err)
	}
	if challenge == nil || challenge.Purpose != models.OTPPurposePasswordReset {
		t.Fatalf("returned challenge = %+v, want password-reset challenge shape", challenge)
	}
	if h.mailer.count() != 0 {
		t.Error("no mail may be sent after the OTP window is exhausted")
	}
}

func TestPasswordResetFlowReplacesPasswordAndSignsIn(t *testing.T) {
	h := newHarness(t)

	challenge, err := h.svc.ForgotPassword(context.Background(), ForgotPasswordInput{
		Email: testAdminEmail,
	}, h.meta())
	if err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}
	if challenge == nil || challenge.ID != resetChallengeID(t, h) {
		t.Fatalf("returned challenge = %+v, want the pending reset challenge", challenge)
	}
	if h.mailer.count() != 1 {
		t.Fatalf("mails sent = %d, want 1", h.mailer.count())
	}
	code := h.mailer.lastCode(t)

	// Wrong code first: it must consume an attempt, not the reset.
	_, err = h.svc.ResetPassword(context.Background(), ResetPasswordInput{
		ChallengeID: challenge.ID, OTP: "000000", NewPassword: "reset-passphrase-99",
	}, h.meta())
	if got := errorCode(err); got != CodeInvalidOTP {
		t.Fatalf("wrong reset code returned %q, want %q", got, CodeInvalidOTP)
	}

	result, err := h.svc.ResetPassword(context.Background(), ResetPasswordInput{
		ChallengeID: challenge.ID, OTP: code, NewPassword: "reset-passphrase-99",
	}, h.meta())
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if result.Tokens == nil {
		t.Fatal("a successful reset must sign the admin in")
	}
	if !h.audit.hasAction(models.AuditPasswordResetAsked) || !h.audit.hasAction(models.AuditPasswordReset) {
		t.Error("expected PASSWORD_RESET_REQUESTED and PASSWORD_RESET audit rows")
	}

	ok, err := security.VerifyPassword(h.admin.PasswordHash, "reset-passphrase-99")
	if err != nil || !ok {
		t.Fatalf("new password not stored correctly: %v %v", ok, err)
	}
	if h.refresh.activeCount() != 1 {
		t.Fatalf("active sessions = %d, want 1 (other sessions revoked)", h.refresh.activeCount())
	}
}
