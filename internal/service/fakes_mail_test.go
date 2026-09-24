package service

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/email"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// fakeRefreshStore is an in-memory refresh_tokens table (also touched by the
// background prune goroutine, hence the mutex).
type fakeRefreshStore struct {
	mu   sync.Mutex
	rows []*models.RefreshToken
}

func (f *fakeRefreshStore) Create(_ context.Context, t *models.RefreshToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, t)
	return nil
}

func (f *fakeRefreshStore) FindByHash(_ context.Context, tokenHash string) (*models.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.TokenHash == tokenHash {
			return r, nil
		}
	}
	return nil, models.ErrNotFound
}

func (f *fakeRefreshStore) Revoke(_ context.Context, id string, at time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id {
			if r.RevokedAt != nil {
				return false, nil
			}
			revoked := at
			r.RevokedAt = &revoked
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeRefreshStore) RevokeAllForAdmin(_ context.Context, adminID string, at time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var count int64
	for _, r := range f.rows {
		if r.AdminID == adminID && r.RevokedAt == nil {
			revoked := at
			r.RevokedAt = &revoked
			count++
		}
	}
	return count, nil
}

func (f *fakeRefreshStore) CountActive(_ context.Context, adminID string, at time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	count := 0
	for _, r := range f.rows {
		if r.AdminID == adminID && r.RevokedAt == nil && at.Before(r.ExpiresAt) {
			count++
		}
	}
	return count, nil
}

func (f *fakeRefreshStore) DeleteExpired(_ context.Context, _ time.Time) error { return nil }

// activeCount counts rows that are still usable.
func (f *fakeRefreshStore) activeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	count := 0
	for _, r := range f.rows {
		if r.RevokedAt == nil {
			count++
		}
	}
	return count
}

// storedCount counts every row, revoked or not.
func (f *fakeRefreshStore) storedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rows)
}

// fakeAuditStore records audit_log inserts.
type fakeAuditStore struct {
	entries []*models.AuditEntry
}

func (f *fakeAuditStore) Insert(_ context.Context, e *models.AuditEntry) error {
	f.entries = append(f.entries, e)
	return nil
}

// ListAuditEntries returns a filtered page so the audit query tests can exercise
// the same contract as the repository.
func (f *fakeAuditStore) ListAuditEntries(_ context.Context, entityType, action string, limit, offset int) ([]models.AuditEntry, int64, error) {
	matched := make([]models.AuditEntry, 0, len(f.entries))
	for _, e := range f.entries {
		if entityType != "" && e.EntityType != entityType {
			continue
		}
		if action != "" && e.Action != action {
			continue
		}
		matched = append(matched, *e)
	}
	total := int64(len(matched))

	// Newest first, matching the repository ordering.
	sort.SliceStable(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID > matched[j].ID
	})

	if offset > len(matched) {
		offset = len(matched)
	}
	matched = matched[offset:]
	if limit > 0 && limit < len(matched) {
		matched = matched[:limit]
	}
	return matched, total, nil
}

func (f *fakeAuditStore) CountAuditEntries(_ context.Context, entityType, action string) (int64, error) {
	var total int64
	for _, e := range f.entries {
		if entityType != "" && e.EntityType != entityType {
			continue
		}
		if action != "" && e.Action != action {
			continue
		}
		total++
	}
	return total, nil
}

func (f *fakeAuditStore) hasAction(action string) bool {
	for _, e := range f.entries {
		if e.Action == action {
			return true
		}
	}
	return false
}

// fakeMailer captures the mails the service tries to deliver.
type fakeMailer struct {
	sent []email.Message
	err  error
}

func (f *fakeMailer) Send(_ context.Context, msg email.Message) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, msg)
	return nil
}

func (f *fakeMailer) count() int { return len(f.sent) }

// lastCode reads the one-time code out of the most recent mail. Reading it from
// the mail keeps the OTP template covered by the tests as well.
func (f *fakeMailer) lastCode(t *testing.T) string {
	t.Helper()

	if len(f.sent) == 0 {
		t.Fatal("no mail was sent")
	}
	const prefix = "Your portfolio admin code: "
	subject := f.sent[len(f.sent)-1].Subject
	if !hasPrefix(subject, prefix) {
		t.Fatalf("unexpected mail subject: %q", subject)
	}
	return subject[len(prefix):]
}
