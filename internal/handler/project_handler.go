package handler

import (
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/service"
	"github.com/go-chi/chi/v5"
)

// ProjectHandler serves the public project showcase and the admin CRUD routes.
type ProjectHandler struct {
	svc *service.ProjectService
}

// NewProjectHandler builds the handler.
func NewProjectHandler(svc *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

// PublicProjects handles GET /api/public/projects.
func (h *ProjectHandler) PublicProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.svc.PublicProjects(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

// PublicProject handles GET /api/public/projects/{id}.
func (h *ProjectHandler) PublicProject(w http.ResponseWriter, r *http.Request) {
	project, err := h.svc.PublicProject(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

// ListProjects handles GET /api/admin/projects.
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.svc.AdminProjects(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

// CreateProject handles POST /api/admin/projects.
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var body projectRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Title, "title"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Description, "description"); err != nil {
		writeErr(w, r, err)
		return
	}

	project, err := h.svc.CreateProject(adminContext(r), body.input())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

// UpdateProject handles PUT /api/admin/projects/{id}.
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	var body projectRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Title, "title"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Description, "description"); err != nil {
		writeErr(w, r, err)
		return
	}

	project, err := h.svc.UpdateProject(adminContext(r), chi.URLParam(r, "id"), body.input())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

// DeleteProject handles DELETE /api/admin/projects/{id}. Its bullets are
// deleted with it.
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteProject(adminContext(r), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}