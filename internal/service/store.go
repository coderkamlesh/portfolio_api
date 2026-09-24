package service

import (
	"context"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// AdminStore is the persistence contract for admin_users.
type AdminStore interface {
	FindByID(ctx context.Context, id string) (*models.AdminUser, error)
	FindByIdentifier(ctx context.Context, identifier string) (*models.AdminUser, error)
	TouchLastLogin(ctx context.Context, id string, at time.Time) error
	UpdatePassword(ctx context.Context, id, passwordHash string, at time.Time) error
}

// TwoFAStore is the persistence contract for admin_2fa.
type TwoFAStore interface {
	FindByAdminAndMethod(ctx context.Context, adminID, method string) (*models.TwoFactorConfig, error)
	ListByAdmin(ctx context.Context, adminID string) ([]models.TwoFactorConfig, error)
	Upsert(ctx context.Context, cfg *models.TwoFactorConfig) error
	SetEnabled(ctx context.Context, adminID, method string, enabled bool, at time.Time) error
}

// OTPStore is the persistence contract for otp_challenges.
type OTPStore interface {
	Create(ctx context.Context, c *models.OTPChallenge) error
	FindByID(ctx context.Context, id string) (*models.OTPChallenge, error)
	FindLatestPending(ctx context.Context, adminID, purpose string, at time.Time) (*models.OTPChallenge, error)
	CountCreatedSince(ctx context.Context, adminID, purpose string, since time.Time) (int, error)
	InvalidatePending(ctx context.Context, adminID, purpose string, at time.Time) error
	IncrementAttempts(ctx context.Context, id string) (int, error)
	Consume(ctx context.Context, id string, at time.Time) (bool, error)
	DeleteOlderThan(ctx context.Context, before time.Time) error
}

// RefreshStore is the persistence contract for refresh_tokens.
type RefreshStore interface {
	Create(ctx context.Context, t *models.RefreshToken) error
	FindByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	Revoke(ctx context.Context, id string, at time.Time) (bool, error)
	RevokeAllForAdmin(ctx context.Context, adminID string, at time.Time) (int64, error)
	CountActive(ctx context.Context, adminID string, at time.Time) (int, error)
	DeleteExpired(ctx context.Context, before time.Time) error
}

// AuditStore is the persistence contract for audit_log.
type AuditStore interface {
	Insert(ctx context.Context, e *models.AuditEntry) error
}

// ProfileStore is the persistence contract for the singleton profile_details row.
type ProfileStore interface {
	Find(ctx context.Context) (*models.Profile, error)
	Upsert(ctx context.Context, profile *models.Profile) error
}

// SkillStore is the persistence contract for skill_categories and skills.
type SkillStore interface {
	ListCategories(ctx context.Context) ([]models.SkillCategory, error)
	FindCategoryByID(ctx context.Context, id string) (*models.SkillCategory, error)
	FindCategoryByName(ctx context.Context, name string) (*models.SkillCategory, error)
	CreateCategory(ctx context.Context, category *models.SkillCategory) error
	UpdateCategory(ctx context.Context, category *models.SkillCategory) error
	DeleteCategory(ctx context.Context, id string) error

	// ListSkills returns the skills of one category, or of every category when
	// categoryID is empty.
	ListSkills(ctx context.Context, categoryID string) ([]models.Skill, error)
	FindSkillByID(ctx context.Context, id string) (*models.Skill, error)
	FindSkillByName(ctx context.Context, categoryID, name string) (*models.Skill, error)
	CreateSkill(ctx context.Context, skill *models.Skill) error
	UpdateSkill(ctx context.Context, skill *models.Skill) error
	DeleteSkill(ctx context.Context, id string) error
}

// ExperienceStore is the persistence contract for work_experiences and
// experience_bullets.
type ExperienceStore interface {
	ListExperiences(ctx context.Context) ([]models.WorkExperience, error)
	FindExperienceByID(ctx context.Context, id string) (*models.WorkExperience, error)
	// ListBullets returns the bullets of one experience, or the bullets of every
	// experience when experienceID is empty.
	ListBullets(ctx context.Context, experienceID string) ([]models.ExperienceBullet, error)
	// SaveExperience writes the row and replaces its bullets in one transaction.
	SaveExperience(ctx context.Context, experience *models.WorkExperience, bullets []models.ExperienceBullet) error
	// DeleteExperience removes the row together with its bullets in one
	// transaction.
	DeleteExperience(ctx context.Context, id string) error
}
