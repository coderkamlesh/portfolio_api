// Package handler exposes the auth service over HTTP (chi). Handlers only
// decode/validate the request, call the service and encode the response —
// all rules live in the service layer.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/service"
)

// maxBodyBytes caps request bodies — auth payloads are tiny.
const maxBodyBytes = 64 * 1024

// errorEnvelope is the shape of every failed response:
//
//	{"error": {"code": "invalid_credentials", "message": "…"}}
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON serialises payload with the given status code.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("⚠️  handler: encode response failed: %v", err)
	}
}

// writeNoContent answers 204.
func writeNoContent(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// writeErr maps any error to the JSON error envelope. Internal errors are
// logged with their cause but never exposed.
func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	apiErr := apierr.From(err)
	if apiErr.Status >= http.StatusInternalServerError {
		log.Printf("❌ handler: %s %s -> %d %s: %v", r.Method, r.URL.Path, apiErr.Status, apiErr.Code, apiErr.Err)
	}
	if apiErr.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(apiErr.RetryAfter))
	}
	writeJSON(w, apiErr.Status, errorEnvelope{Error: errorBody{Code: apiErr.Code, Message: apiErr.Message}})
}

// decodeJSON reads a single JSON object with strict field checking.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		switch {
		case errors.Is(err, io.EOF):
			return apierr.BadRequest("empty_body", "The request body is required.")
		case strings.Contains(err.Error(), "http: request body too large"):
			return apierr.BadRequest("body_too_large", "The request body is too large.")
		default:
			return apierr.Wrap(err, http.StatusBadRequest, "invalid_json",
				fmt.Sprintf("The request body is not valid JSON: %s", err.Error()))
		}
	}
	if decoder.More() {
		return apierr.BadRequest("invalid_json", "The request body must contain a single JSON object.")
	}
	return nil
}

// requestMeta extracts the caller fingerprint stored with sessions/audit rows.
// net/http + chi's RealIP already resolve X-Forwarded-For into RemoteAddr.
func requestMeta(r *http.Request) service.RequestMeta {
	return service.RequestMeta{
		IPAddress: clientIP(r),
		UserAgent: truncate(r.UserAgent(), 255),
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func truncate(v string, max int) string {
	if len(v) > max {
		return v[:max]
	}
	return v
}

// requiredString trims a field and rejects it when empty. `field` is used in
// the message so the admin panel can show which input is missing.
func requiredString(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", requiredFieldError(field)
	}
	return value, nil
}

// requiredFieldError reports a missing field in the request body.
func requiredFieldError(field string) error {
	return apierr.BadRequest("validation_failed", field+" is required.")
}

// unauthorizedContextError is a defensive guard: it fires only if a protected
// route is ever registered without RequireAuth.
func unauthorizedContextError() error {
	return apierr.Unauthorized("unauthorized", "Authentication required.")
}
