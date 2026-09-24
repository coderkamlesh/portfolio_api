package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/service"
	"github.com/go-chi/chi/v5"
)

// fakeProjectStore is a minimal in-memory ProjectStore for the HTTP tests. The
// service-level fakes cover the business rules; this one only has to be a
// faithful stand-in for the persistence contract at the transport boundary.
type fakeProjectStore struct {
	projects []models.Project
	bullets  map[string][]models.ProjectBullet
}

func newFakeProjectStore() *fakeProjectStore {
	return &fakeProjectStore{bullets: make(map[string][]models.ProjectBullet)}
}

func (s *fakeProjectStore) ListProjects(context.Context) ([]models.Project, error) {
	return append([]models.Project(nil), s.projects...), nil
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
		return append([]models.ProjectBullet(nil), s.bullets[projectID]...), nil
	}
	out := make([]models.ProjectBullet, 0)
	for i := range s.projects {
		out = append(out, s.bullets[s.projects[i].ID]...)
	}
	return out, nil
}

func (s *fakeProjectStore) SaveProject(_ context.Context, project *models.Project, bullets []models.ProjectBullet) error {
	stored := *project
	for i := range s.projects {
		if s.projects[i].ID == project.ID {
			s.projects[i] = stored
			s.bullets[project.ID] = append([]models.ProjectBullet(nil), bullets...)
			return nil
		}
	}
	s.projects = append(s.projects, stored)
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

func newProjectHandlerForTest(store service.ProjectStore) *ProjectHandler {
	return NewProjectHandler(service.NewProjectService(service.ProjectDeps{Projects: store}))
}

// callProjectHandler runs one handler with a chi URL param, mirroring how the
// router injects {id}.
func callProjectHandler(t *testing.T, h http.HandlerFunc, method, target, id, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	r := httptest.NewRequest(method, target, reader)

	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}

	w := httptest.NewRecorder()
	h(w, r)
	return w
}

// decodeErrorEnvelope pulls error.code out of a failed response.
func decodeErrorEnvelope(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()

	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response is not a JSON error envelope: %v (body %q)", err, w.Body.String())
	}
	return envelope.Error.Code
}

const validProjectBody = `{
  "title": "AlgoMaster",
  "description": "Built the backend that serves 40k monthly practice sessions.",
  "project_type": "personal",
  "status": "completed",
  "technologies": ["Go"],
  "bullets": ["Cut p95 submission latency by 38%."]
}`

func TestPublicProjectsReturnsAnEmptyArrayWhenNoProjectsExist(t *testing.T) {
	h := newProjectHandlerForTest(newFakeProjectStore())

	w := callProjectHandler(t, h.PublicProjects, http.MethodGet, "/api/public/projects", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var payload struct {
		Projects []service.ProjectView `json:"projects"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v (body %q)", err, w.Body.String())
	}
	if payload.Projects == nil {
		t.Error("projects must be an empty array, not null")
	}
	if len(payload.Projects) != 0 {
		t.Errorf("got %d projects, want 0", len(payload.Projects))
	}
}

func TestPublicProjectNotFoundReturnsTheErrorEnvelope(t *testing.T) {
	h := newProjectHandlerForTest(newFakeProjectStore())

	w := callProjectHandler(t, h.PublicProject, http.MethodGet, "/api/public/projects/missing", "missing", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %q)", w.Code, w.Body.String())
	}
	if code := decodeErrorEnvelope(t, w); code != "project_not_found" {
		t.Errorf("code = %q, want project_not_found", code)
	}
}

func TestCreateProjectReturns201WithTheStoredProject(t *testing.T) {
	h := newProjectHandlerForTest(newFakeProjectStore())

	w := callProjectHandler(t, h.CreateProject, http.MethodPost, "/api/admin/projects", "", validProjectBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", w.Code, w.Body.String())
	}
	var payload struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		ProjectType string   `json:"project_type"`
		Bullets     []string `json:"bullets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v (body %q)", err, w.Body.String())
	}
	if payload.ID == "" {
		t.Error("the created project must come back with an id")
	}
	if payload.Title != "AlgoMaster" {
		t.Errorf("title = %q, want AlgoMaster", payload.Title)
	}
	if payload.ProjectType != "PERSONAL" {
		t.Errorf("project_type = %q, want the upper-cased PERSONAL", payload.ProjectType)
	}
	if len(payload.Bullets) != 1 {
		t.Errorf("bullets = %+v, want the single stored bullet", payload.Bullets)
	}
}

func TestCreateProjectRejectsUnknownFields(t *testing.T) {
	h := newProjectHandlerForTest(newFakeProjectStore())

	body := `{"title":"AlgoMaster","description":"d","is_feature":"yes"}`
	w := callProjectHandler(t, h.CreateProject, http.MethodPost, "/api/admin/projects", "", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %q)", w.Code, w.Body.String())
	}
	if code := decodeErrorEnvelope(t, w); code != "invalid_json" {
		t.Errorf("code = %q, want invalid_json", code)
	}
}

func TestCreateProjectRejectsAMissingRequiredField(t *testing.T) {
	h := newProjectHandlerForTest(newFakeProjectStore())

	w := callProjectHandler(t, h.CreateProject, http.MethodPost, "/api/admin/projects", "", `{"description":"d"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %q)", w.Code, w.Body.String())
	}
	if code := decodeErrorEnvelope(t, w); code != "validation_failed" {
		t.Errorf("code = %q, want validation_failed", code)
	}
}

func TestCreateProjectRejectsAnEmptyBody(t *testing.T) {
	h := newProjectHandlerForTest(newFakeProjectStore())

	w := callProjectHandler(t, h.CreateProject, http.MethodPost, "/api/admin/projects", "", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if code := decodeErrorEnvelope(t, w); code != "empty_body" {
		t.Errorf("code = %q, want empty_body", code)
	}
}

func TestUpdateProjectNotFoundReturnsTheErrorEnvelope(t *testing.T) {
	h := newProjectHandlerForTest(newFakeProjectStore())

	w := callProjectHandler(t, h.UpdateProject, http.MethodPut, "/api/admin/projects/missing", "missing", validProjectBody)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %q)", w.Code, w.Body.String())
	}
	if code := decodeErrorEnvelope(t, w); code != "project_not_found" {
		t.Errorf("code = %q, want project_not_found", code)
	}
}

func TestDeleteProjectReturns204WithNoBody(t *testing.T) {
	h := newProjectHandlerForTest(newFakeProjectStore())

	w := callProjectHandler(t, h.CreateProject, http.MethodPost, "/api/admin/projects", "", validProjectBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup failed: status = %d (body %q)", w.Code, w.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	w = callProjectHandler(t, h.DeleteProject, http.MethodDelete, "/api/admin/projects/"+created.ID, created.ID, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body %q)", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Errorf("204 must have an empty body, got %q", w.Body.String())
	}
}

func TestDeleteProjectNotFoundReturnsTheErrorEnvelope(t *testing.T) {
	h := newProjectHandlerForTest(newFakeProjectStore())

	w := callProjectHandler(t, h.DeleteProject, http.MethodDelete, "/api/admin/projects/missing", "missing", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %q)", w.Code, w.Body.String())
	}
	if code := decodeErrorEnvelope(t, w); code != "project_not_found" {
		t.Errorf("code = %q, want project_not_found", code)
	}
}