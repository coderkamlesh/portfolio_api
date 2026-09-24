package security

import (
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	const password = "correct horse battery staple 42"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("unexpected hash format: %q", hash)
	}

	ok, err := VerifyPassword(hash, password)
	if err != nil || !ok {
		t.Fatalf("VerifyPassword(correct) = %v, %v; want true, nil", ok, err)
	}

	ok, err = VerifyPassword(hash, password+"x")
	if err != nil {
		t.Fatalf("VerifyPassword(wrong): %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword accepted a wrong password")
	}
}

func TestHashPasswordUsesRandomSalt(t *testing.T) {
	a, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	b, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if a == b {
		t.Fatal("two hashes of the same password are identical — salt is not random")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	cases := map[string]string{
		"empty":         "",
		"wrong scheme":  "$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		"missing parts": "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA",
		"bad salt b64":  "$argon2id$v=19$m=65536,t=3,p=2$!!!$aGFzaA",
	}

	for name, hash := range cases {
		ok, err := VerifyPassword(hash, "whatever")
		if ok {
			t.Errorf("%s: accepted a malformed hash", name)
		}
		if err == nil {
			t.Errorf("%s: expected an error for a malformed hash", name)
		}
	}
}

func TestNeedsRehash(t *testing.T) {
	current, err := HashPassword("password-123456")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if NeedsRehash(current) {
		t.Fatal("freshly hashed password reported as needing a rehash")
	}

	weaker := "$argon2id$v=19$m=4096,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2g"
	if !NeedsRehash(weaker) {
		t.Fatal("weaker parameters not detected")
	}
	if !NeedsRehash("not-a-hash") {
		t.Fatal("malformed hash must need a rehash")
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name     string
		password string
		minLen   int
		wantErr  bool
	}{
		{name: "valid", password: "long-passphrase 2026", minLen: 12},
		{name: "too short", password: "short1", minLen: 12, wantErr: true},
		{name: "only letters", password: "abcdefghijklmnop", minLen: 12, wantErr: true},
		{name: "only digits", password: "123456789012345678", minLen: 12, wantErr: true},
		{name: "floor of 8", password: "abc123", minLen: 4, wantErr: true},
		{name: "floor of 8 ok", password: "abcd1234", minLen: 4},
	}

	for _, tc := range cases {
		err := ValidatePassword(tc.password, tc.minLen)
		if tc.wantErr && err == nil {
			t.Errorf("%s: expected an error", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
		}
	}
}

func TestDummyPasswordHashIsVerifiable(t *testing.T) {
	// Login runs this verification for unknown identifiers; it must never
	// panic or error, otherwise timing equalisation breaks.
	if _, err := VerifyPassword(DummyPasswordHash, "anything"); err != nil {
		t.Fatalf("DummyPasswordHash is not a valid hash: %v", err)
	}
}
