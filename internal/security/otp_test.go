package security

import (
	"strings"
	"testing"
)

func TestGenerateOTPShape(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 50; i++ {
		code, err := GenerateOTP(6)
		if err != nil {
			t.Fatalf("GenerateOTP: %v", err)
		}
		if len(code) != 6 {
			t.Fatalf("code %q has length %d, want 6", code, len(code))
		}
		if code[0] == '0' {
			t.Fatalf("code %q starts with a zero", code)
		}
		for _, r := range code {
			if r < '0' || r > '9' {
				t.Fatalf("code %q contains a non-digit", code)
			}
		}
		seen[code] = true
	}

	if len(seen) < 40 {
		t.Fatalf("only %d unique codes in 50 draws — randomness looks broken", len(seen))
	}
}

func TestGenerateOTPRejectsBadLength(t *testing.T) {
	for _, length := range []int{0, 3, 11} {
		if _, err := GenerateOTP(length); err == nil {
			t.Errorf("length %d: expected an error", length)
		}
	}
}

func TestHashAndVerifyOTP(t *testing.T) {
	code, err := GenerateOTP(6)
	if err != nil {
		t.Fatalf("GenerateOTP: %v", err)
	}

	hash, err := HashOTP(code)
	if err != nil {
		t.Fatalf("HashOTP: %v", err)
	}
	if strings.Contains(hash, code) {
		t.Fatal("stored OTP hash contains the plaintext code")
	}

	ok, err := VerifyOTP(hash, code)
	if err != nil || !ok {
		t.Fatalf("VerifyOTP(correct) = %v, %v; want true, nil", ok, err)
	}

	wrong := "000000"
	if wrong == code {
		wrong = "999999"
	}
	ok, err = VerifyOTP(hash, wrong)
	if err != nil {
		t.Fatalf("VerifyOTP(wrong): %v", err)
	}
	if ok {
		t.Fatal("VerifyOTP accepted a wrong code")
	}
}

func TestNormalizeOTP(t *testing.T) {
	cases := map[string]string{
		" 123456 ":    "123456",
		"123-456":     "123456",
		"1 2 3 4 5 6": "123456",
		"\t123 456\n": "123456",
		"123456":      "123456",
	}

	for input, want := range cases {
		if got := NormalizeOTP(input); got != want {
			t.Errorf("NormalizeOTP(%q) = %q, want %q", input, got, want)
		}
	}
}
