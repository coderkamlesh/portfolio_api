package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// ProfileRepository reads and writes the singleton profile_details row.
type ProfileRepository struct {
	base
}

// NewProfileRepository binds the repository to the database connection.
func NewProfileRepository(db *database.DB) *ProfileRepository {
	return &ProfileRepository{base{db: db}}
}

const profileColumns = `id, singleton_key, full_name, title, tagline, summary, email, phone,
	location, avatar_url, linkedin_url, github_url, portfolio_url, twitter_url,
	resume_file_url, career_gap_note, experience_level, updated_at`

// Find loads the singleton profile, or models.ErrNotFound when it has not been
// populated yet.
func (r *ProfileRepository) Find(ctx context.Context) (*models.Profile, error) {
	p, err := scanProfile(r.db.QueryRowContext(ctx, `SELECT `+profileColumns+` FROM profile_details WHERE singleton_key = 1`))
	if err != nil {
		return nil, notFound(err)
	}
	return p, nil
}

// Upsert creates the singleton profile or replaces every editable field of the
// existing row. The row ID is preserved on update.
func (r *ProfileRepository) Upsert(ctx context.Context, p *models.Profile) error {
	const q = `INSERT INTO profile_details (
		id, singleton_key, full_name, title, tagline, summary, email, phone, location,
		avatar_url, linkedin_url, github_url, portfolio_url, twitter_url,
		resume_file_url, career_gap_note, experience_level, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(singleton_key) DO UPDATE SET
		full_name = excluded.full_name,
		title = excluded.title,
		tagline = excluded.tagline,
		summary = excluded.summary,
		email = excluded.email,
		phone = excluded.phone,
		location = excluded.location,
		avatar_url = excluded.avatar_url,
		linkedin_url = excluded.linkedin_url,
		github_url = excluded.github_url,
		portfolio_url = excluded.portfolio_url,
		twitter_url = excluded.twitter_url,
		resume_file_url = excluded.resume_file_url,
		career_gap_note = excluded.career_gap_note,
		experience_level = excluded.experience_level,
		updated_at = excluded.updated_at`

	_, err := r.db.ExecContext(ctx, q,
		p.ID, p.SingletonKey, p.FullName, p.Title, nullString(p.Tagline), nullString(p.Summary),
		p.Email, nullString(p.Phone), nullString(p.Location), nullString(p.AvatarURL),
		nullString(p.LinkedinURL), nullString(p.GithubURL), nullString(p.PortfolioURL),
		nullString(p.TwitterURL), nullString(p.ResumeFileURL), nullString(p.CareerGapNote),
		nullString(p.ExperienceLevel), database.FormatTime(p.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("profile: upsert: %w", err)
	}
	return nil
}

func scanProfile(row rowScanner) (*models.Profile, error) {
	var (
		p               models.Profile
		singletonKey    sql.NullInt64
		tagline         sql.NullString
		summary         sql.NullString
		phone           sql.NullString
		location        sql.NullString
		avatarURL       sql.NullString
		linkedinURL     sql.NullString
		githubURL       sql.NullString
		portfolioURL    sql.NullString
		twitterURL      sql.NullString
		resumeFileURL   sql.NullString
		careerGapNote   sql.NullString
		experienceLevel sql.NullString
		updatedAt       string
	)
	if err := row.Scan(
		&p.ID, &singletonKey, &p.FullName, &p.Title, &tagline, &summary, &p.Email,
		&phone, &location, &avatarURL, &linkedinURL, &githubURL, &portfolioURL,
		&twitterURL, &resumeFileURL, &careerGapNote, &experienceLevel, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("profile: scan: %w", err)
	}

	p.SingletonKey = int(singletonKey.Int64)
	p.Tagline = tagline.String
	p.Summary = summary.String
	p.Phone = phone.String
	p.Location = location.String
	p.AvatarURL = avatarURL.String
	p.LinkedinURL = linkedinURL.String
	p.GithubURL = githubURL.String
	p.PortfolioURL = portfolioURL.String
	p.TwitterURL = twitterURL.String
	p.ResumeFileURL = resumeFileURL.String
	p.CareerGapNote = careerGapNote.String
	p.ExperienceLevel = experienceLevel.String

	var err error
	if p.UpdatedAt, err = database.ParseTime(updatedAt); err != nil {
		return nil, fmt.Errorf("profile: parse updated_at: %w", err)
	}
	return &p, nil
}
