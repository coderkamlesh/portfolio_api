package models

import "time"

// WorkExperience maps work_experiences. Dates are calendar dates in YYYY-MM-DD
// form and Technologies is stored as a JSON array in TEXT.
type WorkExperience struct {
	ID             string
	CompanyName    string
	CompanyLogoURL string
	Role           string
	EmploymentType string
	Location       string
	StartDate      string
	EndDate        string
	IsCurrent      bool
	Technologies   []string
	DisplayOrder   int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ExperienceBullet maps experience_bullets. Bullets belong to one experience
// and are ordered by DisplayOrder.
type ExperienceBullet struct {
	ID           string
	ExperienceID string
	Text         string
	DisplayOrder int
}

// Employment types accepted in work_experiences.employment_type, taken from the
// column comment in db_schema.sql. An empty value means "not specified".
const (
	EmploymentFullTime = "FULL_TIME"
	EmploymentContract = "CONTRACT"
	EmploymentIntern   = "INTERN"
	EmploymentPartTime = "PART_TIME"
	EmploymentRemote   = "REMOTE"
)

// EmploymentTypes lists every accepted employment type in a stable order.
var EmploymentTypes = []string{
	EmploymentFullTime,
	EmploymentContract,
	EmploymentIntern,
	EmploymentPartTime,
	EmploymentRemote,
}
