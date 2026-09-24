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

// RefreshTokenRepository reads and writes refresh_tokens.
type RefreshTokenRepository struct {
	base
}

// NewRefreshTokenRepository binds the repository to the database connection.
func NewRefreshTokenRepository(db *database.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{base{db: db}}
}

const refreshColumns = `id, admin_id, token_hash, expires_at, revoked_at, user_agent, ip_address, created_at`

// Create stores the hash of a freshly minted refresh token.
func (r *RefreshTokenRepository) Create(ctx context.Context, t *models.RefreshToken) error {
	const q = `INSERT INTO refresh_tokens (id, admin_id, token_hash, expires_at, revoked_at, user_agent, ip_address, created_at)
	           VALUES (?, ?, ?, ?, NULL, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, t.ID, t.AdminID, t.TokenHash, database.FormatTime(t.ExpiresAt),
		nullString(t.UserAgent), nullString(t.IPAddress), database.FormatTime(t.CreatedAt))
	if err != nil {
		return fmt.Errorf("refresh: create: %w", err)
	}
	return nil
}

// FindByHash loads a token by its SHA-256 hash.
func (r *RefreshTokenRepository) FindByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	q := `SELECT ` + refreshColumns + ` FROM refresh_tokens WHERE token_hash = ? LIMIT 1`
	t, err := scanRefresh(r.db.QueryRowContext(ctx, q, tokenHash))
	if err != nil {
		return nil, notFound(err)
	}
	return t, nil
}

// Revoke marks a single token as revoked. It returns false when the token was
// already revoked (or never existed).
func (r *RefreshTokenRepository) Revoke(ctx context.Context, id string, at time.Time) (bool, error) {
	const q = `UPDATE refresh_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`
	res, err := r.db.ExecContext(ctx, q, database.FormatTime(at), id)
	if err != nil {
		return false, fmt.Errorf("refresh: revoke: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("refresh: revoke rows: %w", err)
	}
	return n > 0, nil
}

// RevokeAllForAdmin kills every active session of an admin. Used on logout-all,
// password change/reset and when refresh-token reuse is detected.
func (r *RefreshTokenRepository) RevokeAllForAdmin(ctx context.Context, adminID string, at time.Time) (int64, error) {
	const q = `UPDATE refresh_tokens SET revoked_at = ? WHERE admin_id = ? AND revoked_at IS NULL`
	res, err := r.db.ExecContext(ctx, q, database.FormatTime(at), adminID)
	if err != nil {
		return 0, fmt.Errorf("refresh: revoke all: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("refresh: revoke all rows: %w", err)
	}
	return n, nil
}

// CountActive returns how many sessions an admin currently has.
func (r *RefreshTokenRepository) CountActive(ctx context.Context, adminID string, at time.Time) (int, error) {
	const q = `SELECT COUNT(*) FROM refresh_tokens WHERE admin_id = ? AND revoked_at IS NULL AND expires_at > ?`
	var n int
	if err := r.db.QueryRowContext(ctx, q, adminID, database.FormatTime(at)).Scan(&n); err != nil {
		return 0, fmt.Errorf("refresh: count active: %w", err)
	}
	return n, nil
}

// DeleteExpired prunes tokens that expired long ago (best-effort housekeeping).
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	const q = `DELETE FROM refresh_tokens WHERE expires_at < ?`
	if _, err := r.db.ExecContext(ctx, q, database.FormatTime(before)); err != nil {
		return fmt.Errorf("refresh: prune: %w", err)
	}
	return nil
}

func scanRefresh(row rowScanner) (*models.RefreshToken, error) {
	var (
		t         models.RefreshToken
		expiresAt string
		revokedAt sql.NullString
		userAgent sql.NullString
		ip        sql.NullString
		createdAt string
	)
	if err := row.Scan(&t.ID, &t.AdminID, &t.TokenHash, &expiresAt, &revokedAt, &userAgent, &ip, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("refresh: scan: %w", err)
	}
	t.UserAgent = userAgent.String
	t.IPAddress = ip.String

	var err error
	if t.ExpiresAt, err = database.ParseTime(expiresAt); err != nil {
		return nil, fmt.Errorf("refresh: parse expires_at: %w", err)
	}
	if t.RevokedAt, err = database.ParseTimePtr(revokedAt); err != nil {
		return nil, fmt.Errorf("refresh: parse revoked_at: %w", err)
	}
	if t.CreatedAt, err = database.ParseTime(createdAt); err != nil {
		return nil, fmt.Errorf("refresh: parse created_at: %w", err)
	}
	return &t, nil
}
