package security

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// GenerateOTP returns a random numeric one-time code of the given length.
// The first digit is never zero so the code keeps its full entropy when a
// client trims leading whitespace.
func GenerateOTP(length int) (string, error) {
	if length < 4 || length > 10 {
		return "", fmt.Errorf("security: otp length %d out of range", length)
	}
	digits := make([]byte, length)
	for i := range digits {
		max := big.NewInt(10)
		if i == 0 {
			max = big.NewInt(9) // 1..9
		}
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("security: otp generation failed: %w", err)
		}
		offset := int64(0)
		if i == 0 {
			offset = 1
		}
		digits[i] = byte('0' + offset + n.Int64())
	}
	return string(digits), nil
}

// HashOTP hashes an OTP with argon2id before it is stored in
// otp_challenges.otp_hash, so a leaked table cannot be replayed.
func HashOTP(code string) (string, error) {
	return hashSecret(code)
}

// VerifyOTP compares a submitted code against the stored otp_hash.
func VerifyOTP(encoded, code string) (bool, error) {
	return verifySecret(encoded, code)
}

// NormalizeOTP trims spaces and interior separators users tend to paste.
func NormalizeOTP(code string) string {
	code = strings.TrimSpace(code)
	code = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '\t', '\n', '\r':
			return -1
		}
		return r
	}, code)
	return code
}

// mustHash is a helper for package-level constants; it panics because a
// failure here means the process cannot hash anything at all.
func mustHash(secret string) string {
	h, err := hashSecret(secret)
	if err != nil {
		panic(err)
	}
	return h
}
