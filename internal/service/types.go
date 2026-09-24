// Package service implements the admin authentication use-cases on top of the
// repositories: password login, email-OTP second factor, session refresh,
// password management and mandatory email 2FA status.
package service

import (
	"time"
)

// RequestMeta carries request-scoped context stored with sessions and audit
// rows.
type RequestMeta struct {
	IPAddress string
	UserAgent string
}

// LoginInput is the first step of sign-in.
type LoginInput struct {
	Identifier string
	Password   string
}

// VerifyOTPInput completes the email-OTP challenge.
type VerifyOTPInput struct {
	ChallengeID string
	OTP         string
}

// ResendOTPInput asks for a new code for an existing challenge.
type ResendOTPInput struct {
	ChallengeID string
}

// RefreshInput exchanges a refresh token for a new token pair.
type RefreshInput struct {
	RefreshToken string
}

// LogoutInput optionally revokes a single session instead of all of them.
type LogoutInput struct {
	RefreshToken string
	AllDevices   bool
}

// ChangePasswordInput rotates the password of the signed-in admin.
type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

// ForgotPasswordInput starts the reset flow. The response never reveals
// whether the address exists.
type ForgotPasswordInput struct {
	Email string
}

// ResetPasswordInput completes the reset flow with the emailed code.
type ResetPasswordInput struct {
	ChallengeID string
	OTP         string
	NewPassword string
}

// PasswordConfirmationInput is used by the sensitive 2FA confirmation endpoint.
type PasswordConfirmationInput struct {
	Password string
}

// AdminView is the safe projection of an admin account for API responses.
type AdminView struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	IsActive    bool       `json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// ChallengeView describes a pending OTP challenge so the client can render the
// OTP screen without knowing the code.
type ChallengeView struct {
	ID           string    `json:"id"`
	Purpose      string    `json:"purpose"`
	Email        string    `json:"email"` // masked
	CodeLength   int       `json:"code_length"`
	ExpiresAt    time.Time `json:"expires_at"`
	ExpiresIn    int       `json:"expires_in"`
	AttemptsLeft int       `json:"attempts_left"`
	ResendAfter  int       `json:"resend_after"`
}

// TokenPair is what a client stores to access the API.
type TokenPair struct {
	TokenType        string    `json:"token_type"`
	AccessToken      string    `json:"access_token"`
	ExpiresIn        int       `json:"expires_in"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresIn int       `json:"refresh_expires_in"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

// LoginResult is the response of /login and /2fa/verify. The password step
// always returns a pending challenge; OTP verification returns the session.
type LoginResult struct {
	TwoFactorRequired bool           `json:"two_factor_required"`
	Challenge         *ChallengeView `json:"challenge,omitempty"`
	Tokens            *TokenPair     `json:"tokens,omitempty"`
	Admin             *AdminView     `json:"admin,omitempty"`
}

// TwoFAStatus tells the admin panel which second factor is active.
type TwoFAStatus struct {
	Required    bool       `json:"required"`
	Method      string     `json:"method"`
	Enabled     bool       `json:"enabled"`
	Email       string     `json:"email"` // masked
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// MeView is the payload of GET /api/auth/me.
type MeView struct {
	Admin         *AdminView   `json:"admin"`
	TwoFactor     *TwoFAStatus `json:"two_factor"`
	ActiveSession int          `json:"active_sessions"`
}
