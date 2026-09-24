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

// EducationRepository reads and writes educations. The table has no child rows,
// so a single statement is enough per write and no transaction is needed.
type EducationRepository struct {
	base
}

// NewEducationRepository binds the repository to the database connection.
func NewEducationRepository(db *database.DB) *EducationRepository {
	return &EducationRepository{base{db: db}}
}

const educationColumns = `id, institution, degree, field_of_study, start_year, end_year,
	grade, gpa, coursework, honors, display_order, created_at`

// educationOrder is the listing order used by both the public and the admin
// endpoint, with id as a deterministic tie-breaker.
const educationOrder = ` ORDER BY display_order, id`

// ListEducations returns every education entry in display order.
func (r *EducationRepository) ListEducations(ctx context.Context) ([]models.Education, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+educationColumns+` FROM educations`+educationOrder)
	if err != nil {
		return nil, fmt.Errorf("education: list: %w", err)
	}
	defer rows.Close()

	var educations []models.Education
	for rows.Next() {
		education, err := scanEducation(rows)
		if err != nil {
			return nil, err
		}
		educations = append(educations, *education)
	}
	return educations, rows.Err()
}

// FindEducationByID loads one education entry.
func (r *EducationRepository) FindEducationByID(ctx context.Context, id string) (*models.Education, error) {
	education, err := scanEducation(r.db.QueryRowContext(ctx,
		`SELECT `+educationColumns+` FROM educations WHERE id = ?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return education, nil
}

// CreateEducation inserts a new row.
func (r *EducationRepository) CreateEducation(ctx context.Context, education *models.Education) error {
	coursework, err := encodeCoursework(education.Coursework)
	if err != nil {
		return fmt.Errorf("education: encode coursework: %w", err)
	}

	const insert = `INSERT INTO educations (
		id, institution, degree, field_of_study, start_year, end_year,
		grade, gpa, coursework, honors, display_order, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := r.db.ExecContext(ctx, insert,
		education.ID, education.Institution, education.Degree,
		nullString(education.FieldOfStudy), education.StartYear, nullableInt(education.EndYear),
		nullString(education.Grade), nullString(education.GPA), coursework,
		nullString(education.Honors), education.DisplayOrder,
		database.FormatTime(education.CreatedAt)); err != nil {
		return fmt.Errorf("education: insert: %w", err)
	}
	return nil
}

// UpdateEducation replaces the mutable columns of an existing row. created_at is
// never touched.
func (r *EducationRepository) UpdateEducation(ctx context.Context, education *models.Education) error {
	coursework, err := encodeCoursework(education.Coursework)
	if err != nil {
		return fmt.Errorf("education: encode coursework: %w", err)
	}

	const update = `UPDATE educations SET
		institution = ?, degree = ?, field_of_study = ?, start_year = ?, end_year = ?,
		grade = ?, gpa = ?, coursework = ?, honors = ?, display_order = ?
		WHERE id = ?`
	res, err := r.db.ExecContext(ctx, update,
		education.Institution, education.Degree, nullString(education.FieldOfStudy),
		education.StartYear, nullableInt(education.EndYear), nullString(education.Grade),
		nullString(education.GPA), coursework, nullString(education.Honors),
		education.DisplayOrder, education.ID)
	if err != nil {
		return fmt.Errorf("education: update: %w", err)
	}
	return affectedOrNotFound(res, "education: update")
}

// DeleteEducation removes one row.
func (r *EducationRepository) DeleteEducation(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM educations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("education: delete: %w", err)
	}
	return affectedOrNotFound(res, "education: delete")
}

func scanEducation(row rowScanner) (*models.Education, error) {
	var (
		education   models.Education
		fieldOfStudy sql.NullString
		endYear      sql.NullInt64
		grade        sql.NullString
		gpa          sql.NullString
		coursework   sql.NullString
		honors       sql.NullString
		createdAt    string
	)
	if err := row.Scan(&education.ID, &education.Institution, &education.Degree,
		&fieldOfStudy, &education.StartYear, &endYear, &grade, &gpa, &coursework,
		&honors, &education.DisplayOrder, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("education: scan: %w", err)
	}

	education.FieldOfStudy = fieldOfStudy.String
	education.Grade = grade.String
	education.GPA = gpa.String
	education.Honors = honors.String
	if endYear.Valid {
		education.EndYear = int(endYear.Int64)
	}

	decoded, err := decodeCoursework(coursework)
	if err != nil {
		return nil, fmt.Errorf("education: decode coursework: %w", err)
	}
	education.Coursework = decoded

	if education.CreatedAt, err = database.ParseTime(createdAt); err != nil {
		return nil, fmt.Errorf("education: parse created_at: %w", err)
	}
	return &education, nil
}

// nullableInt turns the zero value into a SQL NULL so an in-progress degree
// keeps end_year empty instead of storing 0.
func nullableInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

// encodeCoursework renders the string slice as a JSON array for the TEXT column.
// An empty list is stored as "[]" so reads never have to special-case it.
func encodeCoursework(values []string) (any, error) {
	if len(values) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

// decodeCoursework reads the JSON array back. NULL, an empty string and a JSON
// null all become an empty slice so the API always answers with [].
func decodeCoursework(value sql.NullString) ([]string, error) {
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