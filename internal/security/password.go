// Package security contains the cryptographic primitives of the admin auth
// module: argon2id password hashing, OTP hashing, JWT access tokens, opaque
// refresh tokens and the in-memory login rate limiter.
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. Tuned for a serverless (512 MB) Lambda: 64 MB of memory
// per hash, 3 passes, 2 lanes. They are intentionally hard-coded so hashes
// stay verifiable forever — never change them without a rehash migration.
const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // KiB
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// ErrInvalidHash is returned when a stored hash is malformed.
var ErrInvalidHash = errors.New("security: malformed argon2id hash")

// HashPassword derives an argon2id hash encoded as a PHC string:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<salt-b64>$<hash-b64>
func HashPassword(password string) (string, error) {
	return hashSecret(password)
}

// VerifyPassword compares a plaintext password against a stored hash.
// The comparison is constant-time; a malformed hash returns
// ErrInvalidHash so callers can distinguish "bad data" from "wrong password".
func VerifyPassword(encoded, password string) (bool, error) {
	return verifySecret(encoded, password)
}

func hashSecret(secret string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("security: salt generation failed: %w", err)
	}
	sum := argon2.IDKey([]byte(secret), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

func verifySecret(encoded, secret string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrInvalidHash
	}

	var memory, time2 uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time2, &threads); err != nil {
		return false, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}

	got := argon2.IDKey([]byte(secret), salt, time2, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// NeedsRehash reports whether a stored hash was produced with weaker
// parameters than the current ones, so it can be upgraded on next login.
func NeedsRehash(encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return true
	}
	var memory, time2 uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time2, &threads); err != nil {
		return true
	}
	return memory != argonMemory || time2 != argonTime || threads != argonThreads
}

// ValidatePassword enforces the shared password policy and returns an error
// whose message starts with "must …" so callers can compose their own prefix.
//
// Policy: at least minLength characters (8 minimum), at most 256, and a mix of
// letters with numbers/symbols/spaces. Passphrases are allowed on purpose — no
// character-class zoo, which pushes people towards weaker, reused passwords.
func ValidatePassword(password string, minLength int) error {
	if minLength < 8 {
		minLength = 8
	}
	if len(password) < minLength {
		return fmt.Errorf("be at least %d characters long", minLength)
	}
	if len(password) > 256 {
		return errors.New("be at most 256 characters long")
	}

	var hasLetter, hasDigitOrSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r), unicode.IsPunct(r), unicode.IsSymbol(r), unicode.IsSpace(r):
			hasDigitOrSymbol = true
		}
	}
	if !hasLetter || !hasDigitOrSymbol {
		return errors.New("mix letters with numbers, symbols or spaces")
	}
	return nil
}

// DummyPasswordHash is a valid argon2id hash of a random string. Login runs a
// verification against it when the account does not exist, so the response
// time does not reveal whether a username is registered.
var DummyPasswordHash = mustHash("portfolio-api-dummy-secret")
