package service

import (
	"context"
	"testing"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// Email OTP is the only second factor and it has no off switch, so the status
// is derived from the admin row alone — never from stored configuration.
func TestTwoFAStatusIsMandatory(t *testing.T) {
	h := newHarness(t)

	status, err := h.svc.TwoFAStatusByAdminID(context.Background(), testAdminID)
	if err != nil {
		t.Fatalf("TwoFAStatusByAdminID: %v", err)
	}
	if !status.Required {
		t.Fatalf("2FA must always be required: %+v", status)
	}
	if status.Method != models.TwoFAMethodEmailOTP {
		t.Errorf("method = %q, want %q", status.Method, models.TwoFAMethodEmailOTP)
	}
	if status.Email == testAdminEmail {
		t.Error("2FA status must expose the masked email only")
	}
}

// Logging in always produces an OTP challenge, so there is no code path that
// can issue a session without the second factor.
func TestLoginAlwaysChallenges(t *testing.T) {
	h := newHarness(t)

	result := h.login(t)
	if !result.TwoFactorRequired {
		t.Fatal("login must always require the second factor")
	}
	if result.Challenge == nil {
		t.Fatal("login must always return an OTP challenge")
	}
	if result.Tokens != nil {
		t.Fatal("no tokens may be issued before the OTP is verified")
	}
}
