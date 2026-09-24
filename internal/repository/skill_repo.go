package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// SkillRepository reads and writes skill_categories and skills.
type SkillRepository struct {
	base
}

// NewSkillRepository binds the repository to the database connection.
func NewSkillRepository(db *database.DB) *SkillRepository {
	return &SkillRepository{base{db: db}}
}

const skillCategoryColumns = `id, name, display_order, created_at`

const skillColumns = `id, category_id, skill_name, icon_slug, display_order, created_at`

// ListCategories returns every category in display order.
func (r *SkillRepository) ListCategories(ctx context.Context) ([]models.SkillCategory, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+skillCategoryColumns+` FROM skill_categories ORDER BY display_order, name`)
	if err != nil {
		return nil, fmt.Errorf("skill: list categories: %w", err)
	}
	defer rows.Close()

	var categories []models.SkillCategory
	for rows.Next() {
		category, err := scanSkillCategory(rows)
		if err != nil {
			return nil, err
		}
		categories = append(categories, *category)
	}
	return categories, rows.Err()
}

// FindCategoryByID loads one category.
func (r *SkillRepository) FindCategoryByID(ctx context.Context, id string) (*models.SkillCategory, error) {
	category, err := scanSkillCategory(r.db.QueryRowContext(ctx,
		`SELECT `+skillCategoryColumns+` FROM skill_categories WHERE id = ?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return category, nil
}

// FindCategoryByName loads one category by name. The lookup is case-insensitive
// so the API can reject confusing near-duplicates ("Backend" vs "backend") even
// though the UNIQUE index compares bytes.
func (r *SkillRepository) FindCategoryByName(ctx context.Context, name string) (*models.SkillCategory, error) {
	category, err := scanSkillCategory(r.db.QueryRowContext(ctx,
		`SELECT `+skillCategoryColumns+` FROM skill_categories WHERE name = ? COLLATE NOCASE LIMIT 1`, name))
	if err != nil {
		return nil, notFound(err)
	}
	return category, nil
}

// CreateCategory inserts a category.
func (r *SkillRepository) CreateCategory(ctx context.Context, category *models.SkillCategory) error {
	const q = `INSERT INTO skill_categories (id, name, display_order, created_at) VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		category.ID, category.Name, category.DisplayOrder, database.FormatTime(category.CreatedAt))
	if err != nil {
		return conflictOr("skill: create category", err)
	}
	return nil
}

// UpdateCategory renames and/or reorders an existing category.
func (r *SkillRepository) UpdateCategory(ctx context.Context, category *models.SkillCategory) error {
	const q = `UPDATE skill_categories SET name = ?, display_order = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, q, category.Name, category.DisplayOrder, category.ID)
	if err != nil {
		return conflictOr("skill: update category", err)
	}
	return affectedOrNotFound(res, "skill: update category")
}

// DeleteCategory removes a category together with its skills. The schema
// declares ON DELETE CASCADE, but foreign key enforcement is a per-connection
// pragma, so the child rows are deleted explicitly inside one transaction to
// keep the behaviour deterministic and never leave orphan skills behind.
func (r *SkillRepository) DeleteCategory(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("skill: delete category: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `DELETE FROM skill_categories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("skill: delete category: %w", err)
	}
	if err := affectedOrNotFound(res, "skill: delete category"); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM skills WHERE category_id = ?`, id); err != nil {
		return fmt.Errorf("skill: delete category skills: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("skill: delete category: commit: %w", err)
	}
	return nil
}

// ListSkills returns the skills of one category, or the skills of every
// category (grouped by category display order) when categoryID is empty.
func (r *SkillRepository) ListSkills(ctx context.Context, categoryID string) ([]models.Skill, error) {
	if categoryID != "" {
		return r.querySkills(ctx,
			`SELECT `+skillColumns+` FROM skills WHERE category_id = ? ORDER BY display_order, skill_name`,
			categoryID)
	}
	return r.querySkills(ctx,
		`SELECT s.id, s.category_id, s.skill_name, s.icon_slug, s.display_order, s.created_at
		 FROM skills s JOIN skill_categories c ON c.id = s.category_id
		 ORDER BY c.display_order, c.name, s.display_order, s.skill_name`)
}

// FindSkillByID loads one skill.
func (r *SkillRepository) FindSkillByID(ctx context.Context, id string) (*models.Skill, error) {
	skill, err := scanSkill(r.db.QueryRowContext(ctx,
		`SELECT `+skillColumns+` FROM skills WHERE id = ?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return skill, nil
}

// FindSkillByName loads one skill inside a category. Like the category lookup
// this is case-insensitive on purpose.
func (r *SkillRepository) FindSkillByName(ctx context.Context, categoryID, name string) (*models.Skill, error) {
	skill, err := scanSkill(r.db.QueryRowContext(ctx,
		`SELECT `+skillColumns+` FROM skills WHERE category_id = ? AND skill_name = ? COLLATE NOCASE LIMIT 1`,
		categoryID, name))
	if err != nil {
		return nil, notFound(err)
	}
	return skill, nil
}

// CreateSkill inserts a skill.
func (r *SkillRepository) CreateSkill(ctx context.Context, skill *models.Skill) error {
	const q = `INSERT INTO skills (id, category_id, skill_name, icon_slug, display_order, created_at)
	           VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, skill.ID, skill.CategoryID, skill.Name,
		nullString(skill.IconSlug), skill.DisplayOrder, database.FormatTime(skill.CreatedAt))
	if err != nil {
		return conflictOr("skill: create", err)
	}
	return nil
}

// UpdateSkill updates name, icon, order and (optionally) the owning category.
func (r *SkillRepository) UpdateSkill(ctx context.Context, skill *models.Skill) error {
	const q = `UPDATE skills SET category_id = ?, skill_name = ?, icon_slug = ?, display_order = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, q, skill.CategoryID, skill.Name,
		nullString(skill.IconSlug), skill.DisplayOrder, skill.ID)
	if err != nil {
		return conflictOr("skill: update", err)
	}
	return affectedOrNotFound(res, "skill: update")
}

// DeleteSkill removes one skill.
func (r *SkillRepository) DeleteSkill(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM skills WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("skill: delete: %w", err)
	}
	return affectedOrNotFound(res, "skill: delete")
}

func (r *SkillRepository) querySkills(ctx context.Context, query string, args ...any) ([]models.Skill, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("skill: list: %w", err)
	}
	defer rows.Close()

	var skills []models.Skill
	for rows.Next() {
		skill, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, *skill)
	}
	return skills, rows.Err()
}

func scanSkillCategory(row rowScanner) (*models.SkillCategory, error) {
	var (
		category  models.SkillCategory
		createdAt string
	)
	if err := row.Scan(&category.ID, &category.Name, &category.DisplayOrder, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("skill: scan category: %w", err)
	}

	var err error
	if category.CreatedAt, err = database.ParseTime(createdAt); err != nil {
		return nil, fmt.Errorf("skill: parse category created_at: %w", err)
	}
	return &category, nil
}

func scanSkill(row rowScanner) (*models.Skill, error) {
	var (
		skill     models.Skill
		iconSlug  sql.NullString
		createdAt string
	)
	if err := row.Scan(&skill.ID, &skill.CategoryID, &skill.Name, &iconSlug,
		&skill.DisplayOrder, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("skill: scan: %w", err)
	}

	skill.IconSlug = iconSlug.String

	var err error
	if skill.CreatedAt, err = database.ParseTime(createdAt); err != nil {
		return nil, fmt.Errorf("skill: parse created_at: %w", err)
	}
	return &skill, nil
}
