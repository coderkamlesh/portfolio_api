package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
)

// ProjectDeps wires the project service to persistence. Now is injectable for
// deterministic tests; it defaults to UTC time.
type ProjectDeps struct {
	Projects ProjectStore
	Now      func() time.Time
	// Audit records content changes for the admin trail. Optional.
	Audit *Audit
}

// ProjectService owns the showcase-project use-cases of the portfolio.
type ProjectService struct {
	projects ProjectStore
	now      func() time.Time
	audit    *Audit
}

// NewProjectService builds the service.
func NewProjectService(deps ProjectDeps) *ProjectService {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &ProjectService{projects: deps.Projects, now: now, audit: deps.Audit}
}

// Input limits. db_schema.sql keeps these columns as unbounded TEXT, so the caps
// live here: generous enough for real content, tight enough that nothing absurd
// reaches the database or the future PDF renderer.
const (
	maxProjectTitleLen       = 150
	maxProjectTaglineLen     = 200
	maxProjectDescriptionLen = 4000
	maxProjectRoleLen        = 150
	maxProjectURLLen         = 500
	maxProjectBulletLen      = 300
	maxProjectBullets        = 20
	maxProjectTechnologyLen  = 50
	maxProjectTechnologies   = 30
)

// projectDateLayout is the only accepted date format, matching the
// start_date/end_date columns in db_schema.sql.
const projectDateLayout = "2006-01-02"

// ProjectInput is the create/update payload of a project. DisplayOrder keeps the
// stored order when nil, so a rename never silently reorders a project. Bullets
// are replaced wholesale, in payload order.
type ProjectInput struct {
	Title        string
	Tagline      string
	Description  string
	ProjectType  string
	Role         string
	Technologies []string
	RepoURL      string
	LiveURL      string
	ImageURL     string
	StartDate    string
	EndDate      string
	Status       string
	IsFeatured   bool
	Bullets      []string
	DisplayOrder *int
}

// ProjectView is one project as rendered on the public site.
type ProjectView struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Tagline      string   `json:"tagline,omitempty"`
	Description  string   `json:"description"`
	ProjectType  string   `json:"project_type,omitempty"`
	Role         string   `json:"role,omitempty"`
	Technologies []string `json:"technologies"`
	RepoURL      string   `json:"repo_url,omitempty"`
	LiveURL      string   `json:"live_url,omitempty"`
	ImageURL     string   `json:"image_url,omitempty"`
	StartDate    string   `json:"start_date,omitempty"`
	EndDate      string   `json:"end_date,omitempty"`
	Status       string   `json:"status,omitempty"`
	IsFeatured   bool     `json:"is_featured"`
	Bullets      []string `json:"bullets"`
	DisplayOrder int      `json:"display_order"`
}

// PublicProjectListView is the payload of GET /api/public/projects.
type PublicProjectListView struct {
	Projects []ProjectView `json:"projects"`
}

// PublicProjectDetailView is the payload of GET /api/public/projects/{id}.
type PublicProjectDetailView struct {
	Project ProjectView `json:"project"`
}

// AdminProjectView adds persistence metadata for the admin panel.
type AdminProjectView struct {
	ProjectView
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdminProjectListView is the payload of GET /api/admin/projects.
type AdminProjectListView struct {
	Projects []AdminProjectView `json:"projects"`
}

// PublicProjects returns every project, featured first.
func (s *ProjectService) PublicProjects(ctx context.Context) (*PublicProjectListView, error) {
	projects, bulletsByProject, err := s.loadProjects(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]ProjectView, 0, len(projects))
	for i := range projects {
		views = append(views, projectView(&projects[i], bulletsByProject[projects[i].ID]))
	}
	return &PublicProjectListView{Projects: views}, nil
}

// PublicProject returns one project with its bullets.
func (s *ProjectService) PublicProject(ctx context.Context, id string) (*PublicProjectDetailView, error) {
	project, err := s.findProject(ctx, id)
	if err != nil {
		return nil, err
	}
	bullets, err := s.projects.ListProjectBullets(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	return &PublicProjectDetailView{Project: projectView(project, bullets)}, nil
}

// AdminProjects returns every project with its audit timestamps.
func (s *ProjectService) AdminProjects(ctx context.Context) (*AdminProjectListView, error) {
	projects, bulletsByProject, err := s.loadProjects(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]AdminProjectView, 0, len(projects))
	for i := range projects {
		views = append(views, *adminProjectView(&projects[i], bulletsByProject[projects[i].ID]))
	}
	return &AdminProjectListView{Projects: views}, nil
}

// CreateProject stores a new project together with its bullets.
func (s *ProjectService) CreateProject(ctx context.Context, in ProjectInput) (*AdminProjectView, error) {
	now := s.now()
	project, bullets, err := buildProject(in, nil, now, now)
	if err != nil {
		return nil, err
	}
	if err := s.projects.SaveProject(ctx, project, bullets); err != nil {
		return nil, err
	}
	s.audit.Record(ctx, auditEntityProject, project.ID, AuditActionCreate, nil, project)
	return adminProjectView(project, bullets), nil
}

// UpdateProject replaces every editable field of a project and swaps its bullets
// for the ones in the payload.
func (s *ProjectService) UpdateProject(ctx context.Context, id string, in ProjectInput) (*AdminProjectView, error) {
	existing, err := s.findProject(ctx, id)
	if err != nil {
		return nil, err
	}

	project, bullets, err := buildProject(in, existing, existing.CreatedAt, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.projects.SaveProject(ctx, project, bullets); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errProjectNotFound()
		}
		return nil, err
	}
	s.audit.Record(ctx, auditEntityProject, project.ID, AuditActionUpdate, existing, project)
	return adminProjectView(project, bullets), nil
}

// DeleteProject removes a project and its bullets. The row is loaded first so the
// audit trail can record what was removed.
func (s *ProjectService) DeleteProject(ctx context.Context, id string) error {
	existing, err := s.findProject(ctx, id)
	if err != nil {
		return err
	}
	if err := s.projects.DeleteProject(ctx, id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return errProjectNotFound()
		}
		return err
	}
	s.audit.Record(ctx, auditEntityProject, id, AuditActionDelete, existing, nil)
	return nil
}

func (s *ProjectService) findProject(ctx context.Context, id string) (*models.Project, error) {
	project, err := s.projects.FindProjectByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errProjectNotFound()
		}
		return nil, err
	}
	return project, nil
}

// loadProjects returns the ordered projects plus their bullets keyed by project
// ID, so the views never issue one query per project.
func (s *ProjectService) loadProjects(ctx context.Context) ([]models.Project, map[string][]models.ProjectBullet, error) {
	projects, err := s.projects.ListProjects(ctx)
	if err != nil {
		return nil, nil, err
	}
	bullets, err := s.projects.ListProjectBullets(ctx, "")
	if err != nil {
		return nil, nil, err
	}

	grouped := make(map[string][]models.ProjectBullet, len(projects))
	for i := range bullets {
		grouped[bullets[i].ProjectID] = append(grouped[bullets[i].ProjectID], bullets[i])
	}
	return projects, grouped, nil
}


// buildProject validates the payload and produces the row plus its bullets.
// existing is nil on create; createdAt and updatedAt are resolved by the caller
// so tests stay deterministic.
func buildProject(in ProjectInput, existing *models.Project, createdAt, updatedAt time.Time) (*models.Project, []models.ProjectBullet, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, nil, errProjectRequired("title")
	}
	if err := validateProjectText("title", title, maxProjectTitleLen); err != nil {
		return nil, nil, err
	}

	description := strings.TrimSpace(in.Description)
	if description == "" {
		return nil, nil, errProjectRequired("description")
	}
	if err := validateProjectText("description", description, maxProjectDescriptionLen); err != nil {
		return nil, nil, err
	}

	tagline := strings.TrimSpace(in.Tagline)
	if err := validateOptionalProjectText("tagline", tagline, maxProjectTaglineLen); err != nil {
		return nil, nil, err
	}

	role := strings.TrimSpace(in.Role)
	if err := validateOptionalProjectText("role", role, maxProjectRoleLen); err != nil {
		return nil, nil, err
	}

	repoURL, err := normalizeProjectURL("repo_url", in.RepoURL)
	if err != nil {
		return nil, nil, err
	}
	liveURL, err := normalizeProjectURL("live_url", in.LiveURL)
	if err != nil {
		return nil, nil, err
	}
	imageURL, err := normalizeProjectURL("image_url", in.ImageURL)
	if err != nil {
		return nil, nil, err
	}

	projectType, err := normalizeEnum("project_type", in.ProjectType, models.ProjectTypes)
	if err != nil {
		return nil, nil, err
	}
	status, err := normalizeEnum("status", in.Status, models.ProjectStatuses)
	if err != nil {
		return nil, nil, err
	}

	startDate, start, err := normalizeProjectDate("start_date", in.StartDate)
	if err != nil {
		return nil, nil, err
	}
	endDate, end, err := normalizeProjectDate("end_date", in.EndDate)
	if err != nil {
		return nil, nil, err
	}
	if end != nil {
		if start == nil {
			return nil, nil, errProjectValidation("start_date is required when end_date is set.")
		}
		if end.Before(*start) {
			return nil, nil, errProjectValidation("end_date must not be earlier than start_date.")
		}
	}
	if status == models.ProjectStatusInProgress && endDate != "" {
		return nil, nil, errProjectValidation("end_date must be empty while status is IN_PROGRESS.")
	}

	technologies, err := normalizeProjectTechnologies(in.Technologies)
	if err != nil {
		return nil, nil, err
	}
	bulletTexts, err := normalizeProjectBullets(in.Bullets)
	if err != nil {
		return nil, nil, err
	}

	displayOrder := 0
	if existing != nil {
		displayOrder = existing.DisplayOrder
	}
	if in.DisplayOrder != nil {
		if *in.DisplayOrder < 0 {
			return nil, nil, errProjectValidation("display_order must not be negative.")
		}
		displayOrder = *in.DisplayOrder
	}

	project := &models.Project{
		ID:           ids.New(),
		Title:        title,
		Tagline:      tagline,
		Description:  description,
		ProjectType:  projectType,
		Role:         role,
		Technologies: technologies,
		RepoURL:      repoURL,
		LiveURL:      liveURL,
		ImageURL:     imageURL,
		StartDate:    startDate,
		EndDate:      endDate,
		Status:       status,
		IsFeatured:   in.IsFeatured,
		DisplayOrder: displayOrder,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
	if existing != nil {
		project.ID = existing.ID
	}
	return project, projectBulletsToModels(project.ID, bulletTexts), nil
}

// validateProjectText rejects over-long and control-character values. Tabs and
// newlines count as control characters, so a stray newline in a single-line
// field is rejected instead of silently breaking the PDF layout.
func validateProjectText(field, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return errProjectValidation(fmt.Sprintf("%s must be at most %d characters.", field, max))
	}
	if hasProjectControlChars(value) {
		return errProjectValidation(fmt.Sprintf("%s must not contain control characters.", field))
	}
	return nil
}

// validateOptionalProjectText applies validateProjectText but treats an empty
// value as "clear the column".
func validateOptionalProjectText(field, value string, max int) error {
	if value == "" {
		return nil
	}
	return validateProjectText(field, value, max)
}

func hasProjectControlChars(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// normalizeProjectURL trims the value and checks that it is an absolute http(s)
// URL. The database stores TEXT, so a malformed link would only surface when a
// visitor clicks it; catching it on write keeps the frontend simple.
func normalizeProjectURL(field, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if err := validateProjectText(field, trimmed, maxProjectURLLen); err != nil {
		return "", err
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", errProjectValidation(field + " must be a valid URL.")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errProjectValidation(field + " must be an http or https URL.")
	}
	if parsed.Host == "" {
		return "", errProjectValidation(field + " must be a valid URL.")
	}
	return trimmed, nil
}

// normalizeEnum upper-cases the value and rejects anything outside the allowed
// list. An empty value means "not specified".
func normalizeEnum(field, value string, allowed []string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", nil
	}
	for _, candidate := range allowed {
		if normalized == candidate {
			return normalized, nil
		}
	}
	return "", errProjectValidation(field + " must be one of " + strings.Join(allowed, ", ") + ".")
}

// normalizeProjectDate validates an optional calendar date and returns the
// trimmed string plus the parsed value. A nil time means the field was empty.
func normalizeProjectDate(field, value string) (string, *time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil, nil
	}
	parsed, err := time.Parse(projectDateLayout, trimmed)
	if err != nil {
		return "", nil, errProjectValidation(field + " must be a date in YYYY-MM-DD format.")
	}
	return trimmed, &parsed, nil
}

// normalizeProjectTechnologies trims the list, drops blank entries, enforces the
// caps and returns a non-nil slice so the JSON stays [] instead of null.
func normalizeProjectTechnologies(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if err := validateProjectText("each technology", value, maxProjectTechnologyLen); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	if len(out) > maxProjectTechnologies {
		return nil, errProjectValidation(
			fmt.Sprintf("technologies must contain at most %d entries.", maxProjectTechnologies))
	}
	return out, nil
}

// normalizeProjectBullets trims the points, drops blank lines and enforces the
// caps. The payload order is preserved.
func normalizeProjectBullets(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if err := validateProjectText("each bullet", value, maxProjectBulletLen); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	if len(out) > maxProjectBullets {
		return nil, errProjectValidation(
			fmt.Sprintf("bullets must contain at most %d entries.", maxProjectBullets))
	}
	return out, nil
}

// projectBulletsToModels assigns a fresh ID and a 1-based display_order to every
// point, mirroring the order the admin panel sent.
func projectBulletsToModels(projectID string, texts []string) []models.ProjectBullet {
	out := make([]models.ProjectBullet, 0, len(texts))
	for i, text := range texts {
		out = append(out, models.ProjectBullet{
			ID:           ids.New(),
			ProjectID:    projectID,
			Text:         text,
			DisplayOrder: i + 1,
		})
	}
	return out
}

func projectView(project *models.Project, bullets []models.ProjectBullet) ProjectView {
	return ProjectView{
		ID:           project.ID,
		Title:        project.Title,
		Tagline:      project.Tagline,
		Description:  project.Description,
		ProjectType:  project.ProjectType,
		Role:         project.Role,
		Technologies: projectTechnologyList(project.Technologies),
		RepoURL:      project.RepoURL,
		LiveURL:      project.LiveURL,
		ImageURL:     project.ImageURL,
		StartDate:    project.StartDate,
		EndDate:      project.EndDate,
		Status:       project.Status,
		IsFeatured:   project.IsFeatured,
		Bullets:      projectBulletTexts(bullets),
		DisplayOrder: project.DisplayOrder,
	}
}

func adminProjectView(project *models.Project, bullets []models.ProjectBullet) *AdminProjectView {
	return &AdminProjectView{
		ProjectView: projectView(project, bullets),
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}
}

// projectTechnologyList guarantees a non-nil slice so the JSON stays [] instead
// of null.
func projectTechnologyList(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// projectBulletTexts flattens the stored bullets into the string list the API
// exposes.
func projectBulletTexts(bullets []models.ProjectBullet) []string {
	out := make([]string, 0, len(bullets))
	for i := range bullets {
		out = append(out, bullets[i].Text)
	}
	return out
}
