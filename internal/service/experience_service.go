package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
)

// ExperienceDeps wires the experience service to persistence. Now is injectable
// for deterministic tests; it defaults to UTC time.
type ExperienceDeps struct {
	Experiences ExperienceStore
	Now         func() time.Time
	// Audit records content changes for the admin trail. Optional.
	Audit *Audit
}

// ExperienceService owns the work-history use-cases of the portfolio.
type ExperienceService struct {
	experiences ExperienceStore
	now         func() time.Time
	audit       *Audit
}

// NewExperienceService builds the service.
func NewExperienceService(deps ExperienceDeps) *ExperienceService {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &ExperienceService{experiences: deps.Experiences, now: now, audit: deps.Audit}
}

// Input limits. db_schema.sql keeps these columns as unbounded TEXT, so the caps
// live here: generous enough for real content, tight enough that nothing absurd
// reaches the database or the future PDF renderer.
const (
	maxCompanyNameLen       = 150
	maxRoleLen              = 150
	maxLocationLen          = 150
	maxExperienceURLLen     = 500
	maxBulletLen            = 300
	maxBulletsPerEntry      = 30
	maxTechnologyLen        = 50
	maxTechnologiesPerEntry = 30
)

// experienceDateLayout is the only accepted date format (see start_date and
// end_date in db_schema.sql).
const experienceDateLayout = "2006-01-02"

// ExperienceInput is the create/update payload of a work experience. DisplayOrder
// keeps the stored order when nil, so a rename never silently reorders an entry.
// Bullets are replaced wholesale, in payload order.
type ExperienceInput struct {
	CompanyName    string
	CompanyLogoURL string
	Role           string
	EmploymentType string
	Location       string
	StartDate      string
	EndDate        string
	IsCurrent      bool
	Technologies   []string
	Bullets        []string
	DisplayOrder   *int
}

// ExperienceView is one entry as rendered on the public site.
type ExperienceView struct {
	ID             string   `json:"id"`
	CompanyName    string   `json:"company_name"`
	CompanyLogoURL string   `json:"company_logo_url,omitempty"`
	Role           string   `json:"role"`
	EmploymentType string   `json:"employment_type,omitempty"`
	Location       string   `json:"location,omitempty"`
	StartDate      string   `json:"start_date"`
	EndDate        string   `json:"end_date,omitempty"`
	IsCurrent      bool     `json:"is_current"`
	Technologies   []string `json:"technologies"`
	Bullets        []string `json:"bullets"`
	DisplayOrder   int      `json:"display_order"`
}

// PublicExperienceView is the payload of GET /api/public/experience.
type PublicExperienceView struct {
	Experiences []ExperienceView `json:"experiences"`
}

// AdminExperienceView adds persistence metadata for the admin panel.
type AdminExperienceView struct {
	ExperienceView
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdminExperiencesView is the payload of GET /api/admin/experience.
type AdminExperiencesView struct {
	Experiences []AdminExperienceView `json:"experiences"`
}

// PublicExperiences returns the work history in reverse chronological order.
func (s *ExperienceService) PublicExperiences(ctx context.Context) (*PublicExperienceView, error) {
	entries, bullets, err := s.loadExperiences(ctx)
	if err != nil {
		return nil, err
	}

	view := &PublicExperienceView{Experiences: make([]ExperienceView, 0, len(entries))}
	for i := range entries {
		view.Experiences = append(view.Experiences, experienceView(&entries[i], bullets[entries[i].ID]))
	}
	return view, nil
}

// AdminExperiences returns every entry with its persistence metadata.
func (s *ExperienceService) AdminExperiences(ctx context.Context) (*AdminExperiencesView, error) {
	entries, bullets, err := s.loadExperiences(ctx)
	if err != nil {
		return nil, err
	}

	view := &AdminExperiencesView{Experiences: make([]AdminExperienceView, 0, len(entries))}
	for i := range entries {
		view.Experiences = append(view.Experiences, *adminExperienceView(&entries[i], bullets[entries[i].ID]))
	}
	return view, nil
}

// CreateExperience adds an entry together with its bullets.
func (s *ExperienceService) CreateExperience(ctx context.Context, in ExperienceInput) (*AdminExperienceView, error) {
	now := s.now()
	experience, bullets, err := buildExperience(in, nil, now, now)
	if err != nil {
		return nil, err
	}
	if err := s.experiences.SaveExperience(ctx, experience, bullets); err != nil {
		return nil, err
	}
	s.audit.Record(ctx, auditEntityExperience, experience.ID, AuditActionCreate, nil, experience)
	return adminExperienceView(experience, bullets), nil
}

// UpdateExperience replaces every editable field of an entry and swaps its
// bullets for the ones in the payload.
func (s *ExperienceService) UpdateExperience(ctx context.Context, id string, in ExperienceInput) (*AdminExperienceView, error) {
	existing, err := s.findExperience(ctx, id)
	if err != nil {
		return nil, err
	}

	experience, bullets, err := buildExperience(in, existing, existing.CreatedAt, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.experiences.SaveExperience(ctx, experience, bullets); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errExperienceNotFound()
		}
		return nil, err
	}
	s.audit.Record(ctx, auditEntityExperience, experience.ID, AuditActionUpdate, existing, experience)
	return adminExperienceView(experience, bullets), nil
}

// DeleteExperience removes an entry and its bullets.
//
// The row is loaded before the delete so the audit trail can record what was
// removed; without it a delete would leave the trail with no snapshot.
func (s *ExperienceService) DeleteExperience(ctx context.Context, id string) error {
	existing, err := s.findExperience(ctx, id)
	if err != nil {
		return err
	}
	if err := s.experiences.DeleteExperience(ctx, id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return errExperienceNotFound()
		}
		return err
	}
	s.audit.Record(ctx, auditEntityExperience, id, AuditActionDelete, existing, nil)
	return nil
}

func (s *ExperienceService) findExperience(ctx context.Context, id string) (*models.WorkExperience, error) {
	experience, err := s.experiences.FindExperienceByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errExperienceNotFound()
		}
		return nil, err
	}
	return experience, nil
}

// loadExperiences returns the ordered entries plus their bullets keyed by
// experience ID, so the views never issue one query per entry.
func (s *ExperienceService) loadExperiences(ctx context.Context) ([]models.WorkExperience, map[string][]models.ExperienceBullet, error) {
	entries, err := s.experiences.ListExperiences(ctx)
	if err != nil {
		return nil, nil, err
	}
	bullets, err := s.experiences.ListBullets(ctx, "")
	if err != nil {
		return nil, nil, err
	}

	grouped := make(map[string][]models.ExperienceBullet, len(entries))
	for i := range bullets {
		grouped[bullets[i].ExperienceID] = append(grouped[bullets[i].ExperienceID], bullets[i])
	}
	return entries, grouped, nil
}

// buildExperience validates the payload and produces the row plus its bullets.
// existing is nil on create; createdAt and updatedAt are resolved by the caller
// so tests stay deterministic.
func buildExperience(in ExperienceInput, existing *models.WorkExperience, createdAt, updatedAt time.Time) (*models.WorkExperience, []models.ExperienceBullet, error) {
	companyName := strings.TrimSpace(in.CompanyName)
	if companyName == "" {
		return nil, nil, errExperienceRequired("company_name")
	}
	if err := validateExperienceText("company_name", companyName, maxCompanyNameLen); err != nil {
		return nil, nil, err
	}

	role := strings.TrimSpace(in.Role)
	if role == "" {
		return nil, nil, errExperienceRequired("role")
	}
	if err := validateExperienceText("role", role, maxRoleLen); err != nil {
		return nil, nil, err
	}

	logoURL := strings.TrimSpace(in.CompanyLogoURL)
	if err := validateOptionalExperienceText("company_logo_url", logoURL, maxExperienceURLLen); err != nil {
		return nil, nil, err
	}

	location := strings.TrimSpace(in.Location)
	if err := validateOptionalExperienceText("location", location, maxLocationLen); err != nil {
		return nil, nil, err
	}

	employmentType, err := normalizeEmploymentType(in.EmploymentType)
	if err != nil {
		return nil, nil, err
	}

	startDate := strings.TrimSpace(in.StartDate)
	if startDate == "" {
		return nil, nil, errExperienceRequired("start_date")
	}
	started, err := time.Parse(experienceDateLayout, startDate)
	if err != nil {
		return nil, nil, errExperienceValidation("start_date must be a date in YYYY-MM-DD format.")
	}

	endDate := strings.TrimSpace(in.EndDate)
	if endDate != "" {
		if in.IsCurrent {
			return nil, nil, errExperienceValidation("end_date must be empty while is_current is true.")
		}
		ended, err := time.Parse(experienceDateLayout, endDate)
		if err != nil {
			return nil, nil, errExperienceValidation("end_date must be a date in YYYY-MM-DD format.")
		}
		if ended.Before(started) {
			return nil, nil, errExperienceValidation("end_date must not be earlier than start_date.")
		}
	}

	technologies, err := normalizeTechnologies(in.Technologies)
	if err != nil {
		return nil, nil, err
	}
	bulletTexts, err := normalizeBullets(in.Bullets)
	if err != nil {
		return nil, nil, err
	}

	experience := &models.WorkExperience{
		ID:             ids.New(),
		CompanyName:    companyName,
		CompanyLogoURL: logoURL,
		Role:           role,
		EmploymentType: employmentType,
		Location:       location,
		StartDate:      startDate,
		EndDate:        endDate,
		IsCurrent:      in.IsCurrent,
		Technologies:   technologies,
		DisplayOrder:   displayOrderOrDefault(in.DisplayOrder, 0),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
	if existing != nil {
		// Keep the primary key and the original creation time.
		experience.ID = existing.ID
		experience.CreatedAt = existing.CreatedAt
		experience.DisplayOrder = displayOrderOrDefault(in.DisplayOrder, existing.DisplayOrder)
	}

	return experience, bulletsToModels(experience.ID, bulletTexts), nil
}

// validateExperienceText rejects over-long and control-character single-line
// labels. Experience text is rendered by the public site and later by the
// generated PDF, so a bad payload must never reach the database. Callers check
// emptiness first, because the message differs between required and optional
// fields.
func validateExperienceText(field, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return errExperienceValidation(fmt.Sprintf("%s must be at most %d characters.", field, max))
	}
	if hasControlChars(value) {
		return errExperienceValidation(fmt.Sprintf("%s must not contain control characters.", field))
	}
	return nil
}

// validateOptionalExperienceText applies validateExperienceText but treats an
// empty value as "clear the column".
func validateOptionalExperienceText(field, value string, max int) error {
	if value == "" {
		return nil
	}
	return validateExperienceText(field, value, max)
}

// hasControlChars reports whether value contains a control character. Tabs and
// newlines count: every field of this module is single-line.
func hasControlChars(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// normalizeEmploymentType uppercases the value and rejects anything outside the
// enum documented in db_schema.sql. An empty value means "not specified".
func normalizeEmploymentType(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", nil
	}
	for _, allowed := range models.EmploymentTypes {
		if normalized == allowed {
			return normalized, nil
		}
	}
	return "", errExperienceValidation(
		"employment_type must be one of " + strings.Join(models.EmploymentTypes, ", ") + ".")
}

// normalizeTechnologies trims the list, drops blank entries, enforces the caps
// and returns a non-nil slice so the JSON stays [] instead of null.
func normalizeTechnologies(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if utf8.RuneCountInString(value) > maxTechnologyLen {
			return nil, errExperienceValidation(
				fmt.Sprintf("each technology must be at most %d characters.", maxTechnologyLen))
		}
		if hasControlChars(value) {
			return nil, errExperienceValidation("each technology must not contain control characters.")
		}
		out = append(out, value)
	}
	if len(out) > maxTechnologiesPerEntry {
		return nil, errExperienceValidation(
			fmt.Sprintf("technologies must contain at most %d entries.", maxTechnologiesPerEntry))
	}
	return out, nil
}

// normalizeBullets trims the X-Y-Z points, drops blank lines and enforces the
// caps. The payload order is preserved.
func normalizeBullets(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if err := validateExperienceText("each bullet", value, maxBulletLen); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	if len(out) > maxBulletsPerEntry {
		return nil, errExperienceValidation(
			fmt.Sprintf("bullets must contain at most %d entries.", maxBulletsPerEntry))
	}
	return out, nil
}

// bulletsToModels assigns a fresh ID and a 1-based display_order to every point,
// mirroring the order the admin panel sent.
func bulletsToModels(experienceID string, texts []string) []models.ExperienceBullet {
	out := make([]models.ExperienceBullet, 0, len(texts))
	for i, text := range texts {
		out = append(out, models.ExperienceBullet{
			ID:           ids.New(),
			ExperienceID: experienceID,
			Text:         text,
			DisplayOrder: i + 1,
		})
	}
	return out
}

func experienceView(experience *models.WorkExperience, bullets []models.ExperienceBullet) ExperienceView {
	return ExperienceView{
		ID:             experience.ID,
		CompanyName:    experience.CompanyName,
		CompanyLogoURL: experience.CompanyLogoURL,
		Role:           experience.Role,
		EmploymentType: experience.EmploymentType,
		Location:       experience.Location,
		StartDate:      experience.StartDate,
		EndDate:        experience.EndDate,
		IsCurrent:      experience.IsCurrent,
		Technologies:   technologyList(experience.Technologies),
		Bullets:        bulletTexts(bullets),
		DisplayOrder:   experience.DisplayOrder,
	}
}

func adminExperienceView(experience *models.WorkExperience, bullets []models.ExperienceBullet) *AdminExperienceView {
	return &AdminExperienceView{
		ExperienceView: experienceView(experience, bullets),
		CreatedAt:      experience.CreatedAt,
		UpdatedAt:      experience.UpdatedAt,
	}
}

// technologyList guarantees a non-nil slice so the JSON stays [] instead of null.
func technologyList(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// bulletTexts flattens the stored bullets into the string list the API exposes.
func bulletTexts(bullets []models.ExperienceBullet) []string {
	out := make([]string, 0, len(bullets))
	for i := range bullets {
		out = append(out, bullets[i].Text)
	}
	return out
}
