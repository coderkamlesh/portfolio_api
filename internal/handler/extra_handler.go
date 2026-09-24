package handler

import (
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/service"
	"github.com/go-chi/chi/v5"
)

// ExtraHandler serves the public extras list and the admin CRUD routes.
type ExtraHandler struct {
	svc *service.ExtraService
}

// NewExtraHandler builds the handler.
func NewExtraHandler(svc *service.ExtraService) *ExtraHandler {
	return &ExtraHandler{svc: svc}
}

// PublicExtras handles GET /api/public/extras. The response is grouped by
// category.
func (h *ExtraHandler) PublicExtras(w http.ResponseWriter, r *http.Request) {
	extras, err := h.svc.PublicExtras(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, extras)
}

// ListExtras handles GET /api/admin/extras.
func (h *ExtraHandler) ListExtras(w http.ResponseWriter, r *http.Request) {
	extras, err := h.svc.AdminExtras(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, extras)
}

// CreateExtra handles POST /api/admin/extras.
func (h *ExtraHandler) CreateExtra(w http.ResponseWriter, r *http.Request) {
	var body extraRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Category, "category"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Title, "title"); err != nil {
		writeErr(w, r, err)
		return
	}

	extra, err := h.svc.CreateExtra(adminContext(r), body.input())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, extra)
}

// UpdateExtra handles PUT /api/admin/extras/{id}.
func (h *ExtraHandler) UpdateExtra(w http.ResponseWriter, r *http.Request) {
	var body extraRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Category, "category"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Title, "title"); err != nil {
		writeErr(w, r, err)
		return
	}

	extra, err := h.svc.UpdateExtra(adminContext(r), chi.URLParam(r, "id"), body.input())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, extra)
}

// DeleteExtra handles DELETE /api/admin/extras/{id}.
func (h *ExtraHandler) DeleteExtra(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteExtra(adminContext(r), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}