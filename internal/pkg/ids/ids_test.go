package ids

import (
	"regexp"
	"strings"
	"testing"
)

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewReturnsUUIDv4(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 200; i++ {
		id := New()
		if !uuidV4.MatchString(id) {
			t.Fatalf("id %q is not a UUID v4", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id generated: %q", id)
		}
		seen[id] = true
	}
}

func TestNewTokenIsUrlSafeAndUnique(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		token, err := NewToken(32)
		if err != nil {
			t.Fatalf("NewToken: %v", err)
		}
		if len(token) < 40 {
			t.Fatalf("token %q looks too short for 32 bytes", token)
		}
		if strings.ContainsAny(token, "+/= ") {
			t.Fatalf("token %q is not URL-safe", token)
		}
		if seen[token] {
			t.Fatalf("duplicate token generated: %q", token)
		}
		seen[token] = true
	}
}

func TestNewHex(t *testing.T) {
	value, err := NewHex(8)
	if err != nil {
		t.Fatalf("NewHex: %v", err)
	}
	if len(value) != 16 {
		t.Fatalf("hex length = %d, want 16", len(value))
	}
	if strings.ContainsFunc(value, func(r rune) bool {
		return !strings.ContainsRune("0123456789abcdef", r)
	}) {
		t.Fatalf("NewHex returned a non-hex value: %q", value)
	}
}
