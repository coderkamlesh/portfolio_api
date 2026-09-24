package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
	"github.com/coderkamlesh/portfolio_api/internal/resume"
)

// ResumeDeps wires the resume service. It needs read access to every content
// table because the PDF is assembled from all of them on each request.
type ResumeDeps struct {
	Profiles   ProfileStore
	Skills     SkillStore
	Experience ExperienceStore
	Projects   ProjectStore
	Education  EducationStore
	Extras     ExtraStore
	Analytics  AnalyticsStore
	// HashSecret keys the IP hash used by the download analytics. It is a
	// dedicated secret rather than the JWT secret: reusing a signing key for
	// visitor tracking would widen the blast radius of either leak.
	HashSecret string
	Now        func() time.Time
}

// ResumeService assembles the resume data and renders the PDF.
type ResumeService struct {
	profiles   ProfileStore
	skills     SkillStore
	experience ExperienceStore
	projects   ProjectStore
	education  EducationStore
	extras     ExtraStore
	analytics  AnalyticsStore
	hashSecret string
	now        func() time.Time
}

// NewResumeService builds the service.
func NewResumeService(deps ResumeDeps) *ResumeService {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &ResumeService{
		profiles:   deps.Profiles,
		skills:     deps.Skills,
		experience: deps.Experience,
		projects:   deps.Projects,
		education:  deps.Education,
		extras:     deps.Extras,
		analytics:  deps.Analytics,
		hashSecret: deps.HashSecret,
		now:        now,
	}
}

// ResumeDownloadMeta carries the request fingerprint stored with a download.
type ResumeDownloadMeta struct {
	IPAddress string
	UserAgent string
	Referrer  string
}

// Download renders the resume PDF and records the download.
//
// The write is best effort by design: a visitor must still receive the PDF when
// the analytics insert fails, so a failure there is logged and swallowed.
func (s *ResumeService) Download(ctx context.Context, meta ResumeDownloadMeta) ([]byte, error) {
	data, err := s.assemble(ctx)
	if err != nil {
		return nil, err
	}

	pdf, err := resume.Render(data)
	if err != nil {
		return nil, wrapErr(err, 500, "resume_render_failed", "Could not build the resume.")
	}

	s.recordDownload(ctx, meta)
	return pdf, nil
}

func (s *ResumeService) recordDownload(ctx context.Context, meta ResumeDownloadMeta) {
	if s.analytics == nil {
		return
	}
	entry := &models.ResumeDownload{
		ID:           ids.New(),
		DownloadedAt: s.now(),
		IPHash:       HashIP(meta.IPAddress, s.hashSecret),
		UserAgent:    truncateTo(meta.UserAgent, 255),
		Referrer:     truncateTo(meta.Referrer, 255),
	}
	if err := s.analytics.RecordDownload(ctx, entry); err != nil {
		log.Printf("resume: record download: %v", err)
	}
}

// HashIP returns a stable identifier for an IP address. An empty address or an
// unset secret yields an empty hash, so a deployment without analytics
// configured does not collapse every untracked visitor onto one identifier.
func HashIP(ip, secret string) string {
	if ip == "" || secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}

func truncateTo(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

// assemble reads every content table and maps it onto the template contract.
// A table that is empty simply contributes no section; only a missing profile is
// an error, because a resume without a name is not worth rendering.
func (s *ResumeService) assemble(ctx context.Context) (resume.Data, error) {
	data := resume.Data{}

	profile, err := s.profiles.Find(ctx)
	if err != nil {
		// A missing profile is a configuration gap, not a server fault, so the
		// not-found is translated rather than surfacing as a 500.
		if errors.Is(err, models.ErrNotFound) {
			return data, errProfileNotFound()
		}
		return data, err
	}
	data.Profile = resume.Profile{
		FullName:  profile.FullName,
		Title:     profile.Title,
		Summary:   profile.Summary,
		Email:     profile.Email,
		Phone:     profile.Phone,
		Location:  profile.Location,
		LinkedIn:  profile.LinkedinURL,
		Github:    profile.GithubURL,
		Portfolio: profile.PortfolioURL,
		Twitter:   profile.TwitterURL,
	}

	skills, err := s.skills.ListSkills(ctx, "")
	if err != nil {
		return data, err
	}
	categories, err := s.skills.ListCategories(ctx)
	if err != nil {
		return data, err
	}
	// Group by the category list so the resume follows display_order rather than
	// whatever order the skills query returned.
	byCategory := make(map[string][]string, len(categories))
	for i := range skills {
		byCategory[skills[i].CategoryID] = append(byCategory[skills[i].CategoryID], skills[i].Name)
	}
	for i := range categories {
		names := byCategory[categories[i].ID]
		if len(names) == 0 {
			continue
		}
		data.Skills = append(data.Skills, resume.SkillGroup{
			Category: categories[i].Name,
			Skills:   names,
		})
	}

	experiences, err := s.experience.ListExperiences(ctx)
	if err != nil {
		return data, err
	}
	experienceBullets, err := s.experience.ListBullets(ctx, "")
	if err != nil {
		return data, err
	}
	bulletsByExperience := groupBulletTexts(experienceBullets)
	for i := range experiences {
		entry := experiences[i]
		data.Experience = append(data.Experience, resume.Experience{
			Company:   entry.CompanyName,
			Role:      entry.Role,
			Location:  entry.Location,
			StartDate: entry.StartDate,
			EndDate:   entry.EndDate,
			IsCurrent: entry.IsCurrent,
			Bullets:   bulletsByExperience[entry.ID],
		})
	}

	projects, err := s.projects.ListProjects(ctx)
	if err != nil {
		return data, err
	}
	projectBullets, err := s.projects.ListProjectBullets(ctx, "")
	if err != nil {
		return data, err
	}
	projectBulletTexts := make(map[string][]string, len(projects))
	for i := range projectBullets {
		projectBulletTexts[projectBullets[i].ProjectID] = append(
			projectBulletTexts[projectBullets[i].ProjectID], projectBullets[i].Text)
	}
	for i := range projects {
		entry := projects[i]
		// An in-progress project has no end date; the template shows "Present".
		isCurrent := entry.Status == models.ProjectStatusInProgress
		data.Projects = append(data.Projects, resume.Project{
			Title:        entry.Title,
			Role:         entry.Role,
			Technologies: entry.Technologies,
			RepoURL:      entry.RepoURL,
			LiveURL:      entry.LiveURL,
			StartDate:    entry.StartDate,
			EndDate:      entry.EndDate,
			IsCurrent:    isCurrent,
			Description:  entry.Description,
			Bullets:      projectBulletTexts[entry.ID],
		})
	}

	educations, err := s.education.ListEducations(ctx)
	if err != nil {
		return data, err
	}
	for i := range educations {
		entry := educations[i]
		data.Education = append(data.Education, resume.Education{
			Institution:  entry.Institution,
			Degree:       entry.Degree,
			FieldOfStudy: entry.FieldOfStudy,
			StartYear:    entry.StartYear,
			EndYear:      entry.EndYear,
			// end_year is stored as NULL while the degree is unfinished, which
			// the model flattens to zero.
			IsCurrent: entry.EndYear == 0,
			GPA:       entry.GPA,
			Honors:    entry.Honors,
		})
	}

	extras, err := s.extras.ListExtras(ctx)
	if err != nil {
		return data, err
	}
	data.Extras = groupExtras(extras)

	return data, nil
}

func groupBulletTexts(bullets []models.ExperienceBullet) map[string][]string {
	out := make(map[string][]string)
	for i := range bullets {
		out[bullets[i].ExperienceID] = append(out[bullets[i].ExperienceID], bullets[i].Text)
	}
	return out
}

// groupExtras buckets entries by category, emitting the groups in the order of
// models.ExtraCategories so the resume keeps a stable section order.
func groupExtras(extras []models.Extra) []resume.ExtraGroup {
	byCategory := make(map[string][]resume.Extra, len(models.ExtraCategories))
	for i := range extras {
		byCategory[extras[i].Category] = append(byCategory[extras[i].Category], resume.Extra{
			Title:      extras[i].Title,
			Issuer:     extras[i].Issuer,
			IssuedDate: extras[i].IssuedDate,
		})
	}

	groups := make([]resume.ExtraGroup, 0, len(byCategory))
	for _, category := range models.ExtraCategories {
		entries := byCategory[category]
		if len(entries) == 0 {
			continue
		}
		groups = append(groups, resume.ExtraGroup{Category: category, Entries: entries})
	}
	return groups
}
