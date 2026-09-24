package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// ProjectRepository reads and writes projects and project_bullets. Every write
// that touches both tables runs inside one transaction, so a project and its
// bullets can never drift apart.
type ProjectRepository struct {
	base
}

// NewProjectRepository binds the repository to the database connection.
func NewProjectRepository(db *database.DB) *ProjectRepository {
	return &ProjectRepository{base{db: db}}
}

const projectColumns = `id, title, tagline, description, project_type, role, technologies,
	repo_url, live_url, image_url, start_date, end_date, status, is_featured, display_order,
	created_at, updated_at`

// projectOrder is the public listing order: featured projects first, then
// display_order, with id as a deterministic tie-breaker.
const projectOrder = ` ORDER BY is_featured DESC, display_order, id`

// ListProjects returns every project in featured-then-order sequence.
func (r *ProjectRepository) ListProjects(ctx context.Context) ([]models.Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+projectColumns+` FROM projects`+projectOrder)
	if err != nil {
		return nil, fmt.Errorf("project: list: %w", err)
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *project)
	}
	return projects, rows.Err()
}

// FindProjectByID loads one project.
func (r *ProjectRepository) FindProjectByID(ctx context.Context, id string) (*models.Project, error) {
	project, err := scanProject(r.db.QueryRowContext(ctx,
		`SELECT `+projectColumns+` FROM projects WHERE id = ?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return project, nil
}

// ListProjectBullets returns the bullets of one project, or the bullets of every
// project in the same order the projects are listed when projectID is empty.
func (r *ProjectRepository) ListProjectBullets(ctx context.Context, projectID string) ([]models.ProjectBullet, error) {
	if projectID != "" {
		return r.queryProjectBullets(ctx,
			`SELECT id, project_id, bullet_point, display_order
			 FROM project_bullets WHERE project_id = ? ORDER BY display_order, id`,
			projectID)
	}
	return r.queryProjectBullets(ctx,
		`SELECT b.id, b.project_id, b.bullet_point, b.display_order
		 FROM project_bullets b JOIN projects p ON p.id = b.project_id
		 ORDER BY p.is_featured DESC, p.display_order, p.id, b.display_order, b.id`)
}

// SaveProject writes the project row and replaces its bullets in a single
// transaction. The row is updated when the ID already exists and inserted
// otherwise, so the service does not have to decide between create and update.
// created_at is never touched on the update path.
func (r *ProjectRepository) SaveProject(ctx context.Context, project *models.Project, bullets []models.ProjectBullet) error {
	technologies, err := encodeTechnologies(project.Technologies)
	if err != nil {
		return fmt.Errorf("project: encode technologies: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("project: save: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const update = `UPDATE projects SET
		title = ?, tagline = ?, description = ?, project_type = ?, role = ?,
		technologies = ?, repo_url = ?, live_url = ?, image_url = ?,
		start_date = ?, end_date = ?, status = ?, is_featured = ?, display_order = ?, updated_at = ?
		WHERE id = ?`
	res, err := tx.ExecContext(ctx, update,
		project.Title, nullString(project.Tagline), project.Description,
		nullString(project.ProjectType), nullString(project.Role), technologies,
		nullString(project.RepoURL), nullString(project.LiveURL), nullString(project.ImageURL),
		nullString(project.StartDate), nullString(project.EndDate), nullString(project.Status),
		boolToInt(project.IsFeatured), project.DisplayOrder, database.FormatTime(project.UpdatedAt),
		project.ID)
	if err != nil {
		return fmt.Errorf("project: update: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("project: update: rows affected: %w", err)
	}

	if affected == 0 {
		const insert = `INSERT INTO projects (
			id, title, tagline, description, project_type, role, technologies,
			repo_url, live_url, image_url, start_date, end_date, status,
			is_featured, display_order, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, insert,
			project.ID, project.Title, nullString(project.Tagline), project.Description,
			nullString(project.ProjectType), nullString(project.Role), technologies,
			nullString(project.RepoURL), nullString(project.LiveURL), nullString(project.ImageURL),
			nullString(project.StartDate), nullString(project.EndDate), nullString(project.Status),
			boolToInt(project.IsFeatured), project.DisplayOrder,
			database.FormatTime(project.CreatedAt), database.FormatTime(project.UpdatedAt)); err != nil {
			return fmt.Errorf("project: insert: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM project_bullets WHERE project_id = ?`, project.ID); err != nil {
		return fmt.Errorf("project: clear bullets: %w", err)
	}

	const insertBullet = `INSERT INTO project_bullets (id, project_id, bullet_point, display_order)
	                      VALUES (?, ?, ?, ?)`
	for i := range bullets {
		if _, err := tx.ExecContext(ctx, insertBullet,
			bullets[i].ID, project.ID, bullets[i].Text, bullets[i].DisplayOrder); err != nil {
			return fmt.Errorf("project: insert bullet: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("project: save: commit: %w", err)
	}
	return nil
}

// DeleteProject removes the row together with its bullets in one transaction.
// The schema declares ON DELETE CASCADE, but foreign key enforcement is a
// per-connection pragma, so the child rows are deleted explicitly to keep the
// behaviour deterministic.
func (r *ProjectRepository) DeleteProject(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("project: delete: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("project: delete: %w", err)
	}
	if err := affectedOrNotFound(res, "project: delete"); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM project_bullets WHERE project_id = ?`, id); err != nil {
		return fmt.Errorf("project: delete bullets: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("project: delete: commit: %w", err)
	}
	return nil
}

func (r *ProjectRepository) queryProjectBullets(ctx context.Context, query string, args ...any) ([]models.ProjectBullet, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("project: list bullets: %w", err)
	}
	defer rows.Close()

	var bullets []models.ProjectBullet
	for rows.Next() {
		bullet, err := scanProjectBullet(rows)
		if err != nil {
			return nil, err
		}
		bullets = append(bullets, *bullet)
	}
	return bullets, rows.Err()
}

func scanProject(row rowScanner) (*models.Project, error) {
	var (
		project      models.Project
		tagline      sql.NullString
		projectType  sql.NullString
		role         sql.NullString
		technologies sql.NullString
		repoURL      sql.NullString
		liveURL      sql.NullString
		imageURL     sql.NullString
		startDate    sql.NullString
		endDate      sql.NullString
		status       sql.NullString
		isFeatured   sql.NullInt64
		createdAt    string
		updatedAt    string
	)
	if err := row.Scan(&project.ID, &project.Title, &tagline, &project.Description,
		&projectType, &role, &technologies, &repoURL, &liveURL, &imageURL,
		&startDate, &endDate, &status, &isFeatured, &project.DisplayOrder,
		&createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("project: scan: %w", err)
	}

	project.Tagline = tagline.String
	project.ProjectType = projectType.String
	project.Role = role.String
	project.RepoURL = repoURL.String
	project.LiveURL = liveURL.String
	project.ImageURL = imageURL.String
	project.StartDate = startDate.String
	project.EndDate = endDate.String
	project.Status = status.String
	project.IsFeatured = boolFromInt(isFeatured)

	decoded, err := decodeTechnologies(technologies)
	if err != nil {
		return nil, fmt.Errorf("project: decode technologies: %w", err)
	}
	project.Technologies = decoded

	if project.CreatedAt, err = database.ParseTime(createdAt); err != nil {
		return nil, fmt.Errorf("project: parse created_at: %w", err)
	}
	if project.UpdatedAt, err = database.ParseTime(updatedAt); err != nil {
		return nil, fmt.Errorf("project: parse updated_at: %w", err)
	}
	return &project, nil
}

func scanProjectBullet(row rowScanner) (*models.ProjectBullet, error) {
	var bullet models.ProjectBullet
	if err := row.Scan(&bullet.ID, &bullet.ProjectID, &bullet.Text, &bullet.DisplayOrder); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("project: scan bullet: %w", err)
	}
	return &bullet, nil
}


