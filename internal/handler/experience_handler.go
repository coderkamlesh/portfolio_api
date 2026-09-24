package handler

import (
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/service"
	"github.com/go-chi/chi/v5"
)

// ExperienceHandler serves the public work history and the admin CRUD routes.
type ExperienceHandler struct {
	svc *service.ExperienceService
}

// NewExperienceHandler builds the handler.
func NewExperienceHandler(svc *service.ExperienceService) *ExperienceHandler {
	return &ExperienceHandler{svc: svc}
}

// PublicExperiences handles GET /api/public/experience.
func (h *ExperienceHandler) PublicExperiences(w http.ResponseWriter, r *http.Request) {
	experience, err := h.svc.PublicExperiences(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, experience)
}

// ListExperiences handles GET /api/admin/experience.
func (h *ExperienceHandler) ListExperiences(w http.ResponseWriter, r *http.Request) {
	experience, err := h.svc.AdminExperiences(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, experience)
}

// CreateExperience handles POST /api/admin/experience.
func (h *ExperienceHandler) CreateExperience(w http.ResponseWriter, r *http.Request) {
	var body experienceRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.CompanyName, "company_name"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Role, "role"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.StartDate, "start_date"); err != nil {
		writeErr(w, r, err)
		return
	}

	experience, err := h.svc.CreateExperience(adminContext(r), body.input())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, experience)
}

// UpdateExperience handles PUT /api/admin/experience/{id}.
func (h *ExperienceHandler) UpdateExperience(w http.ResponseWriter, r *http.Request) {
	var body experienceRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.CompanyName, "company_name"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Role, "role"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.StartDate, "start_date"); err != nil {
		writeErr(w, r, err)
		return
	}

	experience, err := h.svc.UpdateExperience(adminContext(r), chi.URLParam(r, "id"), body.input())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, experience)
}

// DeleteExperience handles DELETE /api/admin/experience/{id}. Its bullets are
// deleted with it.
func (h *ExperienceHandler) DeleteExperience(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteExperience(adminContext(r), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}
