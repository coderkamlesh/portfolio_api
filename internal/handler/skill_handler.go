package handler

import (
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/service"
	"github.com/go-chi/chi/v5"
)

// SkillHandler serves the public skills feed and the admin CRUD routes for
// skill categories and skills.
type SkillHandler struct {
	svc *service.SkillService
}

// NewSkillHandler builds the handler.
func NewSkillHandler(svc *service.SkillService) *SkillHandler {
	return &SkillHandler{svc: svc}
}

// PublicSkills handles GET /api/public/skills.
func (h *SkillHandler) PublicSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := h.svc.PublicSkills(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

// ListCategories handles GET /api/admin/skill-categories.
func (h *SkillHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.svc.AdminCategories(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, categories)
}

// CreateCategory handles POST /api/admin/skill-categories.
func (h *SkillHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var body skillCategoryRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Name, "name"); err != nil {
		writeErr(w, r, err)
		return
	}

	category, err := h.svc.CreateCategory(r.Context(), service.CategoryInput{
		Name:         body.Name,
		DisplayOrder: body.DisplayOrder,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, category)
}

// UpdateCategory handles PUT /api/admin/skill-categories/{id}.
func (h *SkillHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	var body skillCategoryRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Name, "name"); err != nil {
		writeErr(w, r, err)
		return
	}

	category, err := h.svc.UpdateCategory(r.Context(), chi.URLParam(r, "id"), service.CategoryInput{
		Name:         body.Name,
		DisplayOrder: body.DisplayOrder,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, category)
}

// DeleteCategory handles DELETE /api/admin/skill-categories/{id}. Its skills are
// deleted with it.
func (h *SkillHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteCategory(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}

// ListSkills handles GET /api/admin/skills.
func (h *SkillHandler) ListSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := h.svc.AdminSkills(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

// CreateSkill handles POST /api/admin/skills.
func (h *SkillHandler) CreateSkill(w http.ResponseWriter, r *http.Request) {
	var body skillRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.CategoryID, "category_id"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Name, "name"); err != nil {
		writeErr(w, r, err)
		return
	}

	skill, err := h.svc.CreateSkill(r.Context(), service.SkillInput{
		CategoryID:   body.CategoryID,
		Name:         body.Name,
		IconSlug:     body.IconSlug,
		DisplayOrder: body.DisplayOrder,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, skill)
}

// UpdateSkill handles PUT /api/admin/skills/{id}.
func (h *SkillHandler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	var body skillRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Name, "name"); err != nil {
		writeErr(w, r, err)
		return
	}

	skill, err := h.svc.UpdateSkill(r.Context(), chi.URLParam(r, "id"), service.SkillInput{
		CategoryID:   body.CategoryID,
		Name:         body.Name,
		IconSlug:     body.IconSlug,
		DisplayOrder: body.DisplayOrder,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

// DeleteSkill handles DELETE /api/admin/skills/{id}.
func (h *SkillHandler) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteSkill(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}
