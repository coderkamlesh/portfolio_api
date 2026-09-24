package email

import (
	"strings"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

func TestOTPMessageForLogin(t *testing.T) {
	msg := OTPMessage("kamlesh@example.com", OTPMail{
		Code:        "482913",
		Purpose:     models.OTPPurposeLogin2FA,
		TTL:         10 * time.Minute,
		RequestedAt: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC),
		IPAddress:   "203.0.113.7",
	})

	if msg.To != "kamlesh@example.com" {
		t.Errorf("to = %q", msg.To)
	}
	if msg.Subject != "Your portfolio admin code: 482913" {
		t.Errorf("subject = %q", msg.Subject)
	}
	for _, body := range []string{msg.TextBody, msg.HTMLBody} {
		if !strings.Contains(body, "482913") {
			t.Error("the body does not contain the code")
		}
		if !strings.Contains(body, "203.0.113.7") {
			t.Error("the body does not show the requesting IP")
		}
	}
	if !strings.Contains(msg.TextBody, "sign in to the admin panel") {
		t.Error("the login mail must explain the action")
	}
	if strings.Contains(msg.TextBody, "reset your password") {
		t.Error("the login mail must not mention a password reset")
	}
	if msg.HTMLBody == "" {
		t.Error("an HTML alternative must be provided")
	}
}

func TestOTPMessageForPasswordReset(t *testing.T) {
	msg := OTPMessage("kamlesh@example.com", OTPMail{
		Code:        "135790",
		Purpose:     models.OTPPurposePasswordReset,
		TTL:         90 * time.Second, // rounds up to 2 minutes
		RequestedAt: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC),
	})

	if !strings.Contains(msg.Subject, "135790") {
		t.Errorf("subject = %q", msg.Subject)
	}
	if !strings.Contains(msg.TextBody, "reset your password") {
		t.Error("the reset mail must explain the action")
	}
	if !strings.Contains(msg.TextBody, "2 minute(s)") {
		t.Error("the TTL must be rendered in minutes")
	}
}

func TestOTPMessageEscapesUntrustedValues(t *testing.T) {
	msg := OTPMessage("kamlesh@example.com", OTPMail{
		Code:        "482913",
		Purpose:     models.OTPPurposeLogin2FA,
		TTL:         time.Minute,
		RequestedAt: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC),
		IPAddress:   `<script>alert(1)</script>`,
	})

	if strings.Contains(msg.HTMLBody, "<script>") {
		t.Error("HTML body must escape the IP address")
	}
	if !strings.Contains(msg.HTMLBody, "&lt;script&gt;") {
		t.Error("expected the escaped IP address in the HTML body")
	}
}

func TestEscapeForLogTrimsAndEscapes(t *testing.T) {
	got := EscapeForLog("  <b>hi</b>  ")
	if got != "&lt;b&gt;hi&lt;/b&gt;" {
		t.Fatalf("EscapeForLog = %q", got)
	}
}
