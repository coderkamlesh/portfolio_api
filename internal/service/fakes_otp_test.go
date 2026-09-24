package service

import (
	"context"
	"sync"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// fakeOTPStore is an in-memory otp_challenges table. It is mutex-protected
// because the service prunes expired rows from a detached goroutine.
type fakeOTPStore struct {
	mu   sync.Mutex
	rows []*models.OTPChallenge
}

func (f *fakeOTPStore) Create(_ context.Context, c *models.OTPChallenge) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, c)
	return nil
}

func (f *fakeOTPStore) FindByID(_ context.Context, id string) (*models.OTPChallenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, models.ErrNotFound
}

func (f *fakeOTPStore) FindLatestPending(_ context.Context, adminID, purpose string, at time.Time) (*models.OTPChallenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var latest *models.OTPChallenge
	for _, r := range f.rows {
		if r.AdminID != adminID || r.Purpose != purpose || r.ConsumedAt != nil || !at.Before(r.ExpiresAt) {
			continue
		}
		if latest == nil || r.CreatedAt.After(latest.CreatedAt) {
			latest = r
		}
	}
	if latest == nil {
		return nil, models.ErrNotFound
	}
	return latest, nil
}

func (f *fakeOTPStore) CountCreatedSince(_ context.Context, adminID, purpose string, since time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	count := 0
	for _, r := range f.rows {
		if r.AdminID == adminID && r.Purpose == purpose && !r.CreatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

func (f *fakeOTPStore) InvalidatePending(_ context.Context, adminID, purpose string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.AdminID == adminID && r.Purpose == purpose && r.ConsumedAt == nil {
			consumed := at
			r.ConsumedAt = &consumed
		}
	}
	return nil
}

func (f *fakeOTPStore) IncrementAttempts(_ context.Context, id string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id {
			r.AttemptCount++
			return r.AttemptCount, nil
		}
	}
	return 0, models.ErrNotFound
}

func (f *fakeOTPStore) Consume(_ context.Context, id string, at time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id {
			if r.ConsumedAt != nil {
				return false, nil
			}
			consumed := at
			r.ConsumedAt = &consumed
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeOTPStore) DeleteOlderThan(_ context.Context, _ time.Time) error { return nil }

// issuedCount reports how many challenges exist for a purpose.
func (f *fakeOTPStore) issuedCount(purpose string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	count := 0
	for _, r := range f.rows {
		if r.Purpose == purpose {
			count++
		}
	}
	return count
}
