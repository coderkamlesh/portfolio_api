package models

import "time"

// Education maps educations. Coursework is stored as a JSON array in TEXT.
// StartYear and EndYear are calendar years; EndYear is optional and left empty
// while the degree is still in progress.
type Education struct {
	ID            string
	Institution   string
	Degree        string
	FieldOfStudy  string
	StartYear     int
	EndYear       int
	Grade         string
	GPA           string
	Coursework    []string
	Honors        string
	DisplayOrder  int
	CreatedAt     time.Time
}