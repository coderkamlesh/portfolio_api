package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// OTPRepository reads and writes otp_challenges.
type OTPRepository struct {
	base
}

// NewOTPRepository binds the repository to the database connection.
func NewOTPRepository(db *database.DB) *OTPRepository {
	return &OTPRepository{base{db: db}}
}

const otpColumns = `id, admin_id, otp_hash, purpose, expires_at, consumed_at, attempt_count, created_at`

// Create stores a new challenge.
func (r *OTPRepository) Create(ctx context.Context, c *models.OTPChallenge) error {
	const q = `INSERT INTO otp_challenges (id, admin_id, otp_hash, purpose, expires_at, consumed_at, attempt_count, created_at)
	           VALUES (?, ?, ?, ?, ?, NULL, 0, ?)`
	_, err := r.db.ExecContext(ctx, q, c.ID, c.AdminID, c.OTPHash, c.Purpose,
		database.FormatTime(c.ExpiresAt), database.FormatTime(c.CreatedAt))
	if err != nil {
		return fmt.Errorf("otp: create: %w", err)
	}
	return nil
}

// FindByID loads a challenge by id.
func (r *OTPRepository) FindByID(ctx context.Context, id string) (*models.OTPChallenge, error) {
	q := `SELECT ` + otpColumns + ` FROM otp_challenges WHERE id = ? LIMIT 1`
	c, err := scanOTP(r.db.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, notFound(err)
	}
	return c, nil
}

// FindLatestPending returns the newest unconsumed challenge of a purpose for
// an admin — used to enforce the resend cooldown.
func (r *OTPRepository) FindLatestPending(ctx context.Context, adminID, purpose string, at time.Time) (*models.OTPChallenge, error) {
	q := `SELECT ` + otpColumns + ` FROM otp_challenges
	      WHERE admin_id = ? AND purpose = ? AND consumed_at IS NULL AND expires_at > ?
	      ORDER BY created_at DESC LIMIT 1`
	c, err := scanOTP(r.db.QueryRowContext(ctx, q, adminID, purpose, database.FormatTime(at)))
	if err != nil {
		return nil, notFound(err)
	}
	return c, nil
}

// CountCreatedSince counts challenges of a purpose issued after `since`, which
// caps how many mails an attacker can trigger for one account.
func (r *OTPRepository) CountCreatedSince(ctx context.Context, adminID, purpose string, since time.Time) (int, error) {
	q := `SELECT COUNT(*) FROM otp_challenges WHERE admin_id = ? AND purpose = ? AND created_at >= ?`
	var n int
	if err := r.db.QueryRowContext(ctx, q, adminID, purpose, database.FormatTime(since)).Scan(&n); err != nil {
		return 0, fmt.Errorf("otp: count since: %w", err)
	}
	return n, nil
}

// InvalidatePending consumes every pending challenge of a purpose, so only the
// most recent code can ever be used.
func (r *OTPRepository) InvalidatePending(ctx context.Context, adminID, purpose string, at time.Time) error {
	const q = `UPDATE otp_challenges SET consumed_at = ? WHERE admin_id = ? AND purpose = ? AND consumed_at IS NULL`
	if _, err := r.db.ExecContext(ctx, q, database.FormatTime(at), adminID, purpose); err != nil {
		return fmt.Errorf("otp: invalidate pending: %w", err)
	}
	return nil
}

// IncrementAttempts bumps attempt_count and returns the new value.
func (r *OTPRepository) IncrementAttempts(ctx context.Context, id string) (int, error) {
	const q = `UPDATE otp_challenges SET attempt_count = attempt_count + 1 WHERE id = ?`
	if _, err := r.db.ExecContext(ctx, q, id); err != nil {
		return 0, fmt.Errorf("otp: increment attempts: %w", err)
	}
	var attempts int
	if err := r.db.QueryRowContext(ctx, `SELECT attempt_count FROM otp_challenges WHERE id = ?`, id).Scan(&attempts); err != nil {
		return 0, fmt.Errorf("otp: read attempts: %w", err)
	}
	return attempts, nil
}

// Consume marks a challenge as used in a single conditional UPDATE, which
// makes the OTP one-shot even when two requests race.
func (r *OTPRepository) Consume(ctx context.Context, id string, at time.Time) (bool, error) {
	const q = `UPDATE otp_challenges SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL`
	res, err := r.db.ExecContext(ctx, q, database.FormatTime(at), id)
	if err != nil {
		return false, fmt.Errorf("otp: consume: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("otp: consume rows: %w", err)
	}
	return n > 0, nil
}

// DeleteOlderThan prunes finished challenges; called best-effort so the table
// does not grow forever.
func (r *OTPRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	const q = `DELETE FROM otp_challenges WHERE created_at < ? AND (consumed_at IS NOT NULL OR expires_at < ?)`
	if _, err := r.db.ExecContext(ctx, q, database.FormatTime(before), database.FormatTime(before)); err != nil {
		return fmt.Errorf("otp: prune: %w", err)
	}
	return nil
}

func scanOTP(row rowScanner) (*models.OTPChallenge, error) {
	var (
		c          models.OTPChallenge
		expiresAt  string
		consumedAt sql.NullString
		createdAt  string
		attempts   sql.NullInt64
	)
	if err := row.Scan(&c.ID, &c.AdminID, &c.OTPHash, &c.Purpose, &expiresAt, &consumedAt, &attempts, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("otp: scan: %w", err)
	}
	c.AttemptCount = int(attempts.Int64)

	var err error
	if c.ExpiresAt, err = database.ParseTime(expiresAt); err != nil {
		return nil, fmt.Errorf("otp: parse expires_at: %w", err)
	}
	if c.ConsumedAt, err = database.ParseTimePtr(consumedAt); err != nil {
		return nil, fmt.Errorf("otp: parse consumed_at: %w", err)
	}
	if c.CreatedAt, err = database.ParseTime(createdAt); err != nil {
		return nil, fmt.Errorf("otp: parse created_at: %w", err)
	}
	return &c, nil
}
