package service

import (
	"context"
	"testing"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

func TestDisabledEmailOTPIsReEnabledOnLogin(t *testing.T) {
	h := newHarness(t)

	// A stale disabled row cannot bypass the mandatory second factor.
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
		t.Error("the 2FA row must be re-enabled by the mandatory policy")
	}
	if !h.audit.hasAction(models.AuditTwoFAEnabled) {
		t.Error("expected a TWO_FA_ENABLED audit row")
	}
}

func TestTwoFAStatusIsMandatoryWithoutAStoredRow(t *testing.T) {
	h := newHarness(t)

	status, err := h.svc.TwoFAStatusByAdminID(context.Background(), testAdminID)
	if err != nil {
		t.Fatalf("TwoFAStatusByAdminID: %v", err)
	}
	if !status.Enabled || !status.Required {
		t.Fatalf("2FA must always report enabled and required: %+v", status)
	}
	if status.Method != models.TwoFAMethodEmailOTP {
		t.Errorf("method = %q, want %q", status.Method, models.TwoFAMethodEmailOTP)
	}
	if status.ConfirmedAt != nil || status.UpdatedAt != nil {
		t.Errorf("no timestamps should exist before auto-provisioning: %+v", status)
	}
}

func TestEnableEmailOTPWithPasswordConfirmation(t *testing.T) {
	h := newHarness(t)

	if _, err := h.svc.EnableEmailOTP(context.Background(), testAdminID,
		PasswordConfirmationInput{Password: "not-my-password"}); errorCode(err) != CodeInvalidPassword {
		t.Fatalf("wrong password returned %q, want %q", errorCode(err), CodeInvalidPassword)
	}

	status, err := h.svc.EnableEmailOTP(context.Background(), testAdminID,
		PasswordConfirmationInput{Password: testPassword})
	if err != nil {
		t.Fatalf("EnableEmailOTP: %v", err)
	}
	if !status.Enabled || !status.Required || status.ConfirmedAt == nil {
		t.Fatalf("unexpected mandatory 2FA status: %+v", status)
	}
	if !h.audit.hasAction(models.AuditTwoFAEnabled) {
		t.Error("expected a TWO_FA_ENABLED audit row")
	}
}
