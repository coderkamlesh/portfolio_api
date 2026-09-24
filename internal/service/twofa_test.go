package service

import (
	"context"
	"testing"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

func TestTwoFAIsReEnabledWhenServerPolicyRequiresIt(t *testing.T) {
	h := newHarness(t)

	// An admin who previously turned 2FA off cannot bypass the policy.
	h.twoFA.rows = append(h.twoFA.rows, &models.TwoFactorConfig{
		ID:        "33333333-3333-4333-8333-333333333333",
		AdminID:   testAdminID,
		Method:    models.TwoFAMethodEmailOTP,
		IsEnabled: false,
		CreatedAt: h.now,
	})

	result := h.login(t)
	if !result.TwoFactorRequired || result.Challenge == nil {
		t.Fatalf("expected a challenge even with a disabled row: %+v", result)
	}
	if !h.twoFA.rows[0].IsEnabled {
		t.Error("the 2FA row must be re-enabled while AUTH_2FA_REQUIRED=true")
	}
	if !h.audit.hasAction(models.AuditTwoFAEnabled) {
		t.Error("expected a TWO_FA_ENABLED audit row")
	}
}

func TestDisableEmailOTPIsRefusedWhenEnforced(t *testing.T) {
	h := newHarness(t)
	h.loginWithOTP(t)

	err := h.svc.DisableEmailOTP(context.Background(), testAdminID,
		PasswordConfirmationInput{Password: testPassword})
	if got := errorCode(err); got != CodeTwoFAEnforced {
		t.Fatalf("disable returned %q, want %q", got, CodeTwoFAEnforced)
	}
	if !h.twoFA.rows[0].IsEnabled {
		t.Error("the 2FA row must stay enabled")
	}
}

func TestDisableEmailOTPRequiresThePassword(t *testing.T) {
	h := newHarness(t)
	h.cfg.TwoFARequired = false

	// Enable it first so there is a row to disable (2FA is optional here).
	if _, err := h.svc.EnableEmailOTP(context.Background(), testAdminID,
		PasswordConfirmationInput{Password: testPassword}); err != nil {
		t.Fatalf("EnableEmailOTP: %v", err)
	}

	err := h.svc.DisableEmailOTP(context.Background(), testAdminID,
		PasswordConfirmationInput{Password: "wrong-password-here"})
	if got := errorCode(err); got != CodeInvalidPassword {
		t.Fatalf("wrong password returned %q, want %q", got, CodeInvalidPassword)
	}

	if err := h.svc.DisableEmailOTP(context.Background(), testAdminID,
		PasswordConfirmationInput{Password: testPassword}); err != nil {
		t.Fatalf("DisableEmailOTP: %v", err)
	}
	if h.twoFA.rows[0].IsEnabled {
		t.Error("the 2FA row should be disabled")
	}
	if !h.audit.hasAction(models.AuditTwoFADisabled) {
		t.Error("expected a TWO_FA_DISABLED audit row")
	}

	// With 2FA off and optional, the next login goes straight to tokens.
	result := h.login(t)
	if result.TwoFactorRequired || result.Challenge != nil || result.Tokens == nil {
		t.Errorf("expected a direct login once 2FA is disabled: %+v", result)
	}
}

func TestEnableEmailOTPWithPasswordConfirmation(t *testing.T) {
	h := newHarness(t)
	h.cfg.TwoFARequired = false

	status, err := h.svc.TwoFAStatusByAdminID(context.Background(), testAdminID)
	if err != nil {
		t.Fatalf("TwoFAStatusByAdminID: %v", err)
	}
	if status.Enabled {
		t.Error("2FA must report as disabled before it is enabled")
	}

	if _, err := h.svc.EnableEmailOTP(context.Background(), testAdminID,
		PasswordConfirmationInput{Password: "not-my-password"}); errorCode(err) != CodeInvalidPassword {
		t.Fatalf("wrong password returned %q, want %q", errorCode(err), CodeInvalidPassword)
	}

	status, err = h.svc.EnableEmailOTP(context.Background(), testAdminID,
		PasswordConfirmationInput{Password: testPassword})
	if err != nil {
		t.Fatalf("EnableEmailOTP: %v", err)
	}
	if !status.Enabled || status.ConfirmedAt == nil {
		t.Fatalf("unexpected status after enabling: %+v", status)
	}
	if status.Required {
		t.Error("Required must reflect AUTH_2FA_REQUIRED, which is false here")
	}
	if !h.audit.hasAction(models.AuditTwoFAEnabled) {
		t.Error("expected a TWO_FA_ENABLED audit row")
	}
}

func TestDisableEmailOTPWithoutRowReportsNotConfigured(t *testing.T) {
	h := newHarness(t)
	h.cfg.TwoFARequired = false

	err := h.svc.DisableEmailOTP(context.Background(), testAdminID,
		PasswordConfirmationInput{Password: testPassword})
	if got := errorCode(err); got != CodeTwoFANotConfigured {
		t.Fatalf("disable without a row returned %q, want %q", got, CodeTwoFANotConfigured)
	}
}
