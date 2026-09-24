package models

import "time"

// Project maps projects. Technologies is stored as a JSON array in TEXT, dates
// are optional calendar dates in YYYY-MM-DD form.
type Project struct {
	ID          string
	Title       string
	Tagline     string
	Description string
	ProjectType string
	Role        string
	Technologies []string
	RepoURL     string
	LiveURL     string
	ImageURL    string
	StartDate   string
	EndDate     string
	Status      string
	IsFeatured  bool
	DisplayOrder int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProjectBullet maps project_bullets. Bullets belong to one project and are
// ordered by DisplayOrder.
type ProjectBullet struct {
	ID           string
	ProjectID    string
	Text         string
	DisplayOrder int
}

// Project types accepted in projects.project_type, taken from the column comment
// in db_schema.sql. An empty value means "not specified".
const (
	ProjectTypePersonal     = "PERSONAL"
	ProjectTypeAcademic     = "ACADEMIC"
	ProjectTypeOpenSource   = "OPEN_SOURCE"
	ProjectTypeInternship   = "INTERNSHIP"
)

// ProjectTypes lists every accepted project type in a stable order.
var ProjectTypes = []string{
	ProjectTypePersonal,
	ProjectTypeAcademic,
	ProjectTypeOpenSource,
	ProjectTypeInternship,
}

// Project statuses accepted in projects.status, taken from the column comment in
// db_schema.sql. An empty value means "not specified".
const (
	ProjectStatusCompleted  = "COMPLETED"
	ProjectStatusInProgress = "IN_PROGRESS"
)

// ProjectStatuses lists every accepted project status in a stable order.
var ProjectStatuses = []string{
	ProjectStatusCompleted,
	ProjectStatusInProgress,
}