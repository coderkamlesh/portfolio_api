package handler

import (
	"net/http"
	"strconv"

	"github.com/coderkamlesh/portfolio_api/internal/service"
)

// AuditHandler serves the admin audit trail.
type AuditHandler struct {
	svc *service.AuditQueryService
}

// NewAuditHandler builds the handler.
func NewAuditHandler(svc *service.AuditQueryService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// Entries handles GET /api/admin/audit-log?entity_type=&action=&limit=&offset=
//
// The query parameters are hints rather than a strict contract, so a malformed
// limit or offset falls back to the service default instead of failing the
// request.
func (h *AuditHandler) Entries(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	page, err := h.svc.Entries(
		r.Context(),
		query.Get("entity_type"),
		query.Get("action"),
		intQueryParam(query.Get("limit")),
		intQueryParam(query.Get("offset")),
	)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func intQueryParam(raw string) int {
	if raw == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return parsed
}
