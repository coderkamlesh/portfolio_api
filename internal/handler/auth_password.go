package handler

import (
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/middleware"
	"github.com/coderkamlesh/portfolio_api/internal/service"
)

// ChangePassword handles POST /api/auth/password/change and returns a fresh
// token pair for the current device (all other sessions are revoked).
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.AdminIDFromContext(r.Context())
	if !ok {
		writeErr(w, r, unauthorizedContextError())
		return
	}

	var body changePasswordRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.CurrentPassword == "" {
		writeErr(w, r, requiredFieldError("current_password"))
		return
	}
	if body.NewPassword == "" {
		writeErr(w, r, requiredFieldError("new_password"))
		return
	}

	tokens, err := h.svc.ChangePassword(r.Context(), adminID, service.ChangePasswordInput{
		CurrentPassword: body.CurrentPassword,
		NewPassword:     body.NewPassword,
	}, requestMeta(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

// ForgotPassword handles POST /api/auth/password/forgot.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body forgotPasswordRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Email, "email"); err != nil {
		writeErr(w, r, err)
		return
	}

	challenge, err := h.svc.ForgotPassword(r.Context(), service.ForgotPasswordInput{Email: body.Email}, requestMeta(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"message":   "If that email belongs to an admin account, a reset code is on its way.",
		"challenge": challenge,
	})
}

// ResetPassword handles POST /api/auth/password/reset.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body resetPasswordRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.ChallengeID, "challenge_id"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.OTP, "otp"); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.NewPassword == "" {
		writeErr(w, r, requiredFieldError("new_password"))
		return
	}

	result, err := h.svc.ResetPassword(r.Context(), service.ResetPasswordInput{
		ChallengeID: body.ChallengeID,
		OTP:         body.OTP,
		NewPassword: body.NewPassword,
	}, requestMeta(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// TwoFAStatus handles GET /api/auth/2fa.
func (h *AuthHandler) TwoFAStatus(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.AdminIDFromContext(r.Context())
	if !ok {
		writeErr(w, r, unauthorizedContextError())
		return
	}

	status, err := h.svc.TwoFAStatusByAdminID(r.Context(), adminID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// EnableEmail2FA handles POST /api/auth/2fa/email/enable.
func (h *AuthHandler) EnableEmail2FA(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.AdminIDFromContext(r.Context())
	if !ok {
		writeErr(w, r, unauthorizedContextError())
		return
	}

	var body confirmPasswordRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Password == "" {
		writeErr(w, r, requiredFieldError("password"))
		return
	}

	status, err := h.svc.EnableEmailOTP(r.Context(), adminID, service.PasswordConfirmationInput{Password: body.Password})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

