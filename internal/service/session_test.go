package service

import (
	"context"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

func TestRefreshRotatesTokensAndRevokesTheOldOne(t *testing.T) {
	h := newHarness(t)
	session := h.loginWithOTP(t)

	rotated, err := h.svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: session.Tokens.RefreshToken,
	}, h.meta())
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if rotated.Tokens.RefreshToken == session.Tokens.RefreshToken {
		t.Fatal("the refresh token was not rotated")
	}
	if rotated.Tokens.AccessToken == session.Tokens.AccessToken {
		t.Fatal("the access token was not rotated")
	}
	if h.refresh.activeCount() != 1 {
		t.Fatalf("active refresh tokens = %d, want 1", h.refresh.activeCount())
	}
	if !h.audit.hasAction(models.AuditTokenRefreshed) {
		t.Error("expected a TOKEN_REFRESHED audit row")
	}
}

func TestRefreshDetectsTokenReuseAndKillsEverySession(t *testing.T) {
	h := newHarness(t)
	session := h.loginWithOTP(t)
	stolen := session.Tokens.RefreshToken

	if _, err := h.svc.Refresh(context.Background(), RefreshInput{RefreshToken: stolen}, h.meta()); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	// Replaying the rotated-away token means it leaked.
	_, err := h.svc.Refresh(context.Background(), RefreshInput{RefreshToken: stolen}, h.meta())
	if got := errorCode(err); got != CodeRefreshTokenReused {
		t.Fatalf("reuse returned %q, want %q", got, CodeRefreshTokenReused)
	}
	if h.refresh.activeCount() != 0 {
		t.Fatalf("active refresh tokens = %d, want 0 after reuse detection", h.refresh.activeCount())
	}
	if !h.audit.hasAction(models.AuditTokenReuseDetected) {
		t.Error("expected a TOKEN_REUSE_DETECTED audit row")
	}
}

func TestRefreshRejectsUnknownAndExpiredTokens(t *testing.T) {
	h := newHarness(t)

	_, err := h.svc.Refresh(context.Background(), RefreshInput{RefreshToken: "never-issued"}, h.meta())
	if got := errorCode(err); got != CodeInvalidRefreshToken {
		t.Fatalf("unknown token returned %q, want %q", got, CodeInvalidRefreshToken)
	}

	session := h.loginWithOTP(t)
	h.advance(h.cfg.RefreshTokenTTL + time.Hour)

	_, err = h.svc.Refresh(context.Background(), RefreshInput{RefreshToken: session.Tokens.RefreshToken}, h.meta())
	if got := errorCode(err); got != CodeInvalidRefreshToken {
		t.Fatalf("expired token returned %q, want %q", got, CodeInvalidRefreshToken)
	}
	if h.refresh.activeCount() != 0 {
		t.Fatalf("expired tokens must be revoked, active = %d", h.refresh.activeCount())
	}
}

func TestRefreshRejectsDisabledAdmin(t *testing.T) {
	h := newHarness(t)
	session := h.loginWithOTP(t)
	h.admin.IsActive = false

	_, err := h.svc.Refresh(context.Background(), RefreshInput{RefreshToken: session.Tokens.RefreshToken}, h.meta())
	if got := errorCode(err); got != CodeAccountDisabled {
		t.Fatalf("disabled admin refresh returned %q, want %q", got, CodeAccountDisabled)
	}
	if h.refresh.activeCount() != 0 {
		t.Error("sessions of a disabled admin must be revoked")
	}
}

func TestLogoutRevokesOneSessionOrAll(t *testing.T) {
	h := newHarness(t)

	first := h.loginWithOTP(t)
	second := h.loginWithOTP(t)
	if h.refresh.activeCount() != 2 {
		t.Fatalf("expected 2 sessions, got %d", h.refresh.activeCount())
	}

	if err := h.svc.Logout(context.Background(), testAdminID, LogoutInput{
		RefreshToken: first.Tokens.RefreshToken,
	}); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if h.refresh.activeCount() != 1 {
		t.Fatalf("active sessions = %d, want 1", h.refresh.activeCount())
	}

	// Second call without a token logs out every remaining device.
	if err := h.svc.Logout(context.Background(), testAdminID, LogoutInput{}); err != nil {
		t.Fatalf("Logout(all): %v", err)
	}
	if h.refresh.activeCount() != 0 {
		t.Fatalf("active sessions = %d, want 0", h.refresh.activeCount())
	}
	_ = second
	if !h.audit.hasAction(models.AuditLoggedOut) {
		t.Error("expected a LOGOUT audit row")
	}
}

func TestLogoutIsIdempotentForUnknownTokens(t *testing.T) {
	h := newHarness(t)
	h.loginWithOTP(t)

	// An unknown token is not an error: the desired end state (no session for
	// that token) already holds, and erroring would leak token existence.
	err := h.svc.Logout(context.Background(), testAdminID, LogoutInput{RefreshToken: "someone-elses-token"})
	if err != nil {
		t.Fatalf("logout with unknown token should be idempotent: %v", err)
	}
	if h.refresh.activeCount() != 1 {
		t.Error("a foreign token must not revoke the local session")
	}
}

func TestCurrentAdminReportsTwoFactorAndSessions(t *testing.T) {
	h := newHarness(t)
	h.loginWithOTP(t)

	me, err := h.svc.CurrentAdmin(context.Background(), testAdminID)
	if err != nil {
		t.Fatalf("CurrentAdmin: %v", err)
	}
	if me.Admin == nil || me.Admin.Username != testAdminUser {
		t.Fatalf("unexpected admin: %+v", me.Admin)
	}
	if me.TwoFactor == nil || !me.TwoFactor.Required {
		t.Fatalf("unexpected 2FA status: %+v", me.TwoFactor)
	}
	if me.TwoFactor.Method != models.TwoFAMethodEmailOTP {
		t.Errorf("method = %q, want %q", me.TwoFactor.Method, models.TwoFAMethodEmailOTP)
	}
	if me.TwoFactor.Email == testAdminEmail {
		t.Error("2FA status must expose the masked email only")
	}
	if me.ActiveSession != 1 {
		t.Errorf("active_sessions = %d, want 1", me.ActiveSession)
	}
}
