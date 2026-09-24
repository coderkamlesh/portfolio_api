package security

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
	"github.com/golang-jwt/jwt/v5"
)

// TokenTypeAccess marks a JWT as an access token so a refresh token (which is
// opaque) can never be replayed as a bearer credential.
const TokenTypeAccess = "access"

// RefreshTokenBytes is the entropy of an opaque refresh token.
const RefreshTokenBytes = 32

// ErrInvalidToken is returned for any token that fails signature, type,
// issuer or expiry validation.
var ErrInvalidToken = errors.New("security: invalid token")

// AccessClaims is the JWT payload of an admin access token.
type AccessClaims struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Type     string `json:"typ"`
	jwt.RegisteredClaims
}

// TokenManager issues and validates access tokens.
type TokenManager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

// NewTokenManager builds a manager from the JWT_SECRET env value. The secret
// is validated (>= 32 chars) at config load time.
func NewTokenManager(secret, issuer string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), issuer: issuer, accessTTL: accessTTL}
}

// AccessTTL exposes the configured access-token lifetime (used in responses).
func (m *TokenManager) AccessTTL() time.Duration { return m.accessTTL }

// IssueAccessToken signs a short-lived HS256 access token for the admin.
func (m *TokenManager) IssueAccessToken(admin *models.AdminUser) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTTL)
	claims := AccessClaims{
		Username: admin.Username,
		Email:    admin.Email,
		Type:     TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   admin.ID,
			Issuer:    m.issuer,
			ID:        ids.New(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("security: sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// ParseAccessToken verifies the signature, issuer, expiry and token type and
// returns the claims for the request context.
func (m *TokenManager) ParseAccessToken(raw string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		return m.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidToken, err)
	}
	if claims.Type != TokenTypeAccess || claims.Subject == "" {
		return nil, fmt.Errorf("%w: unexpected token type", ErrInvalidToken)
	}
	return claims, nil
}

// GenerateRefreshToken returns a fresh opaque refresh token. Only its SHA-256
// hash is persisted, so a database leak cannot be used to impersonate an admin.
func GenerateRefreshToken() (string, error) {
	return ids.NewToken(RefreshTokenBytes)
}

// HashToken hashes an opaque token for storage/lookup.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
