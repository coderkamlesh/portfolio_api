package security

import (
	"strings"
	"testing"
)

func TestMaskEmail(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "kamlesh@example.com", want: "k" + strings.Repeat("*", 5) + "h@example.com"},
		{input: "ab@example.com", want: "a***b@example.com"},
		{input: "a@example.com", want: "a***@example.com"},
		{input: "no-at-sign", want: "***"},
		{input: "@example.com", want: "***"},
		{input: "long.name.here@domain.co.in", want: "l" + strings.Repeat("*", 12) + "e@domain.co.in"},
	}

	for _, tc := range cases {
		if got := MaskEmail(tc.input); got != tc.want {
			t.Errorf("MaskEmail(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMaskEmailHidesLocalPart(t *testing.T) {
	const address = "very.secret.address@example.com"
	masked := MaskEmail(address)

	if strings.Contains(masked, "secret") {
		t.Fatalf("masked address still leaks the local part: %q", masked)
	}
	if !strings.HasSuffix(masked, "@example.com") {
		t.Fatalf("masked address lost the domain: %q", masked)
	}
}
