package service

import (
	"context"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// resumeTestNow is the fixed clock every resume and analytics test runs on.
var resumeTestNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// fakeAnalyticsStore records downloads and serves a summary shaped like the
// repository: one point per day across the requested range.
type fakeAnalyticsStore struct {
	downloads []*models.ResumeDownload
	summary   *DownloadSummary
	from      time.Time
	to        time.Time
}

func (s *fakeAnalyticsStore) RecordDownload(_ context.Context, download *models.ResumeDownload) error {
	s.downloads = append(s.downloads, download)
	return nil
}

func (s *fakeAnalyticsStore) DownloadTotals(_ context.Context, from, to time.Time) (*DownloadSummary, error) {
	s.from, s.to = from, to
	if s.summary != nil {
		return s.summary, nil
	}

	byDay := make([]DownloadPoint, 0)
	for day := from.UTC(); !day.After(to.UTC()); day = day.AddDate(0, 0, 1) {
		byDay = append(byDay, DownloadPoint{Date: day.Format("2006-01-02")})
	}
	return &DownloadSummary{
		ByDay: byDay,
		From:  from.UTC().Format("2006-01-02"),
		To:    to.UTC().Format("2006-01-02"),
	}, nil
}

// empty stores for the content tables the resume reads. A resume can be built
// from a profile alone, so the others deliberately report "nothing configured".
type emptySkillStore struct{}

func (emptySkillStore) ListCategories(context.Context) ([]models.SkillCategory, error) {
	return nil, nil
}
func (emptySkillStore) FindCategoryByID(context.Context, string) (*models.SkillCategory, error) {
	return nil, models.ErrNotFound
}
func (emptySkillStore) FindCategoryByName(context.Context, string) (*models.SkillCategory, error) {
	return nil, models.ErrNotFound
}
func (emptySkillStore) CreateCategory(context.Context, *models.SkillCategory) error { return nil }
func (emptySkillStore) UpdateCategory(context.Context, *models.SkillCategory) error { return nil }
func (emptySkillStore) DeleteCategory(context.Context, string) error                { return nil }
func (emptySkillStore) ListSkills(context.Context, string) ([]models.Skill, error) {
	return nil, nil
}
func (emptySkillStore) FindSkillByID(context.Context, string) (*models.Skill, error) {
	return nil, models.ErrNotFound
}
func (emptySkillStore) FindSkillByName(context.Context, string, string) (*models.Skill, error) {
	return nil, models.ErrNotFound
}
func (emptySkillStore) CreateSkill(context.Context, *models.Skill) error { return nil }
func (emptySkillStore) UpdateSkill(context.Context, *models.Skill) error { return nil }
func (emptySkillStore) DeleteSkill(context.Context, string) error      { return nil }

type emptyExperienceStore struct{}

func (emptyExperienceStore) ListExperiences(context.Context) ([]models.WorkExperience, error) {
	return nil, nil
}
func (emptyExperienceStore) FindExperienceByID(context.Context, string) (*models.WorkExperience, error) {
	return nil, models.ErrNotFound
}
func (emptyExperienceStore) ListBullets(context.Context, string) ([]models.ExperienceBullet, error) {
	return nil, nil
}
func (emptyExperienceStore) SaveExperience(context.Context, *models.WorkExperience, []models.ExperienceBullet) error {
	return nil
}
func (emptyExperienceStore) DeleteExperience(context.Context, string) error { return nil }

type emptyProjectStore struct{}

func (emptyProjectStore) ListProjects(context.Context) ([]models.Project, error) { return nil, nil }
func (emptyProjectStore) FindProjectByID(context.Context, string) (*models.Project, error) {
	return nil, models.ErrNotFound
}
func (emptyProjectStore) ListProjectBullets(context.Context, string) ([]models.ProjectBullet, error) {
	return nil, nil
}
func (emptyProjectStore) SaveProject(context.Context, *models.Project, []models.ProjectBullet) error {
	return nil
}
func (emptyProjectStore) DeleteProject(context.Context, string) error { return nil }

type emptyEducationStore struct{}

func (emptyEducationStore) ListEducations(context.Context) ([]models.Education, error) {
	return nil, nil
}
func (emptyEducationStore) FindEducationByID(context.Context, string) (*models.Education, error) {
	return nil, models.ErrNotFound
}
func (emptyEducationStore) CreateEducation(context.Context, *models.Education) error { return nil }
func (emptyEducationStore) UpdateEducation(context.Context, *models.Education) error { return nil }
func (emptyEducationStore) DeleteEducation(context.Context, string) error          { return nil }

type emptyExtraStore struct{}

func (emptyExtraStore) ListExtras(context.Context) ([]models.Extra, error) { return nil, nil }
func (emptyExtraStore) FindExtraByID(context.Context, string) (*models.Extra, error) {
	return nil, models.ErrNotFound
}
func (emptyExtraStore) CreateExtra(context.Context, *models.Extra) error { return nil }
func (emptyExtraStore) UpdateExtra(context.Context, *models.Extra) error { return nil }
func (emptyExtraStore) DeleteExtra(context.Context, string) error        { return nil }

// newResumeTestService builds a resume service over one profile and empty content.
func newResumeTestService(analytics AnalyticsStore) (*ResumeService, *fakeProfileStore) {
	profiles := &fakeProfileStore{profile: &models.Profile{
		ID:          "p-1",
		FullName:    "Kamlesh Kumar",
		Title:       "Backend Engineer",
		Email:       "kamlesh@example.com",
		Location:    "Noida, India",
		Summary:     "Backend engineer.",
		LinkedinURL: "https://linkedin.com/in/kamlesh",
	}}
	return NewResumeService(ResumeDeps{
		Profiles:   profiles,
		Skills:     emptySkillStore{},
		Experience: emptyExperienceStore{},
		Projects:   emptyProjectStore{},
		Education:  emptyEducationStore{},
		Extras:     emptyExtraStore{},
		Analytics:  analytics,
		HashSecret: "test-hash-secret",
		Now:        func() time.Time { return resumeTestNow },
	}), profiles
}

func TestResumeDownloadProducesAPDF(t *testing.T) {
	analytics := &fakeAnalyticsStore{}
	svc, _ := newResumeTestService(analytics)

	pdf, err := svc.Download(context.Background(), ResumeDownloadMeta{})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if len(pdf) < 5 || string(pdf[:5]) != "%PDF-" {
		t.Errorf("result is not a PDF: %q", string(pdf[:min(5, len(pdf))]))
	}
}

func TestResumeDownloadRecordsTheVisit(t *testing.T) {
	analytics := &fakeAnalyticsStore{}
	svc, _ := newResumeTestService(analytics)

	meta := ResumeDownloadMeta{
		IPAddress: "203.0.113.7",
		UserAgent: "Mozilla/5.0",
		Referrer:  "https://example.com",
	}
	if _, err := svc.Download(context.Background(), meta); err != nil {
		t.Fatalf("Download: %v", err)
	}

	if len(analytics.downloads) != 1 {
		t.Fatalf("got %d recorded downloads, want 1", len(analytics.downloads))
	}
	entry := analytics.downloads[0]
	if entry.IPHash == "" {
		t.Error("the IP hash must be set when a secret is configured")
	}
	if entry.IPHash == meta.IPAddress {
		t.Error("the raw IP must never be stored")
	}
	if entry.UserAgent != meta.UserAgent || entry.Referrer != meta.Referrer {
		t.Errorf("request fingerprint not stored: %+v", entry)
	}
	if !entry.DownloadedAt.Equal(resumeTestNow) {
		t.Errorf("DownloadedAt = %v, want the fixed clock", entry.DownloadedAt)
	}
}

func TestResumeDownloadTruncatesLongFingerprintFields(t *testing.T) {
	analytics := &fakeAnalyticsStore{}
	svc, _ := newResumeTestService(analytics)

	long := make([]byte, 400)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := svc.Download(context.Background(), ResumeDownloadMeta{
		UserAgent: string(long),
		Referrer:  string(long),
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}

	entry := analytics.downloads[0]
	if len(entry.UserAgent) != 255 {
		t.Errorf("user agent length = %d, want 255", len(entry.UserAgent))
	}
	if len(entry.Referrer) != 255 {
		t.Errorf("referrer length = %d, want 255", len(entry.Referrer))
	}
}

func TestHashIPIsStableAndKeyed(t *testing.T) {
	first := HashIP("203.0.113.7", "secret-a")
	if first == "" {
		t.Fatal("HashIP returned empty for a valid input")
	}
	if first != HashIP("203.0.113.7", "secret-a") {
		t.Error("the same IP and secret must produce the same hash")
	}
	if first == HashIP("203.0.113.7", "secret-b") {
		t.Error("a different secret must produce a different hash")
	}
	if first == HashIP("203.0.113.8", "secret-a") {
		t.Error("a different IP must produce a different hash")
	}
	if HashIP("", "secret-a") != "" {
		t.Error("an empty IP must not be hashed")
	}
	if HashIP("203.0.113.7", "") != "" {
		t.Error("an unset secret must disable hashing rather than use a weak default")
	}
}

func TestResumeDownloadFailsWhenTheProfileIsMissing(t *testing.T) {
	svc, profiles := newResumeTestService(&fakeAnalyticsStore{})
	// The shared fake reports not found when no profile is stored, which is
	// exactly the unconfigured-portfolio case.
	profiles.profile = nil

	_, err := svc.Download(context.Background(), ResumeDownloadMeta{})
	if err == nil {
		t.Fatal("expected an error when the profile is not configured")
	}
	if code := apierr.From(err).Code; code != "profile_not_found" {
		t.Errorf("got code %q, want profile_not_found", code)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestDownloadStatsClampsTheWindow(t *testing.T) {
	cases := []struct {
		name       string
		days       int
		wantPoints int
	}{
		{name: "zero falls back to the default", days: 0, wantPoints: defaultAnalyticsDays},
		{name: "negative falls back to the default", days: -5, wantPoints: defaultAnalyticsDays},
		{name: "explicit window", days: 7, wantPoints: 7},
		{name: "over the cap is clamped", days: 5000, wantPoints: maxAnalyticsDays},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeAnalyticsStore{}
			svc := NewAnalyticsService(AnalyticsDeps{
				Analytics: store,
				Now:       func() time.Time { return resumeTestNow },
			})

			summary, err := svc.DownloadStats(context.Background(), tc.days)
			if err != nil {
				t.Fatalf("DownloadStats: %v", err)
			}
			// The service owns the window; the repository turns it into points, so
			// the point count is the observable effect of the range it asked for.
			if len(summary.ByDay) != tc.wantPoints {
				t.Errorf("got %d day points, want %d", len(summary.ByDay), tc.wantPoints)
			}
		})
	}
}

func TestDownloadStatsWindowEndsTodayAndStartsThirtyDaysBack(t *testing.T) {
	store := &fakeAnalyticsStore{}
	svc := NewAnalyticsService(AnalyticsDeps{
		Analytics: store,
		Now:       func() time.Time { return resumeTestNow },
	})

	summary, err := svc.DownloadStats(context.Background(), 30)
	if err != nil {
		t.Fatalf("DownloadStats: %v", err)
	}
	if summary.To != "2026-09-24" {
		t.Errorf("To = %q, want today's date", summary.To)
	}
	// 30 points ending today starts 29 days earlier.
	if summary.From != "2026-08-26" {
		t.Errorf("From = %q, want 29 days before today", summary.From)
	}
	if !store.to.After(store.from) {
		t.Error("the window must end after it starts")
	}
	// The upper bound must still include the whole of today, otherwise downloads
	// made a few minutes ago would fall outside the window.
	if endOfDay := time.Date(2026, 9, 24, 23, 59, 59, 0, time.UTC); store.to.Before(endOfDay) {
		t.Errorf("window ends at %v, before the end of today", store.to)
	}
}

func TestDownloadStatsNeverReturnsNullByDay(t *testing.T) {
	store := &fakeAnalyticsStore{summary: &DownloadSummary{Total: 0, ByDay: nil}}
	svc := NewAnalyticsService(AnalyticsDeps{
		Analytics: store,
		Now:       func() time.Time { return resumeTestNow },
	})

	summary, err := svc.DownloadStats(context.Background(), 7)
	if err != nil {
		t.Fatalf("DownloadStats: %v", err)
	}
	if summary.ByDay == nil {
		t.Error("ByDay must be an empty array, not null")
	}
}

func TestAuditEntriesPaginateAndFilter(t *testing.T) {
	store := &fakeAuditStore{}
	now := resumeTestNow
	store.entries = append(store.entries,
		&models.AuditEntry{ID: "a-3", EntityType: "project", Action: AuditActionUpdate, CreatedAt: now.Add(-time.Hour)},
		&models.AuditEntry{ID: "a-2", EntityType: "project", Action: AuditActionCreate, CreatedAt: now},
		&models.AuditEntry{ID: "a-1", EntityType: "skill", Action: AuditActionCreate, CreatedAt: now.Add(-2 * time.Hour)},
	)
	svc := NewAuditQueryService(AuditQueryDeps{Audit: store})

	// Newest first.
	all, err := svc.Entries(context.Background(), "", "", 0, 0)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(all.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(all.Entries))
	}
	if all.Entries[0].ID != "a-2" {
		t.Errorf("first entry = %q, want the newest (a-2)", all.Entries[0].ID)
	}
	if all.Total != 3 {
		t.Errorf("Total = %d, want 3", all.Total)
	}

	filtered, err := svc.Entries(context.Background(), "project", "", 0, 0)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(filtered.Entries) != 2 {
		t.Errorf("got %d project entries, want 2", len(filtered.Entries))
	}
}

func TestAuditEntriesClampPagination(t *testing.T) {
	svc := NewAuditQueryService(AuditQueryDeps{Audit: &fakeAuditStore{}})

	page, err := svc.Entries(context.Background(), "", "", 0, 0)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if page.Limit != defaultAuditLimit {
		t.Errorf("default limit = %d, want %d", page.Limit, defaultAuditLimit)
	}

	page, err = svc.Entries(context.Background(), "", "", 99999, -10)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if page.Limit != maxAuditLimit {
		t.Errorf("limit = %d, want it clamped to %d", page.Limit, maxAuditLimit)
	}
	if page.Offset != 0 {
		t.Errorf("negative offset = %d, want 0", page.Offset)
	}

	page, err = svc.Entries(context.Background(), "", "", 10, 999999)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if page.Offset != maxAuditOffset {
		t.Errorf("offset = %d, want it clamped to %d", page.Offset, maxAuditOffset)
	}
}

func TestAuditEntriesRejectsAnUnknownAction(t *testing.T) {
	svc := NewAuditQueryService(AuditQueryDeps{Audit: &fakeAuditStore{}})

	_, err := svc.Entries(context.Background(), "", "DESTROY", 0, 0)
	if err == nil {
		t.Fatal("expected an error for an unknown action")
	}
	if code := apierr.From(err).Code; code != "validation_failed" {
		t.Errorf("got code %q, want validation_failed", code)
	}
}

func TestAuditRecordCapturesOldAndNewValues(t *testing.T) {
	store := &fakeAuditStore{}
	audit := NewAudit(store, func() time.Time { return resumeTestNow })
	ctx := WithActor(context.Background(), "admin-1")

	audit.Record(ctx, "project", "p-1", AuditActionUpdate,
		map[string]string{"title": "old"}, map[string]string{"title": "new"})

	if len(store.entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(store.entries))
	}
	entry := store.entries[0]
	if entry.AdminID != "admin-1" {
		t.Errorf("AdminID = %q, want the acting admin", entry.AdminID)
	}
	if entry.OldValue != `{"title":"old"}` || entry.NewValue != `{"title":"new"}` {
		t.Errorf("snapshots = %q -> %q", entry.OldValue, entry.NewValue)
	}
}

func TestAuditRecordLeavesTheUnusedSideBlank(t *testing.T) {
	store := &fakeAuditStore{}
	audit := NewAudit(store, func() time.Time { return resumeTestNow })

	audit.Record(context.Background(), "project", "p-1", AuditActionCreate, nil, map[string]string{"a": "b"})
	if got := store.entries[0].OldValue; got != "" {
		t.Errorf("OldValue on a create = %q, want empty rather than \"null\"", got)
	}
}

func TestAuditIsANoOpWithoutAStore(t *testing.T) {
	// A zero recorder must not panic, so a service can be built without audit.
	var audit *Audit
	audit.Record(context.Background(), "project", "p-1", AuditActionCreate, nil, nil)
}
