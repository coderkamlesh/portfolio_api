// Package apierr defines the single error type the service layer returns and
// the HTTP layer understands. Using one type keeps the mapping from business
// failure to status code + machine-readable code in exactly one place.
package apierr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is an API-visible error: an HTTP status, a stable machine-readable
// code the frontend can switch on, and a human message.
type Error struct {
	Status  int
	Code    string
	Message string
	Err     error
	// RetryAfter (seconds) is emitted as the Retry-After header when > 0.
	RetryAfter int
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// New builds an error without an underlying cause.
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Wrap builds an error that keeps the cause for logs while exposing only the
// curated message to clients.
func Wrap(err error, status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message, Err: err}
}

// From extracts the API error from err, or returns nil when err is nil.
// Non-API errors become a generic 500 so internal details never leak.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return Wrap(err, http.StatusInternalServerError, "internal_error", "Something went wrong. Please try again.")
}

// BadRequest is a helper for handler-level payload validation.
func BadRequest(code, message string) *Error {
	return New(http.StatusBadRequest, code, message)
}

// Unauthorized is a helper for missing/invalid credentials.
func Unauthorized(code, message string) *Error {
	return New(http.StatusUnauthorized, code, message)
}
