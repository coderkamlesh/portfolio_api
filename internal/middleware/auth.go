// Package middleware holds the cross-cutting HTTP concerns: bearer-token
// authentication, CORS for the admin panel, request logging and cache headers.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

type contextKey string

const claimsContextKey contextKey = "auth.access_claims"

// RequireAuth validates the `Authorization: Bearer <jwt>` header and stores the
// access-token claims in the request context.
func RequireAuth(tokens *security.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r)
			if raw == "" {
				writeAuthError(w, apierr.Unauthorized("missing_token", "Authorization header with a bearer token is required."))
				return
			}

			claims, err := tokens.ParseAccessToken(raw)
			if err != nil {
				writeAuthError(w, apierr.Wrap(err, http.StatusUnauthorized, "invalid_token",
					"Your session is invalid or has expired. Please sign in again."))
				return
			}

			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext returns the validated access claims of the current request.
func ClaimsFromContext(ctx context.Context) (*security.AccessClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*security.AccessClaims)
	return claims, ok
}

// AdminIDFromContext returns the subject (admin_users.id) of the current token.
func AdminIDFromContext(ctx context.Context) (string, bool) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok || claims.Subject == "" {
		return "", false
	}
	return claims.Subject, true
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// writeAuthError renders the shared JSON error envelope without importing the
// handler package (which would create a cycle).
func writeAuthError(w http.ResponseWriter, err error) {
	apiErr := apierr.From(err)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if apiErr.Status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="portfolio-api"`)
	}
	w.WriteHeader(apiErr.Status)
	_, _ = w.Write([]byte(`{"error":{"code":"` + apiErr.Code + `","message":` + quote(apiErr.Message) + `}}`))
}

func quote(v string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(v) + `"`
}
