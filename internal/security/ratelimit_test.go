package security

import (
	"testing"
	"time"
)

func TestLimiterBlocksAfterMaxFailures(t *testing.T) {
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	limiter := newLimiterWithClock(3, 15*time.Minute, func() time.Time { return now })
	key := ClientKey("kamlesh", "1.2.3.4")

	if !limiter.Allow(key) {
		t.Fatal("fresh limiter refused the first attempt")
	}

	for i := 0; i < 3; i++ {
		limiter.RegisterFailure(key)
	}

	if limiter.Allow(key) {
		t.Fatal("limiter still allows attempts after max failures")
	}
	if retry := limiter.RetryAfter(key); retry <= 0 || retry > 15*time.Minute {
		t.Fatalf("RetryAfter = %s, want within (0, 15m]", retry)
	}

	// New window -> allowed again.
	now = now.Add(16 * time.Minute)
	if !limiter.Allow(key) {
		t.Fatal("limiter did not reset after the window elapsed")
	}
}

func TestLimiterResetClearsFailures(t *testing.T) {
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	limiter := newLimiterWithClock(2, time.Minute, func() time.Time { return now })
	key := ClientKey("kamlesh", "1.2.3.4")

	limiter.RegisterFailure(key)
	limiter.RegisterFailure(key)
	if limiter.Allow(key) {
		t.Fatal("expected the limiter to be exhausted")
	}

	limiter.Reset(key)
	if !limiter.Allow(key) {
		t.Fatal("Reset did not clear the counter")
	}
	if retry := limiter.RetryAfter(key); retry != 0 {
		t.Fatalf("RetryAfter after reset = %s, want 0", retry)
	}
}

func TestLimiterKeysAreScoped(t *testing.T) {
	limiter := NewLimiter(1, time.Minute)

	one := ClientKey("kamlesh", "1.2.3.4")
	two := ClientKey("KAMLESH ", "5.6.7.8") // same identifier, other IP
	three := ClientKey("someone-else", "1.2.3.4")

	limiter.RegisterFailure(one)

	if !limiter.Allow(two) {
		t.Error("another IP should have its own budget")
	}
	if !limiter.Allow(three) {
		t.Error("another identifier should have its own budget")
	}
	if limiter.Allow(one) {
		t.Error("the exhausted key must stay blocked")
	}
}

func TestLimiterDefaultsAreSafe(t *testing.T) {
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	limiter := newLimiterWithClock(0, 0, func() time.Time { return now })

	limiter.RegisterFailure("k")
	if limiter.Allow("k") {
		t.Fatal("zero max must still block after one failure")
	}
	if retry := limiter.RetryAfter("k"); retry != time.Minute {
		t.Fatalf("RetryAfter = %s, want the 1m fallback window", retry)
	}
}
