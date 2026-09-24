package models

import "time"

// SkillCategory maps skill_categories. Skills are grouped under a category and
// rendered in display_order.
type SkillCategory struct {
	ID           string
	Name         string
	DisplayOrder int
	CreatedAt    time.Time
}

// Skill maps skills. Every row belongs to exactly one category and the pair
// (category_id, skill_name) is unique.
type Skill struct {
	ID           string
	CategoryID   string
	Name         string
	IconSlug     string
	DisplayOrder int
	CreatedAt    time.Time
}
