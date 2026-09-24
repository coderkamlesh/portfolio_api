package service

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// projectTestNow is the fixed clock every project test runs on.
var projectTestNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// fakeProjectStore is an in-memory ProjectStore. It mirrors the real
// repository: projects are listed featured-first and saving replaces the bullets
// of a project.
type fakeProjectStore struct {
	projects []models.Project
	bullets  map[string][]models.ProjectBullet
	saveErr  error
}

func newFakeProjectStore() *fakeProjectStore {
	return &fakeProjectStore{bullets: make(map[string][]models.ProjectBullet)}
}

func (s *fakeProjectStore) ListProjects(context.Context) ([]models.Project, error) {
	out := append([]models.Project(nil), s.projects...)
	// Same order as the SQL: is_featured DESC, display_order, id.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsFeatured != out[j].IsFeatured {
			return out[i].IsFeatured
		}
		return out[i].DisplayOrder < out[j].DisplayOrder
	})
	return out, nil
}

func (s *fakeProjectStore) FindProjectByID(_ context.Context, id string) (*models.Project, error) {
	for i := range s.projects {
		if s.projects[i].ID == id {
			project := s.projects[i]
			return &project, nil
		}
	}
	return nil, models.ErrNotFound
}

func (s *fakeProjectStore) ListProjectBullets(_ context.Context, projectID string) ([]models.ProjectBullet, error) {
	if projectID != "" {
		return s.copyProjectBullets(projectID), nil
	}

	entries, _ := s.ListProjects(context.Background())
	out := make([]models.ProjectBullet, 0)
	for i := range entries {
		out = append(out, s.copyProjectBullets(entries[i].ID)...)
	}
	return out, nil
}

func (s *fakeProjectStore) SaveProject(_ context.Context, project *models.Project, bullets []models.ProjectBullet) error {
	if s.saveErr != nil {
		return s.saveErr
	}

	stored := *project
	replaced := false
	for i := range s.projects {
		if s.projects[i].ID == project.ID {
			s.projects[i] = stored
			replaced = true
			break
		}
	}
	if !replaced {
		s.projects = append(s.projects, stored)
	}

	s.bullets[project.ID] = append([]models.ProjectBullet(nil), bullets...)
	return nil
}

func (s *fakeProjectStore) DeleteProject(_ context.Context, id string) error {
	for i := range s.projects {
		if s.projects[i].ID == id {
			s.projects = append(s.projects[:i], s.projects[i+1:]...)
			delete(s.bullets, id)
			return nil
		}
	}
	return models.ErrNotFound
}

func (s *fakeProjectStore) copyProjectBullets(projectID string) []models.ProjectBullet {
	stored := s.bullets[projectID]
	out := append([]models.ProjectBullet(nil), stored...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].DisplayOrder < out[j].DisplayOrder })
	return out
}

// newProjectTestService builds the service over a fixed clock.
func newProjectTestService(store *fakeProjectStore) *ProjectService {
	return NewProjectService(ProjectDeps{
		Projects: store,
		Now:      func() time.Time { return projectTestNow },
	})
}

// projectErrCode returns the API error code of err, failing the test when err is
// nil.
func projectErrCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return apierr.From(err).Code
}

// validProjectInput returns a payload that passes every validation rule.
func validProjectInput() ProjectInput {
	return ProjectInput{
		Title:        "AlgoMaster",
		Tagline:      "Practice platform for DSA interview preparation.",
		Description:  "Built the backend that serves 40k monthly practice sessions.",
		ProjectType:  "personal",
		Role:         "Backend Developer",
		Technologies: []string{"Go", "PostgreSQL"},
		RepoURL:      "https://github.com/kamlesh/algomaster",
		LiveURL:      "https://algomaster.example.com",
		ImageURL:     "https://cdn.example.com/algomaster.png",
		StartDate:    "2024-04-01",
		EndDate:      "2025-12-31",
		Status:       "completed",
		IsFeatured:   true,
		Bullets: []string{
			"Cut p95 submission latency by 38% with a Redis-backed result cache.",
			"Designed the judge queue that isolates test execution per request.",
		},
	}
}

func TestPublicProjectsListsFeaturedFirstWithNestedBullets(t *testing.T) {
	store := newFakeProjectStore()
	store.projects = []models.Project{
		{ID: "p-plain", Title: "Plain", Description: "d", DisplayOrder: 0, CreatedAt: projectTestNow, UpdatedAt: projectTestNow},
		{ID: "p-featured", Title: "Featured", Description: "d", IsFeatured: true, DisplayOrder: 5, CreatedAt: projectTestNow, UpdatedAt: projectTestNow},
		{ID: "p-featured-first", Title: "Featured first", Description: "d", IsFeatured: true, DisplayOrder: 1, CreatedAt: projectTestNow, UpdatedAt: projectTestNow},
	}
	store.bullets["p-featured-first"] = []models.ProjectBullet{
		{ID: "b2", ProjectID: "p-featured-first", Text: "Second point", DisplayOrder: 2},
		{ID: "b1", ProjectID: "p-featured-first", Text: "First point", DisplayOrder: 1},
	}
	svc := newProjectTestService(store)

	got, err := svc.PublicProjects(context.Background())
	if err != nil {
		t.Fatalf("PublicProjects: %v", err)
	}
	if len(got.Projects) != 3 {
		t.Fatalf("got %d projects, want 3", len(got.Projects))
	}
	if got.Projects[0].ID != "p-featured-first" || got.Projects[1].ID != "p-featured" {
		t.Errorf("featured projects are not first: %q then %q", got.Projects[0].ID, got.Projects[1].ID)
	}
	if got.Projects[2].ID != "p-plain" {
		t.Errorf("non-featured project should come last, got %q", got.Projects[2].ID)
	}
	if len(got.Projects[0].Bullets) != 2 || got.Projects[0].Bullets[0] != "First point" {
		t.Errorf("bullets were not nested in display order: %+v", got.Projects[0].Bullets)
	}
	if got.Projects[2].Bullets == nil || got.Projects[2].Technologies == nil {
		t.Error("a project without bullets or technologies must render empty arrays, not null")
	}
}

func TestPublicProjectsRendersEmptyArrayWhenNothingConfigured(t *testing.T) {
	svc := newProjectTestService(newFakeProjectStore())

	got, err := svc.PublicProjects(context.Background())
	if err != nil {
		t.Fatalf("PublicProjects: %v", err)
	}
	if got.Projects == nil || len(got.Projects) != 0 {
		t.Fatalf("Projects = %#v, want an empty non-nil slice", got.Projects)
	}
}

func TestPublicProjectReturnsOneProject(t *testing.T) {
	store := newFakeProjectStore()
	store.projects = []models.Project{
		{ID: "p-1", Title: "AlgoMaster", Description: "d", CreatedAt: projectTestNow, UpdatedAt: projectTestNow},
	}
	store.bullets["p-1"] = []models.ProjectBullet{
		{ID: "b1", ProjectID: "p-1", Text: "A point", DisplayOrder: 1},
	}
	svc := newProjectTestService(store)

	got, err := svc.PublicProject(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("PublicProject: %v", err)
	}
	if got.Project.ID != "p-1" || len(got.Project.Bullets) != 1 {
		t.Fatalf("unexpected detail payload: %+v", got.Project)
	}
}

func TestPublicProjectNotFound(t *testing.T) {
	svc := newProjectTestService(newFakeProjectStore())

	_, err := svc.PublicProject(context.Background(), "missing")
	if code := projectErrCode(t, err); code != "project_not_found" {
		t.Fatalf("got code %q, want project_not_found", code)
	}
}

func TestCreateProjectNormalizesPayload(t *testing.T) {
	store := newFakeProjectStore()
	svc := newProjectTestService(store)

	in := validProjectInput()
	in.Title = "  AlgoMaster  "
	in.ProjectType = "personal"
	in.Status = "completed"
	in.Technologies = []string{" Go ", "", "PostgreSQL"}
	in.Bullets = []string{" First point ", "   ", "Second point"}
	in.DisplayOrder = nil

	got, err := svc.CreateProject(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if got.Title != "AlgoMaster" {
		t.Errorf("Title = %q, want the trimmed value", got.Title)
	}
	if got.ProjectType != models.ProjectTypePersonal {
		t.Errorf("ProjectType = %q, want %q", got.ProjectType, models.ProjectTypePersonal)
	}
	if got.Status != models.ProjectStatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, models.ProjectStatusCompleted)
	}
	if len(got.Technologies) != 2 || got.Technologies[0] != "Go" {
		t.Errorf("blank technologies must be dropped: %+v", got.Technologies)
	}
	if len(got.Bullets) != 2 || got.Bullets[1] != "Second point" {
		t.Errorf("blank bullets must be dropped: %+v", got.Bullets)
	}
	if got.DisplayOrder != 0 {
		t.Errorf("DisplayOrder = %d, want 0 when the payload omits it", got.DisplayOrder)
	}
	if got.ID == "" {
		t.Error("a new project must get an ID")
	}
	if !got.CreatedAt.Equal(projectTestNow) || !got.UpdatedAt.Equal(projectTestNow) {
		t.Errorf("timestamps were not stamped: created=%v updated=%v", got.CreatedAt, got.UpdatedAt)
	}

	stored := store.bullets[got.ID]
	if len(stored) != 2 || stored[0].DisplayOrder != 1 || stored[1].DisplayOrder != 2 {
		t.Fatalf("bullets were not stored with a 1-based order: %+v", stored)
	}
}

func TestCreateProjectRequiresCoreFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ProjectInput)
	}{
		{name: "title", mutate: func(in *ProjectInput) { in.Title = "   " }},
		{name: "description", mutate: func(in *ProjectInput) { in.Description = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeProjectStore()
			svc := newProjectTestService(store)

			in := validProjectInput()
			tc.mutate(&in)

			_, err := svc.CreateProject(context.Background(), in)
			if code := projectErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if len(store.projects) != 0 {
				t.Errorf("invalid input must not be stored: %+v", store.projects)
			}
		})
	}
}

func TestCreateProjectValidatesEnums(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ProjectInput)
		wantErr string
	}{
		{name: "valid project_type lowercase", mutate: func(in *ProjectInput) { in.ProjectType = "open_source" }},
		{name: "valid status lowercase", mutate: func(in *ProjectInput) { in.Status = "completed" }},
		{name: "empty project_type allowed", mutate: func(in *ProjectInput) { in.ProjectType = "" }},
		{
			name:    "unknown project_type",
			mutate:  func(in *ProjectInput) { in.ProjectType = "startup" },
			wantErr: "project_type must be one of PERSONAL, ACADEMIC, OPEN_SOURCE, INTERNSHIP.",
		},
		{
			name:    "unknown status",
			mutate:  func(in *ProjectInput) { in.Status = "paused" },
			wantErr: "status must be one of COMPLETED, IN_PROGRESS.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newProjectTestService(newFakeProjectStore())

			in := validProjectInput()
			tc.mutate(&in)

			_, err := svc.CreateProject(context.Background(), in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if code := projectErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if got := apierr.From(err).Message; got != tc.wantErr {
				t.Errorf("error message = %q, want %q", got, tc.wantErr)
			}
		})
	}
}

func TestCreateProjectValidatesDates(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ProjectInput)
		wantErr string
	}{
		{
			name:    "bad start_date format",
			mutate:  func(in *ProjectInput) { in.StartDate = "2024/04/01" },
			wantErr: "start_date must be a date in YYYY-MM-DD format.",
		},
		{
			name:    "bad end_date format",
			mutate:  func(in *ProjectInput) { in.EndDate = "31-12-2025" },
			wantErr: "end_date must be a date in YYYY-MM-DD format.",
		},
		{
			name:    "end_date without start_date",
			mutate:  func(in *ProjectInput) { in.StartDate = "" },
			wantErr: "start_date is required when end_date is set.",
		},
		{
			name:    "end_date earlier than start_date",
			mutate:  func(in *ProjectInput) { in.EndDate = "2023-01-01" },
			wantErr: "end_date must not be earlier than start_date.",
		},
		{
			name:    "end_date while in progress",
			mutate:  func(in *ProjectInput) { in.Status = "in_progress" },
			wantErr: "end_date must be empty while status is IN_PROGRESS.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newProjectTestService(newFakeProjectStore())

			in := validProjectInput()
			tc.mutate(&in)

			_, err := svc.CreateProject(context.Background(), in)
			if code := projectErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if got := apierr.From(err).Message; got != tc.wantErr {
				t.Errorf("error message = %q, want %q", got, tc.wantErr)
			}
		})
	}
}

func TestCreateProjectValidatesURLs(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ProjectInput)
		wantErr string
	}{
		{
			name:    "repo_url without scheme",
			mutate:  func(in *ProjectInput) { in.RepoURL = "github.com/kamlesh/algomaster" },
			wantErr: "repo_url must be an http or https URL.",
		},
		{
			name:    "live_url with ftp scheme",
			mutate:  func(in *ProjectInput) { in.LiveURL = "ftp://example.com" },
			wantErr: "live_url must be an http or https URL.",
		},
		{
			name:    "image_url without host",
			mutate:  func(in *ProjectInput) { in.ImageURL = "https://" },
			wantErr: "image_url must be a valid URL.",
		},
		{
			name:    "empty url allowed",
			mutate:  func(in *ProjectInput) { in.RepoURL = ""; in.LiveURL = ""; in.ImageURL = "" },
			wantErr: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newProjectTestService(newFakeProjectStore())

			in := validProjectInput()
			tc.mutate(&in)

			got, err := svc.CreateProject(context.Background(), in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.RepoURL != "" || got.LiveURL != "" || got.ImageURL != "" {
					t.Errorf("empty URLs must stay empty: %+v", got)
				}
				return
			}
			if code := projectErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if msg := apierr.From(err).Message; msg != tc.wantErr {
				t.Errorf("error message = %q, want %q", msg, tc.wantErr)
			}
		})
	}
}

func TestCreateProjectEnforcesLengthLimits(t *testing.T) {
	longText := make([]byte, maxProjectTitleLen+1)
	for i := range longText {
		longText[i] = 'a'
	}

	cases := []struct {
		name    string
		mutate  func(*ProjectInput)
		wantErr string
	}{
		{
			name:    "title too long",
			mutate:  func(in *ProjectInput) { in.Title = string(longText) },
			wantErr: "title must be at most 150 characters.",
		},
		{
			name:    "control characters in title",
			mutate:  func(in *ProjectInput) { in.Title = "Algo\nMaster" },
			wantErr: "title must not contain control characters.",
		},
		{
			name:    "bullet too long",
			mutate:  func(in *ProjectInput) { in.Bullets = []string{strings.Repeat("a", maxProjectBulletLen+1)} },
			wantErr: "each bullet must be at most 300 characters.",
		},
		{
			name:    "technology too long",
			mutate:  func(in *ProjectInput) { in.Technologies = []string{strings.Repeat("a", maxProjectTechnologyLen+1)} },
			wantErr: "each technology must be at most 50 characters.",
		},
		{
			name: "too many bullets",
			mutate: func(in *ProjectInput) {
				bullets := make([]string, maxProjectBullets+1)
				for i := range bullets {
					bullets[i] = "point"
				}
				in.Bullets = bullets
			},
			wantErr: "bullets must contain at most 20 entries.",
		},
		{
			name: "too many technologies",
			mutate: func(in *ProjectInput) {
				technologies := make([]string, maxProjectTechnologies+1)
				for i := range technologies {
					technologies[i] = "tech"
				}
				in.Technologies = technologies
			},
			wantErr: "technologies must contain at most 30 entries.",
		},
		{
			name: "negative display_order",
			mutate: func(in *ProjectInput) {
				order := -1
				in.DisplayOrder = &order
			},
			wantErr: "display_order must not be negative.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeProjectStore()
			svc := newProjectTestService(store)

			in := validProjectInput()
			tc.mutate(&in)

			_, err := svc.CreateProject(context.Background(), in)
			if code := projectErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if msg := apierr.From(err).Message; msg != tc.wantErr {
				t.Errorf("error message = %q, want %q", msg, tc.wantErr)
			}
			if len(store.projects) != 0 {
				t.Error("invalid input must not be stored")
			}
		})
	}
}

func TestUpdateProjectReplacesBulletsAndKeepsIdentity(t *testing.T) {
	store := newFakeProjectStore()
	svc := newProjectTestService(store)

	created, err := svc.CreateProject(context.Background(), validProjectInput())
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	updateIn := validProjectInput()
	updateIn.Title = "AlgoMaster v2"
	updateIn.Bullets = []string{"A single new technical highlight."}
	updateIn.DisplayOrder = nil

	updated, err := svc.UpdateProject(context.Background(), created.ID, updateIn)
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	if updated.ID != created.ID {
		t.Errorf("ID changed on update: %q -> %q", created.ID, updated.ID)
	}
	if updated.Title != "AlgoMaster v2" {
		t.Errorf("Title = %q, want AlgoMaster v2", updated.Title)
	}
	if len(updated.Bullets) != 1 || updated.Bullets[0] != "A single new technical highlight." {
		t.Errorf("bullets were not replaced: %+v", updated.Bullets)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("created_at changed: %v -> %v", created.CreatedAt, updated.CreatedAt)
	}
	if len(store.projects) != 1 {
		t.Errorf("update must not insert a second row, got %d", len(store.projects))
	}
	stored := store.bullets[created.ID]
	if len(stored) != 1 {
		t.Errorf("stored bullets were not replaced: %+v", stored)
	}
}

func TestUpdateProjectAcceptsExplicitDisplayOrder(t *testing.T) {
	store := newFakeProjectStore()
	svc := newProjectTestService(store)

	created, err := svc.CreateProject(context.Background(), validProjectInput())
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	order := 7
	updateIn := validProjectInput()
	updateIn.DisplayOrder = &order

	updated, err := svc.UpdateProject(context.Background(), created.ID, updateIn)
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if updated.DisplayOrder != 7 {
		t.Errorf("DisplayOrder = %d, want 7", updated.DisplayOrder)
	}
}

func TestUpdateProjectNotFound(t *testing.T) {
	svc := newProjectTestService(newFakeProjectStore())

	_, err := svc.UpdateProject(context.Background(), "missing", validProjectInput())
	if code := projectErrCode(t, err); code != "project_not_found" {
		t.Fatalf("got code %q, want project_not_found", code)
	}
}

func TestUpdateProjectDoesNotWriteInvalidInput(t *testing.T) {
	store := newFakeProjectStore()
	svc := newProjectTestService(store)

	created, err := svc.CreateProject(context.Background(), validProjectInput())
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	updateIn := validProjectInput()
	updateIn.Status = "in_progress" // still carries an end_date

	if _, err := svc.UpdateProject(context.Background(), created.ID, updateIn); err == nil {
		t.Fatal("expected a validation error, got nil")
	}
	if len(store.projects) != 1 || store.projects[0].Title != created.Title {
		t.Errorf("an invalid update must not touch the stored row: %+v", store.projects)
	}
}

func TestDeleteProjectRemovesRowAndBullets(t *testing.T) {
	store := newFakeProjectStore()
	svc := newProjectTestService(store)

	created, err := svc.CreateProject(context.Background(), validProjectInput())
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := svc.DeleteProject(context.Background(), created.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if len(store.projects) != 0 {
		t.Errorf("expected an empty store, got %d projects", len(store.projects))
	}
	if len(store.bullets[created.ID]) != 0 {
		t.Errorf("expected bullets to be removed, got %d", len(store.bullets[created.ID]))
	}

	err = svc.DeleteProject(context.Background(), created.ID)
	if code := projectErrCode(t, err); code != "project_not_found" {
		t.Errorf("got code %q, want project_not_found on the second delete", code)
	}
}

func TestAdminProjectsExposeTimestamps(t *testing.T) {
	store := newFakeProjectStore()
	svc := newProjectTestService(store)

	created, err := svc.CreateProject(context.Background(), validProjectInput())
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	got, err := svc.AdminProjects(context.Background())
	if err != nil {
		t.Fatalf("AdminProjects: %v", err)
	}
	if len(got.Projects) != 1 {
		t.Fatalf("got %d projects, want 1", len(got.Projects))
	}
	if got.Projects[0].ID != created.ID {
		t.Errorf("ID = %q, want %q", got.Projects[0].ID, created.ID)
	}
	if !got.Projects[0].CreatedAt.Equal(projectTestNow) || !got.Projects[0].UpdatedAt.Equal(projectTestNow) {
		t.Errorf("timestamps were not exposed: %+v", got.Projects[0])
	}
}