package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// ExtraRepository reads and writes extras. The table has no child rows, so a
// single statement is enough per write and no transaction is needed.
type ExtraRepository struct {
	base
}

// NewExtraRepository binds the repository to the database connection.
func NewExtraRepository(db *database.DB) *ExtraRepository {
	return &ExtraRepository{base{db: db}}
}

const extraColumns = `id, category, title, issuer, issued_date, credential_url,
	description, display_order, created_at`

// extraOrder is the listing order used by the admin endpoint. The public
// endpoint re-groups the same rows by category.
const extraOrder = ` ORDER BY category, display_order, id`

// ListExtras returns every extra entry ordered by category then display order.
func (r *ExtraRepository) ListExtras(ctx context.Context) ([]models.Extra, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+extraColumns+` FROM extras`+extraOrder)
	if err != nil {
		return nil, fmt.Errorf("extra: list: %w", err)
	}
	defer rows.Close()

	var extras []models.Extra
	for rows.Next() {
		extra, err := scanExtra(rows)
		if err != nil {
			return nil, err
		}
		extras = append(extras, *extra)
	}
	return extras, rows.Err()
}

// FindExtraByID loads one extra entry.
func (r *ExtraRepository) FindExtraByID(ctx context.Context, id string) (*models.Extra, error) {
	extra, err := scanExtra(r.db.QueryRowContext(ctx,
		`SELECT `+extraColumns+` FROM extras WHERE id = ?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return extra, nil
}

// CreateExtra inserts a new row.
func (r *ExtraRepository) CreateExtra(ctx context.Context, extra *models.Extra) error {
	const insert = `INSERT INTO extras (
		id, category, title, issuer, issued_date, credential_url,
		description, display_order, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := r.db.ExecContext(ctx, insert,
		extra.ID, extra.Category, extra.Title, nullString(extra.Issuer),
		nullString(extra.IssuedDate), nullString(extra.CredentialURL),
		nullString(extra.Description), extra.DisplayOrder,
		database.FormatTime(extra.CreatedAt)); err != nil {
		return fmt.Errorf("extra: insert: %w", err)
	}
	return nil
}

// UpdateExtra replaces the mutable columns of an existing row. created_at is
// never touched, since the table has no updated_at column.
func (r *ExtraRepository) UpdateExtra(ctx context.Context, extra *models.Extra) error {
	const update = `UPDATE extras SET
		category = ?, title = ?, issuer = ?, issued_date = ?, credential_url = ?,
		description = ?, display_order = ?
		WHERE id = ?`
	res, err := r.db.ExecContext(ctx, update,
		extra.Category, extra.Title, nullString(extra.Issuer),
		nullString(extra.IssuedDate), nullString(extra.CredentialURL),
		nullString(extra.Description), extra.DisplayOrder, extra.ID)
	if err != nil {
		return fmt.Errorf("extra: update: %w", err)
	}
	return affectedOrNotFound(res, "extra: update")
}

// DeleteExtra removes one row.
func (r *ExtraRepository) DeleteExtra(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM extras WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("extra: delete: %w", err)
	}
	return affectedOrNotFound(res, "extra: delete")
}

func scanExtra(row rowScanner) (*models.Extra, error) {
	var (
		extra         models.Extra
		issuer        sql.NullString
		issuedDate    sql.NullString
		credentialURL sql.NullString
		description   sql.NullString
		createdAt     string
	)
	if err := row.Scan(&extra.ID, &extra.Category, &extra.Title, &issuer,
		&issuedDate, &credentialURL, &description, &extra.DisplayOrder,
		&createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("extra: scan: %w", err)
	}

	extra.Issuer = issuer.String
	extra.IssuedDate = issuedDate.String
	extra.CredentialURL = credentialURL.String
	extra.Description = description.String

	parsedAt, err := database.ParseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("extra: parse created_at: %w", err)
	}
	extra.CreatedAt = parsedAt
	return &extra, nil
}