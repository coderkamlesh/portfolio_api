package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// ExperienceRepository reads and writes work_experiences and
// experience_bullets. Every write that touches both tables runs inside one
// transaction, so an experience and its bullets can never drift apart.
type ExperienceRepository struct {
	base
}

// NewExperienceRepository binds the repository to the database connection.
func NewExperienceRepository(db *database.DB) *ExperienceRepository {
	return &ExperienceRepository{base{db: db}}
}

const experienceColumns = `id, company_name, company_logo_url, role, employment_type, location,
	start_date, end_date, is_current, technologies, display_order, created_at, updated_at`

// experienceOrder is reverse chronological: current roles first, then the most
// recent start date, with display_order and company name as deterministic
// tie-breakers.
const experienceOrder = ` ORDER BY is_current DESC, start_date DESC, display_order, company_name`

// ListExperiences returns every experience in reverse chronological order.
func (r *ExperienceRepository) ListExperiences(ctx context.Context) ([]models.WorkExperience, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+experienceColumns+` FROM work_experiences`+experienceOrder)
	if err != nil {
		return nil, fmt.Errorf("experience: list: %w", err)
	}
	defer rows.Close()

	var experiences []models.WorkExperience
	for rows.Next() {
		experience, err := scanExperience(rows)
		if err != nil {
			return nil, err
		}
		experiences = append(experiences, *experience)
	}
	return experiences, rows.Err()
}

// FindExperienceByID loads one experience.
func (r *ExperienceRepository) FindExperienceByID(ctx context.Context, id string) (*models.WorkExperience, error) {
	experience, err := scanExperience(r.db.QueryRowContext(ctx,
		`SELECT `+experienceColumns+` FROM work_experiences WHERE id = ?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return experience, nil
}

// ListBullets returns the bullets of one experience, or the bullets of every
// experience in that experience's display order when experienceID is empty.
func (r *ExperienceRepository) ListBullets(ctx context.Context, experienceID string) ([]models.ExperienceBullet, error) {
	if experienceID != "" {
		return r.queryBullets(ctx,
			`SELECT id, experience_id, bullet_point, display_order
			 FROM experience_bullets WHERE experience_id = ? ORDER BY display_order, id`,
			experienceID)
	}
	return r.queryBullets(ctx,
		`SELECT b.id, b.experience_id, b.bullet_point, b.display_order
		 FROM experience_bullets b JOIN work_experiences e ON e.id = b.experience_id
		 ORDER BY e.is_current DESC, e.start_date DESC, e.display_order, e.company_name,
		          b.display_order, b.id`)
}

// SaveExperience writes the experience row and replaces its bullets in a single
// transaction. The row is updated when the ID already exists and inserted
// otherwise, so the service does not have to decide between create and update.
// created_at is never touched on the update path.
func (r *ExperienceRepository) SaveExperience(ctx context.Context, experience *models.WorkExperience, bullets []models.ExperienceBullet) error {
	technologies, err := encodeTechnologies(experience.Technologies)
	if err != nil {
		return fmt.Errorf("experience: encode technologies: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("experience: save: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const update = `UPDATE work_experiences SET
		company_name = ?, company_logo_url = ?, role = ?, employment_type = ?, location = ?,
		start_date = ?, end_date = ?, is_current = ?, technologies = ?, display_order = ?, updated_at = ?
		WHERE id = ?`
	res, err := tx.ExecContext(ctx, update,
		experience.CompanyName, nullString(experience.CompanyLogoURL), experience.Role,
		nullString(experience.EmploymentType), nullString(experience.Location),
		experience.StartDate, nullString(experience.EndDate), boolToInt(experience.IsCurrent),
		technologies, experience.DisplayOrder, database.FormatTime(experience.UpdatedAt),
		experience.ID)
	if err != nil {
		return fmt.Errorf("experience: update: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("experience: update: rows affected: %w", err)
	}

	if affected == 0 {
		const insert = `INSERT INTO work_experiences (
			id, company_name, company_logo_url, role, employment_type, location,
			start_date, end_date, is_current, technologies, display_order, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, insert,
			experience.ID, experience.CompanyName, nullString(experience.CompanyLogoURL),
			experience.Role, nullString(experience.EmploymentType), nullString(experience.Location),
			experience.StartDate, nullString(experience.EndDate), boolToInt(experience.IsCurrent),
			technologies, experience.DisplayOrder, database.FormatTime(experience.CreatedAt),
			database.FormatTime(experience.UpdatedAt)); err != nil {
			return fmt.Errorf("experience: insert: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM experience_bullets WHERE experience_id = ?`, experience.ID); err != nil {
		return fmt.Errorf("experience: clear bullets: %w", err)
	}

	const insertBullet = `INSERT INTO experience_bullets (id, experience_id, bullet_point, display_order)
	                      VALUES (?, ?, ?, ?)`
	for i := range bullets {
		if _, err := tx.ExecContext(ctx, insertBullet,
			bullets[i].ID, experience.ID, bullets[i].Text, bullets[i].DisplayOrder); err != nil {
			return fmt.Errorf("experience: insert bullet: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("experience: save: commit: %w", err)
	}
	return nil
}

// DeleteExperience removes the row together with its bullets in one
// transaction. The schema declares ON DELETE CASCADE, but foreign key
// enforcement is a per-connection pragma, so the child rows are deleted
// explicitly to keep the behaviour deterministic.
func (r *ExperienceRepository) DeleteExperience(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("experience: delete: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `DELETE FROM work_experiences WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("experience: delete: %w", err)
	}
	if err := affectedOrNotFound(res, "experience: delete"); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM experience_bullets WHERE experience_id = ?`, id); err != nil {
		return fmt.Errorf("experience: delete bullets: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("experience: delete: commit: %w", err)
	}
	return nil
}

func (r *ExperienceRepository) queryBullets(ctx context.Context, query string, args ...any) ([]models.ExperienceBullet, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("experience: list bullets: %w", err)
	}
	defer rows.Close()

	var bullets []models.ExperienceBullet
	for rows.Next() {
		bullet, err := scanBullet(rows)
		if err != nil {
			return nil, err
		}
		bullets = append(bullets, *bullet)
	}
	return bullets, rows.Err()
}

func scanExperience(row rowScanner) (*models.WorkExperience, error) {
	var (
		experience     models.WorkExperience
		logoURL        sql.NullString
		employmentType sql.NullString
		location       sql.NullString
		endDate        sql.NullString
		isCurrent      sql.NullInt64
		technologies   sql.NullString
		createdAt      string
		updatedAt      string
	)
	if err := row.Scan(&experience.ID, &experience.CompanyName, &logoURL, &experience.Role,
		&employmentType, &location, &experience.StartDate, &endDate, &isCurrent, &technologies,
		&experience.DisplayOrder, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("experience: scan: %w", err)
	}

	experience.CompanyLogoURL = logoURL.String
	experience.EmploymentType = employmentType.String
	experience.Location = location.String
	experience.EndDate = endDate.String
	experience.IsCurrent = boolFromInt(isCurrent)

	decoded, err := decodeTechnologies(technologies)
	if err != nil {
		return nil, fmt.Errorf("experience: decode technologies: %w", err)
	}
	experience.Technologies = decoded

	if experience.CreatedAt, err = database.ParseTime(createdAt); err != nil {
		return nil, fmt.Errorf("experience: parse created_at: %w", err)
	}
	if experience.UpdatedAt, err = database.ParseTime(updatedAt); err != nil {
		return nil, fmt.Errorf("experience: parse updated_at: %w", err)
	}
	return &experience, nil
}

func scanBullet(row rowScanner) (*models.ExperienceBullet, error) {
	var bullet models.ExperienceBullet
	if err := row.Scan(&bullet.ID, &bullet.ExperienceID, &bullet.Text, &bullet.DisplayOrder); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("experience: scan bullet: %w", err)
	}
	return &bullet, nil
}

// encodeTechnologies renders the string slice as a JSON array for the TEXT
// column. An empty list is stored as "[]" rather than SQL NULL so reads never
// have to special-case it.
func encodeTechnologies(values []string) (any, error) {
	if len(values) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

// decodeTechnologies reads the JSON array back. NULL, an empty string and a
// JSON null all become an empty slice so the API always answers with [].
func decodeTechnologies(value sql.NullString) ([]string, error) {
	if !value.Valid {
		return []string{}, nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" || trimmed == "null" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}

