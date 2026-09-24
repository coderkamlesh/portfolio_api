package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// AdminRepository reads and writes admin_users.
type AdminRepository struct {
	base
}

// NewAdminRepository binds the repository to the database connection.
func NewAdminRepository(db *database.DB) *AdminRepository {
	return &AdminRepository{base{db: db}}
}

const adminColumns = `id, username, email, password_hash, is_active, last_login_at, created_at, updated_at`

// Create inserts a new admin account.
func (r *AdminRepository) Create(ctx context.Context, a *models.AdminUser) error {
	const q = `INSERT INTO admin_users (id, username, email, password_hash, is_active, created_at, updated_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		a.ID, a.Username, normalizeEmail(a.Email), a.PasswordHash, boolToInt(a.IsActive),
		database.FormatTime(a.CreatedAt), database.FormatTime(a.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("admin: create: %w", err)
	}
	return nil
}

// Upsert creates the admin, or refreshes the credentials of the existing
// username. Used by the adminctl CLI so seeding stays idempotent.
func (r *AdminRepository) Upsert(ctx context.Context, a *models.AdminUser) error {
	const q = `INSERT INTO admin_users (id, username, email, password_hash, is_active, created_at, updated_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?)
	           ON CONFLICT(username) DO UPDATE SET
	               email = excluded.email,
	               password_hash = excluded.password_hash,
	               is_active = 1,
	               updated_at = excluded.updated_at`
	_, err := r.db.ExecContext(ctx, q,
		a.ID, a.Username, normalizeEmail(a.Email), a.PasswordHash, boolToInt(a.IsActive),
		database.FormatTime(a.CreatedAt), database.FormatTime(a.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("admin: upsert: %w", err)
	}
	return nil
}

// FindByID loads one admin.
func (r *AdminRepository) FindByID(ctx context.Context, id string) (*models.AdminUser, error) {
	return r.findOne(ctx, `SELECT `+adminColumns+` FROM admin_users WHERE id = ?`, id)
}

// FindByIdentifier loads one admin by username OR email (case-insensitive).
func (r *AdminRepository) FindByIdentifier(ctx context.Context, identifier string) (*models.AdminUser, error) {
	v := strings.TrimSpace(identifier)
	return r.findOne(ctx, `SELECT `+adminColumns+` FROM admin_users
	                       WHERE username = ? COLLATE NOCASE OR email = ? COLLATE NOCASE LIMIT 1`, v, v)
}

// List returns every admin ordered by username.
func (r *AdminRepository) List(ctx context.Context) ([]models.AdminUser, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+adminColumns+` FROM admin_users ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("admin: list: %w", err)
	}
	defer rows.Close()

	var admins []models.AdminUser
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		admins = append(admins, *a)
	}
	return admins, rows.Err()
}

// UpdatePassword replaces the stored argon2id hash.
func (r *AdminRepository) UpdatePassword(ctx context.Context, id, passwordHash string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE admin_users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		passwordHash, database.FormatTime(at), id)
	if err != nil {
		return fmt.Errorf("admin: update password: %w", err)
	}
	return nil
}

// TouchLastLogin records a successful sign-in.
func (r *AdminRepository) TouchLastLogin(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE admin_users SET last_login_at = ?, updated_at = ? WHERE id = ?`,
		database.FormatTime(at), database.FormatTime(at), id)
	if err != nil {
		return fmt.Errorf("admin: touch last login: %w", err)
	}
	return nil
}

// SetActive enables or disables an account (adminctl).
func (r *AdminRepository) SetActive(ctx context.Context, id string, active bool, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE admin_users SET is_active = ?, updated_at = ? WHERE id = ?`,
		boolToInt(active), database.FormatTime(at), id)
	if err != nil {
		return fmt.Errorf("admin: set active: %w", err)
	}
	return nil
}

func (r *AdminRepository) findOne(ctx context.Context, query string, args ...any) (*models.AdminUser, error) {
	a, err := scanAdmin(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, notFound(err)
	}
	return a, nil
}

func scanAdmin(row rowScanner) (*models.AdminUser, error) {
	var (
		a         models.AdminUser
		active    sql.NullInt64
		lastLogin sql.NullString
		createdAt string
		updatedAt string
	)
	if err := row.Scan(&a.ID, &a.Username, &a.Email, &a.PasswordHash, &active, &lastLogin, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("admin: scan: %w", err)
	}
	a.IsActive = boolFromInt(active)

	lastLoginAt, err := database.ParseTimePtr(lastLogin)
	if err != nil {
		return nil, fmt.Errorf("admin: parse last_login_at: %w", err)
	}
	a.LastLoginAt = lastLoginAt

	if a.CreatedAt, err = database.ParseTime(createdAt); err != nil {
		return nil, fmt.Errorf("admin: parse created_at: %w", err)
	}
	if a.UpdatedAt, err = database.ParseTime(updatedAt); err != nil {
		return nil, fmt.Errorf("admin: parse updated_at: %w", err)
	}
	return &a, nil
}

// normalizeEmail stores emails lower-cased so the UNIQUE index and the
// case-insensitive lookup agree.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
