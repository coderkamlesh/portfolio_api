package handler

import (
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/service"
)

// ProfileHandler serves the public and admin profile routes.
type ProfileHandler struct {
	svc *service.ProfileService
}

// NewProfileHandler builds the handler.
func NewProfileHandler(svc *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{svc: svc}
}

// PublicProfile handles GET /api/public/profile.
func (h *ProfileHandler) PublicProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.svc.PublicProfile(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// AdminProfile handles GET /api/admin/profile.
func (h *ProfileHandler) AdminProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.svc.AdminProfile(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// UpdateProfile handles PUT /api/admin/profile.
func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var body updateProfileRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.FullName, "full_name"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Title, "title"); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := requiredString(body.Email, "email"); err != nil {
		writeErr(w, r, err)
		return
	}

	profile, err := h.svc.UpdateProfile(adminContext(r), service.ProfileInput{
		FullName:        body.FullName,
		Title:           body.Title,
		Tagline:         body.Tagline,
		Summary:         body.Summary,
		Email:           body.Email,
		Phone:           body.Phone,
		Location:        body.Location,
		AvatarURL:       body.AvatarURL,
		LinkedinURL:     body.LinkedinURL,
		GithubURL:       body.GithubURL,
		PortfolioURL:    body.PortfolioURL,
		TwitterURL:      body.TwitterURL,
		ResumeFileURL:   body.ResumeFileURL,
		CareerGapNote:   body.CareerGapNote,
		ExperienceLevel: body.ExperienceLevel,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}
