package handler

import (
	"net/http"
	"strconv"

	"github.com/coderkamlesh/portfolio_api/internal/service"
)

// ResumeHandler streams the generated resume PDF.
type ResumeHandler struct {
	svc *service.ResumeService
}

// NewResumeHandler builds the handler.
func NewResumeHandler(svc *service.ResumeService) *ResumeHandler {
	return &ResumeHandler{svc: svc}
}

// Download handles GET /api/public/resume/download. The body is the PDF itself
// rather than JSON, so the route skips the writeJSON helper.
func (h *ResumeHandler) Download(w http.ResponseWriter, r *http.Request) {
	pdf, err := h.svc.Download(r.Context(), service.ResumeDownloadMeta{
		IPAddress: clientIP(r),
		UserAgent: r.UserAgent(),
		Referrer:  r.Referer(),
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}

	// A descriptive filename helps the recruiter keep versions apart, which the
	// guidance calls out as a last-check item.
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="resume.pdf"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	w.WriteHeader(http.StatusOK)
	// A write error after the header is already sent cannot be reported, so the
	// response is simply truncated; the client sees a short download.
	_, _ = w.Write(pdf)
}

// AnalyticsHandler serves the admin download dashboard.
type AnalyticsHandler struct {
	svc *service.AnalyticsService
}

// NewAnalyticsHandler builds the handler.
func NewAnalyticsHandler(svc *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc}
}

// DownloadStats handles GET /api/admin/analytics/downloads?days=30
func (h *AnalyticsHandler) DownloadStats(w http.ResponseWriter, r *http.Request) {
	days := 0
	if raw := r.URL.Query().Get("days"); raw != "" {
		// A non-numeric value falls back to the service default rather than
		// erroring, since this is a dashboard hint rather than a data contract.
		if parsed, err := strconv.Atoi(raw); err == nil {
			days = parsed
		}
	}

	summary, err := h.svc.DownloadStats(r.Context(), days)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
