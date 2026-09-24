package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
)

// SkillDeps wires the skills service to persistence. Now is injectable for
// deterministic tests; it defaults to UTC time.
type SkillDeps struct {
	Skills SkillStore
	Now    func() time.Time
	// Audit records content changes for the admin trail. Optional.
	Audit *Audit
}

// SkillService owns the skills and skill-category use-cases.
type SkillService struct {
	skills SkillStore
	now    func() time.Time
	audit  *Audit
}

// NewSkillService builds the service.
func NewSkillService(deps SkillDeps) *SkillService {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &SkillService{skills: deps.Skills, now: now, audit: deps.Audit}
}

// Input limits. db_schema.sql keeps these columns as unbounded TEXT, so the caps
// live here: long enough for real labels, short enough that nothing absurd
// reaches the database or the future PDF renderer.
const (
	maxCategoryNameLen = 100
	maxSkillNameLen    = 100
	maxIconSlugLen     = 64
)

// CategoryInput is the create/update payload of a skill category. A nil
// DisplayOrder keeps the stored order on update and defaults to 0 on create, so
// a rename never silently reorders a category.
type CategoryInput struct {
	Name         string
	DisplayOrder *int
}

// SkillInput is the create/update payload of a skill. CategoryID is required on
// create; on update it moves the skill when set and keeps the current category
// when empty. IconSlug is optional — an empty value clears it.
type SkillInput struct {
	CategoryID   string
	Name         string
	IconSlug     string
	DisplayOrder *int
}

// SkillView is one skill inside a category.
type SkillView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IconSlug     string `json:"icon_slug,omitempty"`
	DisplayOrder int    `json:"display_order"`
}

// SkillCategoryView groups the skills of one category.
type SkillCategoryView struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	DisplayOrder int         `json:"display_order"`
	Skills       []SkillView `json:"skills"`
}

// SkillsView is the payload of GET /api/public/skills.
type SkillsView struct {
	Categories []SkillCategoryView `json:"categories"`
}

// AdminSkillCategoryView is one category as returned to the admin panel. Its
// skills are listed separately through GET /api/admin/skills, so this view
// carries no nested array.
type AdminSkillCategoryView struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	DisplayOrder int       `json:"display_order"`
	CreatedAt    time.Time `json:"created_at"`
}

// AdminSkillView adds the owning category and persistence metadata.
type AdminSkillView struct {
	SkillView
	CategoryID string    `json:"category_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdminSkillCategoriesView is the payload of GET /api/admin/skill-categories.
type AdminSkillCategoriesView struct {
	Categories []AdminSkillCategoryView `json:"categories"`
}

// AdminSkillsView is the payload of GET /api/admin/skills.
type AdminSkillsView struct {
	Skills []AdminSkillView `json:"skills"`
}

// PublicSkills returns every category with its skills nested, ordered for the
// public portfolio page.
func (s *SkillService) PublicSkills(ctx context.Context) (*SkillsView, error) {
	categories, err := s.skills.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	all, err := s.skills.ListSkills(ctx, "")
	if err != nil {
		return nil, err
	}

	grouped := make(map[string][]SkillView, len(categories))
	for i := range all {
		grouped[all[i].CategoryID] = append(grouped[all[i].CategoryID], skillView(&all[i]))
	}

	view := &SkillsView{Categories: make([]SkillCategoryView, 0, len(categories))}
	for i := range categories {
		skills := grouped[categories[i].ID]
		if skills == nil {
			// Keep the JSON stable: a category without skills renders as [].
			skills = []SkillView{}
		}
		view.Categories = append(view.Categories, SkillCategoryView{
			ID:           categories[i].ID,
			Name:         categories[i].Name,
			DisplayOrder: categories[i].DisplayOrder,
			Skills:       skills,
		})
	}
	return view, nil
}

// AdminCategories returns every category as a flat list with metadata.
func (s *SkillService) AdminCategories(ctx context.Context) (*AdminSkillCategoriesView, error) {
	categories, err := s.skills.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	view := &AdminSkillCategoriesView{Categories: make([]AdminSkillCategoryView, 0, len(categories))}
	for i := range categories {
		view.Categories = append(view.Categories, *adminSkillCategoryView(&categories[i]))
	}
	return view, nil
}

// CreateCategory adds a category. Names are compared case-insensitively.
func (s *SkillService) CreateCategory(ctx context.Context, in CategoryInput) (*AdminSkillCategoryView, error) {
	name := strings.TrimSpace(in.Name)
	if err := validateText("name", name, maxCategoryNameLen); err != nil {
		return nil, err
	}
	if err := validateDisplayOrder(in.DisplayOrder); err != nil {
		return nil, err
	}

	existing, err := s.skills.FindCategoryByName(ctx, name)
	switch {
	case err == nil:
		return nil, errSkillCategoryExists(existing.Name)
	case !errors.Is(err, models.ErrNotFound):
		return nil, err
	}

	category := &models.SkillCategory{
		ID:           ids.New(),
		Name:         name,
		DisplayOrder: displayOrderOrDefault(in.DisplayOrder, 0),
		CreatedAt:    s.now(),
	}
	if err := s.skills.CreateCategory(ctx, category); err != nil {
		return nil, mapCategoryWriteErr(err, name)
	}
	s.audit.Record(ctx, auditEntitySkillCategory, category.ID, AuditActionCreate, nil, category)
	return adminSkillCategoryView(category), nil
}

// UpdateCategory renames and/or reorders a category.
func (s *SkillService) UpdateCategory(ctx context.Context, id string, in CategoryInput) (*AdminSkillCategoryView, error) {
	name := strings.TrimSpace(in.Name)
	if err := validateText("name", name, maxCategoryNameLen); err != nil {
		return nil, err
	}
	if err := validateDisplayOrder(in.DisplayOrder); err != nil {
		return nil, err
	}

	category, err := s.findCategory(ctx, id)
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(category.Name, name) {
		existing, err := s.skills.FindCategoryByName(ctx, name)
		switch {
		case err == nil && existing.ID != category.ID:
			return nil, errSkillCategoryExists(existing.Name)
		case err != nil && !errors.Is(err, models.ErrNotFound):
			return nil, err
		}
	}

	// The row is mutated in place below, so the snapshot has to be taken first.
	previous := *category

	category.Name = name
	category.DisplayOrder = displayOrderOrDefault(in.DisplayOrder, category.DisplayOrder)

	if err := s.skills.UpdateCategory(ctx, category); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errSkillCategoryNotFound()
		}
		return nil, mapCategoryWriteErr(err, name)
	}
	s.audit.Record(ctx, auditEntitySkillCategory, category.ID, AuditActionUpdate, &previous, category)
	return adminSkillCategoryView(category), nil
}

// DeleteCategory removes a category and every skill inside it. The children are
// read before the delete so the audit trail keeps a snapshot of them too: without
// that, deleting a category would erase its skills from the record entirely.
func (s *SkillService) DeleteCategory(ctx context.Context, id string) error {
	category, err := s.findCategory(ctx, id)
	if err != nil {
		return err
	}
	children, _ := s.skills.ListSkills(ctx, id)

	if err := s.skills.DeleteCategory(ctx, id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return errSkillCategoryNotFound()
		}
		return err
	}
	s.audit.Record(ctx, auditEntitySkillCategory, id, AuditActionDelete, category, nil)
	for i := range children {
		s.audit.Record(ctx, auditEntitySkill, children[i].ID, AuditActionDelete, &children[i], nil)
	}
	return nil
}

// AdminSkills returns every skill as a flat list ordered by category.
func (s *SkillService) AdminSkills(ctx context.Context) (*AdminSkillsView, error) {
	skills, err := s.skills.ListSkills(ctx, "")
	if err != nil {
		return nil, err
	}
	view := &AdminSkillsView{Skills: make([]AdminSkillView, 0, len(skills))}
	for i := range skills {
		view.Skills = append(view.Skills, *adminSkillView(&skills[i]))
	}
	return view, nil
}

// CreateSkill adds a skill to an existing category.
func (s *SkillService) CreateSkill(ctx context.Context, in SkillInput) (*AdminSkillView, error) {
	categoryID := strings.TrimSpace(in.CategoryID)
	if categoryID == "" {
		return nil, errSkillValidation("category_id")
	}
	name := strings.TrimSpace(in.Name)
	if err := validateText("name", name, maxSkillNameLen); err != nil {
		return nil, err
	}
	iconSlug := strings.TrimSpace(in.IconSlug)
	if err := validateOptionalText("icon_slug", iconSlug, maxIconSlugLen); err != nil {
		return nil, err
	}
	if err := validateDisplayOrder(in.DisplayOrder); err != nil {
		return nil, err
	}
	if _, err := s.findCategory(ctx, categoryID); err != nil {
		return nil, err
	}
	if err := s.ensureSkillNameFree(ctx, categoryID, name, ""); err != nil {
		return nil, err
	}

	skill := &models.Skill{
		ID:           ids.New(),
		CategoryID:   categoryID,
		Name:         name,
		IconSlug:     iconSlug,
		DisplayOrder: displayOrderOrDefault(in.DisplayOrder, 0),
		CreatedAt:    s.now(),
	}
	if err := s.skills.CreateSkill(ctx, skill); err != nil {
		return nil, mapSkillWriteErr(err, name)
	}
	s.audit.Record(ctx, auditEntitySkill, skill.ID, AuditActionCreate, nil, skill)
	return adminSkillView(skill), nil
}

// UpdateSkill renames, reorders, re-icons and optionally moves a skill to
// another category.
func (s *SkillService) UpdateSkill(ctx context.Context, id string, in SkillInput) (*AdminSkillView, error) {
	skill, err := s.findSkill(ctx, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if err := validateText("name", name, maxSkillNameLen); err != nil {
		return nil, err
	}
	iconSlug := strings.TrimSpace(in.IconSlug)
	if err := validateOptionalText("icon_slug", iconSlug, maxIconSlugLen); err != nil {
		return nil, err
	}
	if err := validateDisplayOrder(in.DisplayOrder); err != nil {
		return nil, err
	}

	categoryID := strings.TrimSpace(in.CategoryID)
	if categoryID == "" {
		categoryID = skill.CategoryID
	} else if categoryID != skill.CategoryID {
		if _, err := s.findCategory(ctx, categoryID); err != nil {
			return nil, err
		}
	}
	if err := s.ensureSkillNameFree(ctx, categoryID, name, skill.ID); err != nil {
		return nil, err
	}

	// The row is mutated in place below, so the snapshot has to be taken first.
	previous := *skill

	skill.CategoryID = categoryID
	skill.Name = name
	skill.IconSlug = iconSlug
	skill.DisplayOrder = displayOrderOrDefault(in.DisplayOrder, skill.DisplayOrder)

	if err := s.skills.UpdateSkill(ctx, skill); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errSkillNotFound()
		}
		return nil, mapSkillWriteErr(err, name)
	}
	s.audit.Record(ctx, auditEntitySkill, skill.ID, AuditActionUpdate, &previous, skill)
	return adminSkillView(skill), nil
}

// DeleteSkill removes one skill.
func (s *SkillService) DeleteSkill(ctx context.Context, id string) error {
	existing, err := s.findSkill(ctx, id)
	if err != nil {
		return err
	}
	if err := s.skills.DeleteSkill(ctx, id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return errSkillNotFound()
		}
		return err
	}
	s.audit.Record(ctx, auditEntitySkill, id, AuditActionDelete, existing, nil)
	return nil
}

func (s *SkillService) findCategory(ctx context.Context, id string) (*models.SkillCategory, error) {
	category, err := s.skills.FindCategoryByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errSkillCategoryNotFound()
		}
		return nil, err
	}
	return category, nil
}

func (s *SkillService) findSkill(ctx context.Context, id string) (*models.Skill, error) {
	skill, err := s.skills.FindSkillByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errSkillNotFound()
		}
		return nil, err
	}
	return skill, nil
}

// ensureSkillNameFree rejects a name already used inside the category. selfID
// is the skill being updated, so renaming a skill to a different case is fine.
func (s *SkillService) ensureSkillNameFree(ctx context.Context, categoryID, name, selfID string) error {
	existing, err := s.skills.FindSkillByName(ctx, categoryID, name)
	switch {
	case err == nil:
		if existing.ID != selfID {
			return errSkillExists(existing.Name)
		}
		return nil
	case errors.Is(err, models.ErrNotFound):
		return nil
	default:
		return err
	}
}

// validateText rejects empty, over-long and control-character labels. Category
// and skill names are rendered by the public site and by the future PDF
// generator, so a bad payload must never reach the database.
func validateText(field, value string, max int) error {
	if value == "" {
		return errSkillValidation(field)
	}
	if utf8.RuneCountInString(value) > max {
		return errSkillValidationTooLong(field, max)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return errSkillValidationControlChars(field)
		}
	}
	return nil
}

// validateOptionalText applies validateText but treats an empty value as "clear
// the column", which skills.icon_slug allows.
func validateOptionalText(field, value string, max int) error {
	if value == "" {
		return nil
	}
	return validateText(field, value, max)
}

// validateDisplayOrder rejects negative orders; db_schema.sql has no CHECK
// constraint on the column, so the rule lives here.
func validateDisplayOrder(in *int) error {
	if in != nil && *in < 0 {
		return errSkillValidationDisplayOrder()
	}
	return nil
}

// mapCategoryWriteErr turns a UNIQUE violation into the 409 the admin panel
// expects. The duplicate check in the caller covers the normal case; this is
// the safety net for the index firing anyway.
func mapCategoryWriteErr(err error, name string) error {
	if errors.Is(err, models.ErrConflict) {
		return errSkillCategoryExists(name)
	}
	return err
}

// mapSkillWriteErr is mapCategoryWriteErr for the skills table.
func mapSkillWriteErr(err error, name string) error {
	if errors.Is(err, models.ErrConflict) {
		return errSkillExists(name)
	}
	return err
}

// displayOrderOrDefault resolves the optional DisplayOrder payload field.
func displayOrderOrDefault(in *int, fallback int) int {
	if in == nil {
		return fallback
	}
	return *in
}

func skillView(skill *models.Skill) SkillView {
	return SkillView{
		ID:           skill.ID,
		Name:         skill.Name,
		IconSlug:     skill.IconSlug,
		DisplayOrder: skill.DisplayOrder,
	}
}

func adminSkillView(skill *models.Skill) *AdminSkillView {
	return &AdminSkillView{
		SkillView:  skillView(skill),
		CategoryID: skill.CategoryID,
		CreatedAt:  skill.CreatedAt,
	}
}

func adminSkillCategoryView(category *models.SkillCategory) *AdminSkillCategoryView {
	return &AdminSkillCategoryView{
		ID:           category.ID,
		Name:         category.Name,
		DisplayOrder: category.DisplayOrder,
		CreatedAt:    category.CreatedAt,
	}
}

