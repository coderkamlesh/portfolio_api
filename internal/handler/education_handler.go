package handler

import (
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/service"
	"github.com/go-chi/chi/v5"
)

// EducationHandler serves the public education list and the admin CRUD routes.
type EducationHandler struct {
	svc *service.EducationService
}

// NewEducationHandler builds the handler.
func NewEducationHandler(svc *service.EducationService) *EducationHandler {
	return &EducationHandler{svc: svc}
}

// PublicEducations handles GET /api/public/education.
func (h *EducationHandler) PublicEducations(w http.ResponseWriter, r *http.Request) {
	education, err := h.svc.PublicEducations(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, education)
}

// ListEducations handles GET /api/admin/education.
func (h *EducationHandler) ListEducations(w http.ResponseWriter, r *http.Request) {
	education, err := h.svc.AdminEducations(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, education)
}

// CreateEducation handles POST /api/admin/education.
func (h *EducationHandler) CreateEducation(w http.ResponseWriter, r *http.Request) {
	var body educationRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Institution, "institution"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Degree, "degree"); err != nil {
		writeErr(w, r, err)
		return
	}

	education, err := h.svc.CreateEducation(r.Context(), body.input())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, education)
}

// UpdateEducation handles PUT /api/admin/education/{id}.
func (h *EducationHandler) UpdateEducation(w http.ResponseWriter, r *http.Request) {
	var body educationRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Institution, "institution"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Degree, "degree"); err != nil {
		writeErr(w, r, err)
		return
	}

	education, err := h.svc.UpdateEducation(r.Context(), chi.URLParam(r, "id"), body.input())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, education)
}

// DeleteEducation handles DELETE /api/admin/education/{id}.
func (h *EducationHandler) DeleteEducation(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteEducation(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}