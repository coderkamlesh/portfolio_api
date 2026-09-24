package security

import (
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

const testSecret = "unit-test-secret-key-that-is-long-enough-1234"

func testAdmin() *models.AdminUser {
	return &models.AdminUser{
		ID:       "11111111-1111-4111-8111-111111111111",
		Username: "kamlesh",
		Email:    "kamlesh@example.com",
		IsActive: true,
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	manager := NewTokenManager(testSecret, "portfolio-api", 15*time.Minute)

	token, expiresAt, err := manager.IssueAccessToken(testAdmin())
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if time.Until(expiresAt) <= 0 {
		t.Fatalf("token already expired at %s", expiresAt)
	}

	claims, err := manager.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.Subject != testAdmin().ID {
		t.Errorf("subject = %q, want %q", claims.Subject, testAdmin().ID)
	}
	if claims.Username != "kamlesh" || claims.Email != "kamlesh@example.com" {
		t.Errorf("unexpected claims: %+v", claims)
	}
	if claims.Type != TokenTypeAccess {
		t.Errorf("typ = %q, want %q", claims.Type, TokenTypeAccess)
	}
}

func TestParseAccessTokenRejectsForeignSecret(t *testing.T) {
	issuer := NewTokenManager(testSecret, "portfolio-api", 15*time.Minute)
	verifier := NewTokenManager("another-secret-key-that-is-long-enough-99", "portfolio-api", 15*time.Minute)

	token, _, err := issuer.IssueAccessToken(testAdmin())
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := verifier.ParseAccessToken(token); err == nil {
		t.Fatal("a token signed with another secret was accepted")
	}
}

func TestParseAccessTokenRejectsWrongIssuer(t *testing.T) {
	issuer := NewTokenManager(testSecret, "other-issuer", 15*time.Minute)
	verifier := NewTokenManager(testSecret, "portfolio-api", 15*time.Minute)

	token, _, err := issuer.IssueAccessToken(testAdmin())
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := verifier.ParseAccessToken(token); err == nil {
		t.Fatal("a token from another issuer was accepted")
	}
}

func TestParseAccessTokenRejectsExpired(t *testing.T) {
	manager := NewTokenManager(testSecret, "portfolio-api", -time.Minute)

	token, _, err := manager.IssueAccessToken(testAdmin())
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := manager.ParseAccessToken(token); err == nil {
		t.Fatal("an expired token was accepted")
	}
}

func TestParseAccessTokenRejectsGarbage(t *testing.T) {
	manager := NewTokenManager(testSecret, "portfolio-api", 15*time.Minute)

	for _, raw := range []string{"", "not.a.token", "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4In0."} {
		if _, err := manager.ParseAccessToken(raw); err == nil {
			t.Errorf("token %q was accepted", raw)
		}
	}
}

func TestRefreshTokenHashing(t *testing.T) {
	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if len(token) < 40 {
		t.Fatalf("refresh token looks too short: %q", token)
	}

	hash := HashToken(token)
	if hash == token {
		t.Fatal("HashToken returned the token itself")
	}
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64 hex chars", len(hash))
	}
	if HashToken(token) != hash {
		t.Fatal("HashToken is not deterministic")
	}
	if HashToken(token+"x") == hash {
		t.Fatal("different tokens produced the same hash")
	}

	other, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if other == token {
		t.Fatal("two generated refresh tokens are identical")
	}
}
