// Package email delivers transactional mail for the admin panel (currently
// the login OTP and password reset codes) through AWS SES or, in local
// development, to the server log.
package email

import "context"

// Message is a plain transactional mail with an optional HTML alternative.
type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// Sender delivers a Message. Implementations must return an error when the
// provider rejects the send so callers can surface a 502 to the client.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
