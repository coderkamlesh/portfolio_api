// Package ids generates the TEXT primary keys and opaque tokens used across
// the API. Everything comes from crypto/rand — no third-party UUID dependency.
package ids

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// New returns a random UUID v4 string (e.g. "9f1c....-....-4...-8...-....").
// All tables in db_schema.sql use TEXT primary keys, so a UUID keeps rows
// sortable, loggable and safe to expose in URLs.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand never fails on supported platforms; if it does, the
		// process cannot safely mint IDs, so fail fast.
		panic(fmt.Errorf("ids: crypto/rand failed: %w", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NewToken returns a URL-safe random token of nBytes of entropy.
func NewToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("ids: crypto/rand failed: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewHex returns a random hex string of nBytes bytes.
func NewHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("ids: crypto/rand failed: %w", err)
	}
	return hex.EncodeToString(b), nil
}
