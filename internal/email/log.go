package email

import (
	"context"
	"log"
	"strings"
)

// LogSender writes mails to the process log instead of sending them. It exists
// so the OTP flow can be exercised locally without SES credentials — it must
// never be enabled in production (EMAIL_PROVIDER=log).
type LogSender struct{}

// NewLogSender builds the development sender.
func NewLogSender() *LogSender { return &LogSender{} }

// Send logs the message and always succeeds.
func (s *LogSender) Send(_ context.Context, msg Message) error {
	log.Printf("📧 [EMAIL_PROVIDER=log] to=%s subject=%q\n%s",
		msg.To, msg.Subject, strings.TrimSpace(msg.TextBody))
	return nil
}
