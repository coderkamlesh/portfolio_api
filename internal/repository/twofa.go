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

// TwoFARepository reads and writes admin_2fa.
type TwoFARepository struct {
	base
}

// NewTwoFARepository binds the repository to the database connection.
func NewTwoFARepository(db *database.DB) *TwoFARepository {
	return &TwoFARepository{base{db: db}}
}

const twoFAColumns = `id, admin_id, method, totp_secret, is_enabled, confirmed_at, created_at`

// FindByAdminAndMethod loads the 2FA config for one method, or
// models.ErrNotFound when the admin never enrolled.
func (r *TwoFARepository) FindByAdminAndMethod(ctx context.Context, adminID, method string) (*models.TwoFactorConfig, error) {
	q := `SELECT ` + twoFAColumns + ` FROM admin_2fa WHERE admin_id = ? AND method = ? LIMIT 1`
	cfg, err := scanTwoFA(r.db.QueryRowContext(ctx, q, adminID, method))
	if err != nil {
		return nil, notFound(err)
	}
	return cfg, nil
}

// ListByAdmin returns every 2FA method configured for an admin.
func (r *TwoFARepository) ListByAdmin(ctx context.Context, adminID string) ([]models.TwoFactorConfig, error) {
	q := `SELECT ` + twoFAColumns + ` FROM admin_2fa WHERE admin_id = ? ORDER BY method`
	rows, err := r.db.QueryContext(ctx, q, adminID)
	if err != nil {
		return nil, fmt.Errorf("2fa: list: %w", err)
	}
	defer rows.Close()

	var out []models.TwoFactorConfig
	for rows.Next() {
		cfg, err := scanTwoFA(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *cfg)
	}
	return out, rows.Err()
}

// Upsert creates or updates the config row for (admin_id, method).
func (r *TwoFARepository) Upsert(ctx context.Context, cfg *models.TwoFactorConfig) error {
	const update = `UPDATE admin_2fa SET is_enabled = ?, confirmed_at = ? WHERE admin_id = ? AND method = ?`
	res, err := r.db.ExecContext(ctx, update,
		boolToInt(cfg.IsEnabled), database.FormatTimePtr(cfg.ConfirmedAt), cfg.AdminID, cfg.Method)
	if err != nil {
		return fmt.Errorf("2fa: upsert update: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n > 0 {
		return nil
	}

	const insert = `INSERT INTO admin_2fa (id, admin_id, method, totp_secret, is_enabled, confirmed_at, created_at)
	                VALUES (?, ?, ?, ?, ?, ?, ?)`
	if _, err := r.db.ExecContext(ctx, insert,
		cfg.ID, cfg.AdminID, cfg.Method, nullString(cfg.TOTPSecret), boolToInt(cfg.IsEnabled),
		database.FormatTimePtr(cfg.ConfirmedAt), database.FormatTime(cfg.CreatedAt),
	); err != nil {
		return fmt.Errorf("2fa: upsert insert: %w", err)
	}
	return nil
}

// SetEnabled flips the enabled flag for an existing row and returns
// models.ErrNotFound when the admin has no such method configured.
func (r *TwoFARepository) SetEnabled(ctx context.Context, adminID, method string, enabled bool, at time.Time) error {
	const q = `UPDATE admin_2fa SET is_enabled = ?, confirmed_at = ? WHERE admin_id = ? AND method = ?`
	res, err := r.db.ExecContext(ctx, q, boolToInt(enabled), database.FormatTimePtr(&at), adminID, method)
	if err != nil {
		return fmt.Errorf("2fa: set enabled: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("2fa: set enabled rows: %w", err)
	}
	if n == 0 {
		return models.ErrNotFound
	}
	return nil
}

func scanTwoFA(row rowScanner) (*models.TwoFactorConfig, error) {
	var (
		cfg         models.TwoFactorConfig
		secret      sql.NullString
		enabled     sql.NullInt64
		confirmedAt sql.NullString
		createdAt   string
	)
	if err := row.Scan(&cfg.ID, &cfg.AdminID, &cfg.Method, &secret, &enabled, &confirmedAt, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("2fa: scan: %w", err)
	}
	cfg.TOTPSecret = secret.String
	cfg.IsEnabled = boolFromInt(enabled)

	confirmed, err := database.ParseTimePtr(confirmedAt)
	if err != nil {
		return nil, fmt.Errorf("2fa: parse confirmed_at: %w", err)
	}
	cfg.ConfirmedAt = confirmed

	if cfg.CreatedAt, err = database.ParseTime(createdAt); err != nil {
		return nil, fmt.Errorf("2fa: parse created_at: %w", err)
	}
	return &cfg, nil
}

// nullString turns an empty string into a SQL NULL so optional columns stay
// nullable (totp_secret is unused for EMAIL_OTP).
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
