// Package router wires configuration, database, repositories, services,
// handlers and middleware into the HTTP handler served by cmd/api and
// cmd/lambda.
package router

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/coderkamlesh/portfolio_api/internal/config"
	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/email"
	"github.com/coderkamlesh/portfolio_api/internal/handler"
	"github.com/coderkamlesh/portfolio_api/internal/middleware"
	"github.com/coderkamlesh/portfolio_api/internal/repository"
	"github.com/coderkamlesh/portfolio_api/internal/security"
	"github.com/coderkamlesh/portfolio_api/internal/service"
	"github.com/coderkamlesh/portfolio_api/internal/storage"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// newUploadHandler wires the upload service. A missing bucket is not a boot
// error: the service is built without a presigner and answers 503, so every
// other route keeps working on a machine with no AWS access.
func newUploadHandler(ctx context.Context, cfg *config.Config) (*handler.UploadHandler, error) {
	var presigner storage.Presigner
	if cfg.UploadsEnabled() {
		s3Presigner, err := storage.NewS3PresignerFromConfig(ctx, cfg)
		if err != nil {
			return nil, err
		}
		presigner = s3Presigner
	}

	return handler.NewUploadHandler(service.NewUploadService(service.UploadDeps{
		Presigner:      presigner,
		UploadPrefix:   cfg.S3UploadPrefix,
		PutPresignTTL:  cfg.S3PutPresignTTL,
		GetPresignTTL:  cfg.S3GetPresignTTL,
		MaxImageBytes:  cfg.S3MaxImageBytes,
		MaxResumeBytes: cfg.S3MaxResumeBytes,
	})), nil
}

// New builds the complete HTTP handler.
func New(ctx context.Context, cfg *config.Config, db *database.DB) (http.Handler, error) {
	mailer, err := email.NewSender(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tokens := security.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTokenTTL)
	// One recorder is shared by every content service so the audit trail has a
	// single clock and a single store.
	auditRepository := repository.NewAuditRepository(db)
	auditRecorder := service.NewAudit(auditRepository, nil)
	authService := service.NewAuthService(service.Deps{
		Admins:  repository.NewAdminRepository(db),
		TwoFA:   repository.NewTwoFARepository(db),
		OTPs:    repository.NewOTPRepository(db),
		Refresh: repository.NewRefreshTokenRepository(db),
		Audit:   repository.NewAuditRepository(db),
		Mailer:  mailer,
		Tokens:  tokens,
		Cfg:     cfg,
	})
	authHandler := handler.NewAuthHandler(authService)
	profileService := service.NewProfileService(service.ProfileDeps{
		Profiles: repository.NewProfileRepository(db),
		Audit:    auditRecorder,
	})
	profileHandler := handler.NewProfileHandler(profileService)
	skillService := service.NewSkillService(service.SkillDeps{
		Skills: repository.NewSkillRepository(db),
		Audit:    auditRecorder,
	})
	skillHandler := handler.NewSkillHandler(skillService)
	experienceService := service.NewExperienceService(service.ExperienceDeps{
		Experiences: repository.NewExperienceRepository(db),
		Audit:    auditRecorder,
	})
	experienceHandler := handler.NewExperienceHandler(experienceService)
	projectService := service.NewProjectService(service.ProjectDeps{
		Projects: repository.NewProjectRepository(db),
		Audit:    auditRecorder,
	})
	projectHandler := handler.NewProjectHandler(projectService)
	educationService := service.NewEducationService(service.EducationDeps{
		Educations: repository.NewEducationRepository(db),
		Audit:    auditRecorder,
	})
	educationHandler := handler.NewEducationHandler(educationService)
	extraService := service.NewExtraService(service.ExtraDeps{
		Extras: repository.NewExtraRepository(db),
		Audit:    auditRecorder,
	})
	extraHandler := handler.NewExtraHandler(extraService)
	socialLinkService := service.NewSocialLinkService(service.SocialLinkDeps{
		SocialLinks: repository.NewSocialLinkRepository(db),
		Audit:       auditRecorder,
	})
	socialLinkHandler := handler.NewSocialLinkHandler(socialLinkService)

	// Uploads stay optional: without a bucket the API still boots and the
	// upload routes answer 503, so a local clone needs no AWS access.
	uploadHandler, err := newUploadHandler(ctx, cfg)
	if err != nil {
		return nil, err
	}

	analyticsRepository := repository.NewAnalyticsRepository(db)
	resumeService := service.NewResumeService(service.ResumeDeps{
		Profiles:   repository.NewProfileRepository(db),
		Skills:     repository.NewSkillRepository(db),
		Experience: repository.NewExperienceRepository(db),
		Projects:   repository.NewProjectRepository(db),
		Education:  repository.NewEducationRepository(db),
		Extras:     repository.NewExtraRepository(db),
		Analytics:  analyticsRepository,
		HashSecret: cfg.AnalyticsHashSecret,
	})
	resumeHandler := handler.NewResumeHandler(resumeService)
	analyticsHandler := handler.NewAnalyticsHandler(service.NewAnalyticsService(service.AnalyticsDeps{
		Analytics: analyticsRepository,
	}))
	auditHandler := handler.NewAuditHandler(service.NewAuditQueryService(service.AuditQueryDeps{
		Audit: auditRepository,
	}))

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "portfolio-api"})
	})
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "health is good", "message": "portfolio api project running"})
	})

	r.Route("/api/auth", func(r chi.Router) {
		// Public: sign-in flow.
		r.Group(func(r chi.Router) {
			r.Use(middleware.NoStore)
			r.Post("/login", authHandler.Login)
			r.Post("/2fa/verify", authHandler.Verify2FA)
			r.Post("/2fa/resend", authHandler.Resend2FA)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/password/forgot", authHandler.ForgotPassword)
			r.Post("/password/reset", authHandler.ResetPassword)
		})

		// Admin session (JWT access token required).
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(tokens))
			r.Use(middleware.NoStore)
			r.Get("/me", authHandler.Me)
			r.Post("/logout", authHandler.Logout)
			r.Post("/password/change", authHandler.ChangePassword)
			r.Get("/2fa", authHandler.TwoFAStatus)
			r.Post("/2fa/email/enable", authHandler.EnableEmail2FA)
		})
	})

	r.Route("/api/public", func(r chi.Router) {
		r.Use(middleware.NoStore)
		r.Get("/profile", profileHandler.PublicProfile)
		r.Get("/skills", skillHandler.PublicSkills)
		r.Get("/experience", experienceHandler.PublicExperiences)
		r.Get("/projects", projectHandler.PublicProjects)
		r.Get("/projects/{id}", projectHandler.PublicProject)
		r.Get("/education", educationHandler.PublicEducations)
		r.Get("/extras", extraHandler.PublicExtras)
		r.Get("/social-links", socialLinkHandler.PublicSocialLinks)
		r.Get("/resume/download", resumeHandler.Download)
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(middleware.RequireAuth(tokens))
		r.Use(middleware.NoStore)
		r.Get("/profile", profileHandler.AdminProfile)
		r.Put("/profile", profileHandler.UpdateProfile)

		r.Get("/skill-categories", skillHandler.ListCategories)
		r.Post("/skill-categories", skillHandler.CreateCategory)
		r.Put("/skill-categories/{id}", skillHandler.UpdateCategory)
		r.Delete("/skill-categories/{id}", skillHandler.DeleteCategory)

		r.Get("/skills", skillHandler.ListSkills)
		r.Post("/skills", skillHandler.CreateSkill)
		r.Put("/skills/{id}", skillHandler.UpdateSkill)
		r.Delete("/skills/{id}", skillHandler.DeleteSkill)

		r.Get("/experience", experienceHandler.ListExperiences)
		r.Post("/experience", experienceHandler.CreateExperience)
		r.Put("/experience/{id}", experienceHandler.UpdateExperience)
		r.Delete("/experience/{id}", experienceHandler.DeleteExperience)

		r.Get("/projects", projectHandler.ListProjects)
		r.Post("/projects", projectHandler.CreateProject)
		r.Put("/projects/{id}", projectHandler.UpdateProject)
		r.Delete("/projects/{id}", projectHandler.DeleteProject)

		r.Get("/education", educationHandler.ListEducations)
		r.Post("/education", educationHandler.CreateEducation)
		r.Put("/education/{id}", educationHandler.UpdateEducation)
		r.Delete("/education/{id}", educationHandler.DeleteEducation)

		r.Get("/extras", extraHandler.ListExtras)
		r.Post("/extras", extraHandler.CreateExtra)
		r.Put("/extras/{id}", extraHandler.UpdateExtra)
		r.Delete("/extras/{id}", extraHandler.DeleteExtra)

		r.Put("/social-links", socialLinkHandler.ReplaceSocialLinks)

		r.Route("/uploads", func(r chi.Router) {
			r.Post("/presign", uploadHandler.PresignUpload)
			r.Get("/download-url", uploadHandler.PresignDownload)
		})

		r.Get("/analytics/downloads", analyticsHandler.DownloadStats)
		r.Get("/audit-log", auditHandler.Entries)
	})

	return r, nil
}

// writeJSON is the tiny local response helper for the infra endpoints; the
// auth handlers use the richer version in internal/handler.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("ÃƒÂ¢Ã…Â¡Ã‚Â ÃƒÂ¯Ã‚Â¸Ã‚Â  router: encode response failed: %v", err)
	}
}
