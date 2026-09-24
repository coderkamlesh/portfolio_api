package handler

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
