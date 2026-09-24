package handler

import (
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/service"
)

// SocialLinkHandler serves the public link list and the admin replace endpoint.
type SocialLinkHandler struct {
	svc *service.SocialLinkService
}

// NewSocialLinkHandler builds the handler.
func NewSocialLinkHandler(svc *service.SocialLinkService) *SocialLinkHandler {
	return &SocialLinkHandler{svc: svc}
}

// PublicSocialLinks handles GET /api/public/social-links.
func (h *SocialLinkHandler) PublicSocialLinks(w http.ResponseWriter, r *http.Request) {
	links, err := h.svc.PublicSocialLinks(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, links)
}

// ReplaceSocialLinks handles PUT /api/admin/social-links. The payload carries
// the complete set; omitted platforms are removed by the transactional write.
func (h *SocialLinkHandler) ReplaceSocialLinks(w http.ResponseWriter, r *http.Request) {
	var body socialLinkReplaceRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}

	links, err := h.svc.ReplaceSocialLinks(adminContext(r), body.input())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, links)
}