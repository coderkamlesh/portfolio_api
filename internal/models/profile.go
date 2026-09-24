package models

import "time"

// Profile maps profile_details. The table is constrained to one row through
// SingletonKey, so the API always treats this as the portfolio owner's profile.
type Profile struct {
	ID              string
	SingletonKey    int
	FullName        string
	Title           string
	Tagline         string
	Summary         string
	Email           string
	Phone           string
	Location        string
	AvatarURL       string
	LinkedinURL     string
	GithubURL       string
	PortfolioURL    string
	TwitterURL      string
	ResumeFileURL   string
	CareerGapNote   string
	ExperienceLevel string
	UpdatedAt       time.Time
}
