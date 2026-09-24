package security

import (
	"strings"
	"sync"
	"time"
)

// Limiter is a small in-memory sliding-window counter used to slow down
// password and OTP brute force. It is per-process: behind several Lambda
// instances an attacker gets `instances x max` attempts, which is an accepted
// trade-off for a single-admin panel (no Redis dependency). The database
// based OTP counters (attempt_count, AUTH_OTP_MAX_PER_WINDOW) remain the
// hard limit.
type Limiter struct {
	mu       sync.Mutex
	attempts map[string]*counter
	max      int
	window   time.Duration
	now      func() time.Time
}

type counter struct {
	count   int
	resetAt time.Time
}

// NewLimiter creates a limiter allowing max failures per window.
func NewLimiter(max int, window time.Duration) *Limiter {
	return newLimiterWithClock(max, window, time.Now)
}

func newLimiterWithClock(max int, window time.Duration, now func() time.Time) *Limiter {
	if max < 1 {
		max = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &Limiter{attempts: make(map[string]*counter), max: max, window: window, now: now}
}

// Allow reports whether another attempt is permitted for key.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.attempts[key]
	if !ok {
		return true
	}
	if !l.now().Before(c.resetAt) {
		delete(l.attempts, key)
		return true
	}
	return c.count < l.max
}

// RegisterFailure counts one failed attempt and (re)arms the window.
func (l *Limiter) RegisterFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	c, ok := l.attempts[key]
	if !ok || !now.Before(c.resetAt) {
		l.attempts[key] = &counter{count: 1, resetAt: now.Add(l.window)}
		l.sweepLocked(now)
		return
	}
	c.count++
}

// Reset clears the counter, e.g. after a successful login.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// RetryAfter reports how long the caller must wait before the next attempt.
func (l *Limiter) RetryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.attempts[key]
	if !ok {
		return 0
	}
	if d := c.resetAt.Sub(l.now()); d > 0 {
		return d
	}
	return 0
}

// sweepLocked drops expired counters to keep the map bounded on a long-lived
// process. Callers must hold l.mu.
func (l *Limiter) sweepLocked(now time.Time) {
	if len(l.attempts) < 1000 {
		return
	}
	for k, c := range l.attempts {
		if !now.Before(c.resetAt) {
			delete(l.attempts, k)
		}
	}
}

// ClientKey builds a limiter key from an identifier (username/email) and the
// caller IP, so one noisy client cannot lock out an unrelated admin.
func ClientKey(identifier, ip string) string {
	return strings.ToLower(strings.TrimSpace(identifier)) + "|" + ip
}
