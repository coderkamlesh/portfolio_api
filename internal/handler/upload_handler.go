package handler

import (
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/service"
)

// UploadHandler issues presigned S3 URLs. It never receives file bytes: the
// admin panel uploads straight to S3 with the returned URL.
type UploadHandler struct {
	svc *service.UploadService
}

// NewUploadHandler builds the handler.
func NewUploadHandler(svc *service.UploadService) *UploadHandler {
	return &UploadHandler{svc: svc}
}

// PresignUpload handles POST /api/admin/uploads/presign.
func (h *UploadHandler) PresignUpload(w http.ResponseWriter, r *http.Request) {
	var body uploadPresignRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}

	upload, err := h.svc.PresignUpload(r.Context(), body.input())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, upload)
}

// PresignDownload handles GET /api/admin/uploads/download-url?key=...
func (h *UploadHandler) PresignDownload(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")

	download, err := h.svc.PresignDownload(r.Context(), key)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, download)
}