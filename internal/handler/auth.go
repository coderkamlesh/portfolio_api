package handler

import (
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/middleware"
	"github.com/coderkamlesh/portfolio_api/internal/service"
)

// AuthHandler serves the /api/auth routes.
type AuthHandler struct {
	svc *service.AuthService
}

// NewAuthHandler builds the handler.
func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Login handles POST /api/auth/login.
//
//	challenge returned -> email OTP sent, complete it with /api/auth/2fa/verify
//	tokens returned    -> 2FA not required for this account
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Identifier, "identifier"); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Password == "" {
		writeErr(w, r, requiredFieldError("password"))
		return
	}

	result, err := h.svc.Login(r.Context(), service.LoginInput{
		Identifier: body.Identifier,
		Password:   body.Password,
	}, requestMeta(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Verify2FA handles POST /api/auth/2fa/verify.
func (h *AuthHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	var body verifyOTPRequest
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

	result, err := h.svc.VerifyLoginOTP(r.Context(), service.VerifyOTPInput{
		ChallengeID: body.ChallengeID,
		OTP:         body.OTP,
	}, requestMeta(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Resend2FA handles POST /api/auth/2fa/resend.
func (h *AuthHandler) Resend2FA(w http.ResponseWriter, r *http.Request) {
	var body resendOTPRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.ChallengeID, "challenge_id"); err != nil {
		writeErr(w, r, err)
		return
	}

	challenge, err := h.svc.ResendLoginOTP(r.Context(), service.ResendOTPInput{
		ChallengeID: body.ChallengeID,
	}, requestMeta(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, challenge)
}

// Refresh handles POST /api/auth/refresh (rotates the refresh token).
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.RefreshToken, "refresh_token"); err != nil {
		writeErr(w, r, err)
		return
	}

	result, err := h.svc.Refresh(r.Context(), service.RefreshInput{RefreshToken: body.RefreshToken}, requestMeta(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Logout handles POST /api/auth/logout for the signed-in admin.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.AdminIDFromContext(r.Context())
	if !ok {
		writeErr(w, r, unauthorizedContextError())
		return
	}

	// The body is optional: no body means "log out everywhere".
	var body logoutRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(w, r, &body); err != nil {
			writeErr(w, r, err)
			return
		}
	}

	if err := h.svc.Logout(r.Context(), adminID, service.LogoutInput{
		RefreshToken: body.RefreshToken,
		AllDevices:   body.AllDevices,
	}); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}

// Me handles GET /api/auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.AdminIDFromContext(r.Context())
	if !ok {
		writeErr(w, r, unauthorizedContextError())
		return
	}

	view, err := h.svc.CurrentAdmin(r.Context(), adminID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}
