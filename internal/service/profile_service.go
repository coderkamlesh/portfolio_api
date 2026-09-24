package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
)

const singletonProfileKey = 1

// ProfileDeps wires the profile service to persistence. Now is injectable for
// deterministic tests; it defaults to UTC time.
type ProfileDeps struct {
	Profiles ProfileStore
	Now      func() time.Time
	// Audit records content changes for the admin trail. Optional.
	Audit *Audit
}

// ProfileService owns the singleton portfolio profile use-cases.
type ProfileService struct {
	profiles ProfileStore
	now      func() time.Time
	audit    *Audit
}

// NewProfileService builds the service.
func NewProfileService(deps ProfileDeps) *ProfileService {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &ProfileService{profiles: deps.Profiles, now: now, audit: deps.Audit}
}

// ProfileInput is the full replacement payload accepted by the admin profile
// endpoint. Empty optional strings clear their columns.
type ProfileInput struct {
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
}

// PublicProfileView is exposed to visitors of the portfolio site.
type PublicProfileView struct {
	FullName        string `json:"full_name"`
	Title           string `json:"title"`
	Tagline         string `json:"tagline,omitempty"`
	Summary         string `json:"summary,omitempty"`
	Email           string `json:"email"`
	Phone           string `json:"phone,omitempty"`
	Location        string `json:"location,omitempty"`
	AvatarURL       string `json:"avatar_url,omitempty"`
	LinkedinURL     string `json:"linkedin_url,omitempty"`
	GithubURL       string `json:"github_url,omitempty"`
	PortfolioURL    string `json:"portfolio_url,omitempty"`
	TwitterURL      string `json:"twitter_url,omitempty"`
	ResumeFileURL   string `json:"resume_file_url,omitempty"`
	CareerGapNote   string `json:"career_gap_note,omitempty"`
	ExperienceLevel string `json:"experience_level,omitempty"`
}

// AdminProfileView includes the persistence metadata the admin panel needs.
type AdminProfileView struct {
	PublicProfileView
	ID        string    `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PublicProfile returns the profile rendered on the public site.
func (s *ProfileService) PublicProfile(ctx context.Context) (*PublicProfileView, error) {
	profile, err := s.find(ctx)
	if err != nil {
		return nil, err
	}
	return publicProfileView(profile), nil
}

// AdminProfile returns the profile with persistence metadata for the panel.
func (s *ProfileService) AdminProfile(ctx context.Context) (*AdminProfileView, error) {
	profile, err := s.find(ctx)
	if err != nil {
		return nil, err
	}
	return adminProfileView(profile), nil
}

// UpdateProfile creates the singleton profile on first save and fully replaces
// its editable fields on later saves.
func (s *ProfileService) UpdateProfile(ctx context.Context, in ProfileInput) (*AdminProfileView, error) {
	in = normalizeProfileInput(in)
	if in.FullName == "" {
		return nil, errProfileValidation("full_name")
	}
	if in.Title == "" {
		return nil, errProfileValidation("title")
	}
	if in.Email == "" {
		return nil, errProfileValidation("email")
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return nil, errInvalidProfileEmail()
	}

	profile, err := s.profiles.Find(ctx)
	switch {
	case err == nil:
		// Keep the existing primary key and singleton marker.
	case errors.Is(err, models.ErrNotFound):
		profile = &models.Profile{ID: ids.New(), SingletonKey: singletonProfileKey}
	default:
		return nil, err
	}

	// The first save is a create and every later one is an update, so the
	// previous row is captured here to decide which audit action to record.
	existed := err == nil
	var previous *models.Profile
	if existed {
		snapshot := *profile
		previous = &snapshot
	}

	profile.FullName = in.FullName
	profile.Title = in.Title
	profile.Tagline = in.Tagline
	profile.Summary = in.Summary
	profile.Email = in.Email
	profile.Phone = in.Phone
	profile.Location = in.Location
	profile.AvatarURL = in.AvatarURL
	profile.LinkedinURL = in.LinkedinURL
	profile.GithubURL = in.GithubURL
	profile.PortfolioURL = in.PortfolioURL
	profile.TwitterURL = in.TwitterURL
	profile.ResumeFileURL = in.ResumeFileURL
	profile.CareerGapNote = in.CareerGapNote
	profile.ExperienceLevel = in.ExperienceLevel
	profile.UpdatedAt = s.now()

	if err := s.profiles.Upsert(ctx, profile); err != nil {
		return nil, err
	}
	if existed {
		s.audit.Record(ctx, auditEntityProfile, profile.ID, AuditActionUpdate, previous, profile)
	} else {
		s.audit.Record(ctx, auditEntityProfile, profile.ID, AuditActionCreate, nil, profile)
	}
	return adminProfileView(profile), nil
}

func (s *ProfileService) find(ctx context.Context) (*models.Profile, error) {
	profile, err := s.profiles.Find(ctx)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errProfileNotFound()
		}
		return nil, err
	}
	return profile, nil
}

func normalizeProfileInput(in ProfileInput) ProfileInput {
	in.FullName = strings.TrimSpace(in.FullName)
	in.Title = strings.TrimSpace(in.Title)
	in.Tagline = strings.TrimSpace(in.Tagline)
	in.Summary = strings.TrimSpace(in.Summary)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Phone = strings.TrimSpace(in.Phone)
	in.Location = strings.TrimSpace(in.Location)
	in.AvatarURL = strings.TrimSpace(in.AvatarURL)
	in.LinkedinURL = strings.TrimSpace(in.LinkedinURL)
	in.GithubURL = strings.TrimSpace(in.GithubURL)
	in.PortfolioURL = strings.TrimSpace(in.PortfolioURL)
	in.TwitterURL = strings.TrimSpace(in.TwitterURL)
	in.ResumeFileURL = strings.TrimSpace(in.ResumeFileURL)
	in.CareerGapNote = strings.TrimSpace(in.CareerGapNote)
	in.ExperienceLevel = strings.TrimSpace(in.ExperienceLevel)
	return in
}

func publicProfileView(p *models.Profile) *PublicProfileView {
	return &PublicProfileView{
		FullName:        p.FullName,
		Title:           p.Title,
		Tagline:         p.Tagline,
		Summary:         p.Summary,
		Email:           p.Email,
		Phone:           p.Phone,
		Location:        p.Location,
		AvatarURL:       p.AvatarURL,
		LinkedinURL:     p.LinkedinURL,
		GithubURL:       p.GithubURL,
		PortfolioURL:    p.PortfolioURL,
		TwitterURL:      p.TwitterURL,
		ResumeFileURL:   p.ResumeFileURL,
		CareerGapNote:   p.CareerGapNote,
		ExperienceLevel: p.ExperienceLevel,
	}
}

func adminProfileView(p *models.Profile) *AdminProfileView {
	public := publicProfileView(p)
	return &AdminProfileView{
		PublicProfileView: *public,
		ID:                p.ID,
		UpdatedAt:         p.UpdatedAt,
	}
}
