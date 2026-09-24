package email

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/config"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// NewSender picks the mail transport from EMAIL_PROVIDER:
// "ses" (default) uses AWS SES, "log" prints to stdout for local development.
func NewSender(ctx context.Context, cfg *config.Config) (Sender, error) {
	if cfg.UsesSES() {
		return NewSESSender(ctx, cfg)
	}
	return NewLogSender(), nil
}

// OTPMail carries the context rendered into a one-time-code mail.
type OTPMail struct {
	Code        string
	Purpose     string
	TTL         time.Duration
	RequestedAt time.Time
	IPAddress   string
	UserAgent   string
}

// OTPMessage renders the subject and bodies for a login or reset code.
func OTPMessage(to string, data OTPMail) Message {
	// Round up so a 90-second TTL reads as "2 minute(s)", never "1".
	minutes := int((data.TTL + time.Minute - 1) / time.Minute)
	if minutes < 1 {
		minutes = 1
	}

	action := "sign in to the admin panel"
	headline := "Your sign-in code"
	ignore := "If you did not try to sign in, someone may have your password — change it right away."
	if data.Purpose == models.OTPPurposePasswordReset {
		action = "reset your password"
		headline = "Your password reset code"
		ignore = "If you did not request a password reset, you can safely ignore this email."
	}

	when := data.RequestedAt.UTC().Format("02 Jan 2006 15:04 UTC")
	origin := EscapeForLog(data.IPAddress)
	if origin == "" {
		origin = "unknown"
	}

	text := fmt.Sprintf(`%s

Use this code to %s:

%s

It expires in %d minute(s). Never share it with anyone — our team will never ask for it.

Requested: %s
IP address: %s

%s
`, headline, action, data.Code, minutes, when, origin, ignore)

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html><body style="margin:0;padding:24px;background:#f4f5f7;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#1f2933;">
  <div style="max-width:520px;margin:0 auto;background:#ffffff;border-radius:12px;padding:32px;border:1px solid #e4e7eb;">
    <h1 style="margin:0 0 8px;font-size:20px;">%s</h1>
    <p style="margin:0 0 24px;color:#616e7c;">Use this one-time code to continue. It expires in %d minute(s).</p>
    <p style="margin:0 0 24px;font-size:34px;font-weight:700;letter-spacing:8px;color:#0f62fe;">%s</p>
    <p style="margin:0 0 8px;color:#616e7c;font-size:13px;">Requested: %s</p>
    <p style="margin:0 0 24px;color:#616e7c;font-size:13px;">IP address: %s</p>
    <hr style="border:none;border-top:1px solid #e4e7eb;margin:0 0 16px;">
    <p style="margin:0;color:#9aa5b1;font-size:12px;">%s</p>
  </div>
</body></html>`,
		headline, minutes, data.Code, when, origin, ignore)

	return Message{
		To:       to,
		Subject:  fmt.Sprintf("Your portfolio admin code: %s", data.Code),
		TextBody: text,
		HTMLBody: htmlBody,
	}
}

// EscapeForLog is a tiny helper used by tests/handlers to show untrusted
// values (user agent, ip) safely in mail templates.
func EscapeForLog(v string) string {
	return html.EscapeString(strings.TrimSpace(v))
}
