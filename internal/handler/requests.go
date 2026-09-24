package handler

import (
	"github.com/coderkamlesh/portfolio_api/internal/service"
)

// loginRequest is the body of POST /api/auth/login.
type loginRequest struct {
	// Identifier accepts either the username or the email address.
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

// verifyOTPRequest is the body of POST /api/auth/2fa/verify and
// POST /api/auth/password/reset.
type verifyOTPRequest struct {
	ChallengeID string `json:"challenge_id"`
	OTP         string `json:"otp"`
}

// resendOTPRequest is the body of POST /api/auth/2fa/resend.
type resendOTPRequest struct {
	ChallengeID string `json:"challenge_id"`
}

// refreshRequest is the body of POST /api/auth/refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// logoutRequest is the body of POST /api/auth/logout.
type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
	AllDevices   bool   `json:"all_devices"`
}

// changePasswordRequest is the body of POST /api/auth/password/change.
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// forgotPasswordRequest is the body of POST /api/auth/password/forgot.
type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// resetPasswordRequest is the body of POST /api/auth/password/reset.
type resetPasswordRequest struct {
	ChallengeID string `json:"challenge_id"`
	OTP         string `json:"otp"`
	NewPassword string `json:"new_password"`
}

// confirmPasswordRequest is the body of the 2FA confirmation endpoint.
type confirmPasswordRequest struct {
	Password string `json:"password"`
}

// updateProfileRequest is the full replacement body of PUT /api/admin/profile.
type updateProfileRequest struct {
	FullName        string `json:"full_name"`
	Title           string `json:"title"`
	Tagline         string `json:"tagline"`
	Summary         string `json:"summary"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	Location        string `json:"location"`
	AvatarURL       string `json:"avatar_url"`
	LinkedinURL     string `json:"linkedin_url"`
	GithubURL       string `json:"github_url"`
	PortfolioURL    string `json:"portfolio_url"`
	TwitterURL      string `json:"twitter_url"`
	ResumeFileURL   string `json:"resume_file_url"`
	CareerGapNote   string `json:"career_gap_note"`
	ExperienceLevel string `json:"experience_level"`
}

// skillCategoryRequest is the body of POST /api/admin/skill-categories and
// PUT /api/admin/skill-categories/{id}. An omitted display_order keeps the
// stored order on update and defaults to 0 on create.
type skillCategoryRequest struct {
	Name         string `json:"name"`
	DisplayOrder *int   `json:"display_order"`
}

// skillRequest is the body of POST /api/admin/skills and
// PUT /api/admin/skills/{id}. category_id is required on create; on update an
// omitted value keeps the current category.
type skillRequest struct {
	CategoryID   string `json:"category_id"`
	Name         string `json:"name"`
	IconSlug     string `json:"icon_slug"`
	DisplayOrder *int   `json:"display_order"`
}

// experienceRequest is the body of POST /api/admin/experience and
// PUT /api/admin/experience/{id}. Bullets are replaced wholesale, in payload
// order. An omitted display_order keeps the stored order on update and defaults
// to 0 on create.
type experienceRequest struct {
	CompanyName    string   `json:"company_name"`
	CompanyLogoURL string   `json:"company_logo_url"`
	Role           string   `json:"role"`
	EmploymentType string   `json:"employment_type"`
	Location       string   `json:"location"`
	StartDate      string   `json:"start_date"`
	EndDate        string   `json:"end_date"`
	IsCurrent      bool     `json:"is_current"`
	Technologies   []string `json:"technologies"`
	Bullets        []string `json:"bullets"`
	DisplayOrder   *int     `json:"display_order"`
}

// input converts the decoded payload into the service input.
func (b experienceRequest) input() service.ExperienceInput {
	return service.ExperienceInput{
		CompanyName:    b.CompanyName,
		CompanyLogoURL: b.CompanyLogoURL,
		Role:           b.Role,
		EmploymentType: b.EmploymentType,
		Location:       b.Location,
		StartDate:      b.StartDate,
		EndDate:        b.EndDate,
		IsCurrent:      b.IsCurrent,
		Technologies:   b.Technologies,
		Bullets:        b.Bullets,
		DisplayOrder:   b.DisplayOrder,
	}
}

// projectRequest is the body of POST /api/admin/projects and
// PUT /api/admin/projects/{id}. Bullets are replaced wholesale, in payload
// order. An omitted display_order keeps the stored order on update and defaults
// to 0 on create.
type projectRequest struct {
	Title        string   `json:"title"`
	Tagline      string   `json:"tagline"`
	Description  string   `json:"description"`
	ProjectType  string   `json:"project_type"`
	Role         string   `json:"role"`
	Technologies []string `json:"technologies"`
	RepoURL      string   `json:"repo_url"`
	LiveURL      string   `json:"live_url"`
	ImageURL     string   `json:"image_url"`
	StartDate    string   `json:"start_date"`
	EndDate      string   `json:"end_date"`
	Status       string   `json:"status"`
	IsFeatured   bool     `json:"is_featured"`
	Bullets      []string `json:"bullets"`
	DisplayOrder *int     `json:"display_order"`
}

// input converts the decoded payload into the service input.
func (b projectRequest) input() service.ProjectInput {
	return service.ProjectInput{
		Title:        b.Title,
		Tagline:      b.Tagline,
		Description:  b.Description,
		ProjectType:  b.ProjectType,
		Role:         b.Role,
		Technologies: b.Technologies,
		RepoURL:      b.RepoURL,
		LiveURL:      b.LiveURL,
		ImageURL:     b.ImageURL,
		StartDate:    b.StartDate,
		EndDate:      b.EndDate,
		Status:       b.Status,
		IsFeatured:   b.IsFeatured,
		Bullets:      b.Bullets,
		DisplayOrder: b.DisplayOrder,
	}
}
