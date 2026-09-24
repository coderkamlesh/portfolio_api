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

// EducationDeps wires the education service to persistence. Now is injectable
// so the year bounds and timestamps stay deterministic in tests.
type EducationDeps struct {
	Educations EducationStore
	Now        func() time.Time
}

// EducationService owns the education use-cases of the portfolio.
type EducationService struct {
	educations EducationStore
	now        func() time.Time
}

// NewEducationService builds the service.
func NewEducationService(deps EducationDeps) *EducationService {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &EducationService{educations: deps.Educations, now: now}
}

// Input limits. db_schema.sql keeps these columns as unbounded TEXT, so the caps
// live here: generous enough for real content, tight enough that nothing absurd
// reaches the database or the future PDF renderer.
const (
	maxInstitutionLen  = 200
	maxDegreeLen       = 150
	maxFieldOfStudyLen = 150
	maxGradeLen        = 50
	maxGPALen          = 20
	maxHonorsLen       = 200
	maxCourseLen       = 100
	// maxCoursework follows the schema comment on coursework: "3-6 relevant
	// courses". Six is the hard ceiling; a shorter list is allowed.
	maxCoursework = 6

	// minStartYear and maxFutureYears bound the year range. The lower bound
	// catches typos like 204; the upper bound allows an admission date slightly
	// in the future without allowing nonsense.
	minStartYear   = 1950
	maxFutureYears = 10
)

// EducationInput is the create/update payload. DisplayOrder keeps the stored
// order when nil, so an edit never silently reorders an entry. EndYear is a
// pointer so "not finished yet" and "explicitly 0" stay distinguishable.
type EducationInput struct {
	Institution  string
	Degree       string
	FieldOfStudy string
	StartYear    int
	EndYear      *int
	Grade        string
	GPA          string
	Coursework   []string
	Honors       string
	DisplayOrder *int
}

// EducationView is one entry as rendered on the public site.
type EducationView struct {
	ID           string   `json:"id"`
	Institution  string   `json:"institution"`
	Degree       string   `json:"degree"`
	FieldOfStudy string   `json:"field_of_study,omitempty"`
	StartYear    int      `json:"start_year"`
	EndYear      *int     `json:"end_year,omitempty"`
	Grade        string   `json:"grade,omitempty"`
	GPA          string   `json:"gpa,omitempty"`
	Coursework   []string `json:"coursework"`
	Honors       string   `json:"honors,omitempty"`
	DisplayOrder int      `json:"display_order"`
}

// PublicEducationListView is the payload of GET /api/public/education.
type PublicEducationListView struct {
	Education []EducationView `json:"education"`
}

// AdminEducationView adds persistence metadata for the admin panel. The
// educations table has no updated_at column, so only created_at is exposed.
type AdminEducationView struct {
	EducationView
	CreatedAt time.Time `json:"created_at"`
}

// AdminEducationListView is the payload of GET /api/admin/education.
type AdminEducationListView struct {
	Education []AdminEducationView `json:"education"`
}

// PublicEducations returns the education entries in display order.
func (s *EducationService) PublicEducations(ctx context.Context) (*PublicEducationListView, error) {
	entries, err := s.educations.ListEducations(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]EducationView, 0, len(entries))
	for i := range entries {
		views = append(views, educationView(&entries[i]))
	}
	return &PublicEducationListView{Education: views}, nil
}

// AdminEducations returns the education entries with their created_at stamp.
func (s *EducationService) AdminEducations(ctx context.Context) (*AdminEducationListView, error) {
	entries, err := s.educations.ListEducations(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]AdminEducationView, 0, len(entries))
	for i := range entries {
		views = append(views, *adminEducationView(&entries[i]))
	}
	return &AdminEducationListView{Education: views}, nil
}

// CreateEducation stores a new entry.
func (s *EducationService) CreateEducation(ctx context.Context, in EducationInput) (*AdminEducationView, error) {
	education, err := buildEducation(in, nil, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.educations.CreateEducation(ctx, education); err != nil {
		return nil, err
	}
	return adminEducationView(education), nil
}

// UpdateEducation replaces every editable field of an entry.
func (s *EducationService) UpdateEducation(ctx context.Context, id string, in EducationInput) (*AdminEducationView, error) {
	existing, err := s.findEducation(ctx, id)
	if err != nil {
		return nil, err
	}

	education, err := buildEducation(in, existing, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.educations.UpdateEducation(ctx, education); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errEducationNotFound()
		}
		return nil, err
	}
	return adminEducationView(education), nil
}

// DeleteEducation removes one entry.
func (s *EducationService) DeleteEducation(ctx context.Context, id string) error {
	if err := s.educations.DeleteEducation(ctx, id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return errEducationNotFound()
		}
		return err
	}
	return nil
}

func (s *EducationService) findEducation(ctx context.Context, id string) (*models.Education, error) {
	education, err := s.educations.FindEducationByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errEducationNotFound()
		}
		return nil, err
	}
	return education, nil
}

// buildEducation validates the payload and produces the row. existing is nil on
// create; its ID and created_at are carried over on update.
func buildEducation(in EducationInput, existing *models.Education, now time.Time) (*models.Education, error) {
	institution := strings.TrimSpace(in.Institution)
	if institution == "" {
		return nil, errEducationRequired("institution")
	}
	if err := validateEducationText("institution", institution, maxInstitutionLen); err != nil {
		return nil, err
	}

	degree := strings.TrimSpace(in.Degree)
	if degree == "" {
		return nil, errEducationRequired("degree")
	}
	if err := validateEducationText("degree", degree, maxDegreeLen); err != nil {
		return nil, err
	}

	fieldOfStudy := strings.TrimSpace(in.FieldOfStudy)
	if err := validateOptionalEducationText("field_of_study", fieldOfStudy, maxFieldOfStudyLen); err != nil {
		return nil, err
	}

	grade := strings.TrimSpace(in.Grade)
	if err := validateOptionalEducationText("grade", grade, maxGradeLen); err != nil {
		return nil, err
	}

	// gpa stays free text: the schema stores values like "8.5/10" or "3.8/4.0"
	// and the scale is unknown, so only the shape is checked here.
	gpa := strings.TrimSpace(in.GPA)
	if err := validateOptionalEducationText("gpa", gpa, maxGPALen); err != nil {
		return nil, err
	}

	honors := strings.TrimSpace(in.Honors)
	if err := validateOptionalEducationText("honors", honors, maxHonorsLen); err != nil {
		return nil, err
	}

	coursework, err := normalizeCoursework(in.Coursework)
	if err != nil {
		return nil, err
	}

	if err := validateEducationYears(in.StartYear, in.EndYear, now); err != nil {
		return nil, err
	}

	displayOrder := 0
	if existing != nil {
		displayOrder = existing.DisplayOrder
	}
	if in.DisplayOrder != nil {
		if *in.DisplayOrder < 0 {
			return nil, errEducationValidation("display_order must not be negative.")
		}
		displayOrder = *in.DisplayOrder
	}

	education := &models.Education{
		ID:           ids.New(),
		Institution:  institution,
		Degree:       degree,
		FieldOfStudy: fieldOfStudy,
		StartYear:    in.StartYear,
		EndYear:      derefInt(in.EndYear),
		Grade:        grade,
		GPA:          gpa,
		Coursework:   coursework,
		Honors:       honors,
		DisplayOrder: displayOrder,
		CreatedAt:    now,
	}
	if existing != nil {
		education.ID = existing.ID
		education.CreatedAt = existing.CreatedAt
	}
	return education, nil
}

// validateEducationYears checks the start year bounds, an optional end year, and
// the ordering between them.
func validateEducationYears(startYear int, endYear *int, now time.Time) error {
	maxYear := now.UTC().Year() + maxFutureYears

	if startYear == 0 {
		return errEducationRequired("start_year")
	}
	if startYear < minStartYear {
		return errEducationValidation(fmt.Sprintf("start_year must be %d or later.", minStartYear))
	}
	if startYear > maxYear {
		return errEducationValidation(fmt.Sprintf("start_year must not be later than %d.", maxYear))
	}
	if endYear == nil {
		return nil
	}
	if *endYear < startYear {
		return errEducationValidation("end_year must not be earlier than start_year.")
	}
	if *endYear > maxYear {
		return errEducationValidation(fmt.Sprintf("end_year must not be later than %d.", maxYear))
	}
	return nil
}

// normalizeCoursework trims the list, drops blank entries, enforces the cap and
// returns a non-nil slice so the JSON stays [] instead of null.
func normalizeCoursework(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if err := validateEducationText("each coursework", value, maxCourseLen); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	if len(out) > maxCoursework {
		return nil, errEducationValidation(
			fmt.Sprintf("coursework must contain at most %d entries.", maxCoursework))
	}
	return out, nil
}

// validateEducationText rejects over-long and control-character values. Tabs and
// newlines count as control characters, so a stray newline in a single-line
// field is rejected instead of silently breaking the PDF layout.
func validateEducationText(field, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return errEducationValidation(fmt.Sprintf("%s must be at most %d characters.", field, max))
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return errEducationValidation(fmt.Sprintf("%s must not contain control characters.", field))
		}
	}
	return nil
}

// validateOptionalEducationText applies validateEducationText but treats an empty
// value as "clear the column".
func validateOptionalEducationText(field, value string, max int) error {
	if value == "" {
		return nil
	}
	return validateEducationText(field, value, max)
}

func educationView(education *models.Education) EducationView {
	return EducationView{
		ID:           education.ID,
		Institution:  education.Institution,
		Degree:       education.Degree,
		FieldOfStudy: education.FieldOfStudy,
		StartYear:    education.StartYear,
		EndYear:      optionalInt(education.EndYear),
		Grade:        education.Grade,
		GPA:          education.GPA,
		Coursework:   educationCourseworkList(education.Coursework),
		Honors:       education.Honors,
		DisplayOrder: education.DisplayOrder,
	}
}

func adminEducationView(education *models.Education) *AdminEducationView {
	return &AdminEducationView{
		EducationView: educationView(education),
		CreatedAt:     education.CreatedAt,
	}
}

// educationCourseworkList guarantees a non-nil slice so the JSON stays []
// instead of null.
func educationCourseworkList(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// derefInt flattens an optional input year into the model's zero-means-empty
// convention.
func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

// optionalInt turns the model's zero back into a pointer so the JSON omits an
// in-progress end_year.
func optionalInt(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}