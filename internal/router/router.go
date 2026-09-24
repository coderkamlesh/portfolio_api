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
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// New builds the complete HTTP handler.
func New(ctx context.Context, cfg *config.Config, db *database.DB) (http.Handler, error) {
	mailer, err := email.NewSender(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tokens := security.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTokenTTL)
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
	})
	profileHandler := handler.NewProfileHandler(profileService)
	skillService := service.NewSkillService(service.SkillDeps{
		Skills: repository.NewSkillRepository(db),
	})
	skillHandler := handler.NewSkillHandler(skillService)
	experienceService := service.NewExperienceService(service.ExperienceDeps{
		Experiences: repository.NewExperienceRepository(db),
	})
	experienceHandler := handler.NewExperienceHandler(experienceService)
	projectService := service.NewProjectService(service.ProjectDeps{
		Projects: repository.NewProjectRepository(db),
	})
	projectHandler := handler.NewProjectHandler(projectService)
	educationService := service.NewEducationService(service.EducationDeps{
		Educations: repository.NewEducationRepository(db),
	})
	educationHandler := handler.NewEducationHandler(educationService)
	extraService := service.NewExtraService(service.ExtraDeps{
		Extras: repository.NewExtraRepository(db),
	})
	extraHandler := handler.NewExtraHandler(extraService)

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
	})

	return r, nil
}

// writeJSON is the tiny local response helper for the infra endpoints; the
// auth handlers use the richer version in internal/handler.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("⚠️  router: encode response failed: %v", err)
	}
}
