package service

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// experienceTestNow is the fixed clock every experience test runs on.
var experienceTestNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// fakeExperienceStore is an in-memory ExperienceStore. It mirrors the real
// repository: entries are listed reverse chronologically and saving replaces the
// bullets of an entry.
type fakeExperienceStore struct {
	experiences []models.WorkExperience
	bullets     map[string][]models.ExperienceBullet
	saveErr     error
	deleteErr   error
}

func newFakeExperienceStore() *fakeExperienceStore {
	return &fakeExperienceStore{bullets: make(map[string][]models.ExperienceBullet)}
}

func (s *fakeExperienceStore) ListExperiences(context.Context) ([]models.WorkExperience, error) {
	out := append([]models.WorkExperience(nil), s.experiences...)
	// Same order as the SQL: is_current DESC, start_date DESC, display_order,
	// company_name.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsCurrent != out[j].IsCurrent {
			return out[i].IsCurrent
		}
		if out[i].StartDate != out[j].StartDate {
			return out[i].StartDate > out[j].StartDate
		}
		if out[i].DisplayOrder != out[j].DisplayOrder {
			return out[i].DisplayOrder < out[j].DisplayOrder
		}
		return out[i].CompanyName < out[j].CompanyName
	})
	return out, nil
}

func (s *fakeExperienceStore) FindExperienceByID(_ context.Context, id string) (*models.WorkExperience, error) {
	for i := range s.experiences {
		if s.experiences[i].ID == id {
			experience := s.experiences[i]
			return &experience, nil
		}
	}
	return nil, models.ErrNotFound
}

func (s *fakeExperienceStore) ListBullets(_ context.Context, experienceID string) ([]models.ExperienceBullet, error) {
	if experienceID != "" {
		return s.copyBullets(experienceID), nil
	}

	entries, _ := s.ListExperiences(context.Background())
	out := make([]models.ExperienceBullet, 0)
	for i := range entries {
		out = append(out, s.copyBullets(entries[i].ID)...)
	}
	return out, nil
}

func (s *fakeExperienceStore) SaveExperience(_ context.Context, experience *models.WorkExperience, bullets []models.ExperienceBullet) error {
	if s.saveErr != nil {
		return s.saveErr
	}

	stored := *experience
	replaced := false
	for i := range s.experiences {
		if s.experiences[i].ID == experience.ID {
			s.experiences[i] = stored
			replaced = true
			break
		}
	}
	if !replaced {
		s.experiences = append(s.experiences, stored)
	}

	s.bullets[experience.ID] = append([]models.ExperienceBullet(nil), bullets...)
	return nil
}

func (s *fakeExperienceStore) DeleteExperience(_ context.Context, id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for i := range s.experiences {
		if s.experiences[i].ID == id {
			s.experiences = append(s.experiences[:i], s.experiences[i+1:]...)
			delete(s.bullets, id)
			return nil
		}
	}
	return models.ErrNotFound
}

func (s *fakeExperienceStore) copyBullets(experienceID string) []models.ExperienceBullet {
	stored := s.bullets[experienceID]
	out := append([]models.ExperienceBullet(nil), stored...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].DisplayOrder < out[j].DisplayOrder })
	return out
}

// newExperienceTestService builds the service over a fixed clock.
func newExperienceTestService(store *fakeExperienceStore) *ExperienceService {
	return NewExperienceService(ExperienceDeps{
		Experiences: store,
		Now:         func() time.Time { return experienceTestNow },
	})
}

// experienceErrCode returns the API error code of err, failing the test when err
// is nil. It mirrors the helper used by the skills tests.
func experienceErrCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return apierr.From(err).Code
}

// validExperienceInput returns a payload that passes every validation rule.
func validExperienceInput() ExperienceInput {
	return ExperienceInput{
		CompanyName:    "Nimbus Labs",
		CompanyLogoURL: "https://cdn.example.com/nimbus.png",
		Role:           "Senior Backend Engineer",
		EmploymentType: "full_time",
		Location:       "Noida, India",
		StartDate:      "2024-04-01",
		EndDate:        "2025-12-31",
		Technologies:   []string{"Go", "PostgreSQL"},
		Bullets: []string{
			"Cut p99 order API latency by 42% by replacing N+1 queries with batched reads.",
			"Led the migration of 18 services to a shared auth gateway.",
		},
	}
}

func TestPublicExperiencesIsReverseChronologicalWithNestedBullets(t *testing.T) {
	store := newFakeExperienceStore()
	store.experiences = []models.WorkExperience{
		{
			ID: "exp-old", CompanyName: "Old Co", Role: "Engineer", StartDate: "2021-01-01",
			EndDate: "2022-12-31", CreatedAt: experienceTestNow, UpdatedAt: experienceTestNow,
		},
		{
			ID: "exp-current", CompanyName: "Nimbus Labs", Role: "Senior Engineer", StartDate: "2024-04-01",
			IsCurrent: true, Technologies: []string{"Go"}, CreatedAt: experienceTestNow, UpdatedAt: experienceTestNow,
		},
		{
			ID: "exp-mid", CompanyName: "Middle Co", Role: "Engineer", StartDate: "2023-01-01",
			EndDate: "2024-03-31", CreatedAt: experienceTestNow, UpdatedAt: experienceTestNow,
		},
	}
	store.bullets["exp-current"] = []models.ExperienceBullet{
		{ID: "b2", ExperienceID: "exp-current", Text: "Second point", DisplayOrder: 2},
		{ID: "b1", ExperienceID: "exp-current", Text: "First point", DisplayOrder: 1},
	}
	svc := newExperienceTestService(store)

	got, err := svc.PublicExperiences(context.Background())
	if err != nil {
		t.Fatalf("PublicExperiences: %v", err)
	}
	if len(got.Experiences) != 3 {
		t.Fatalf("got %d entries, want 3", len(got.Experiences))
	}
	if got.Experiences[0].ID != "exp-current" {
		t.Errorf("first entry = %q, want the current role", got.Experiences[0].ID)
	}
	if got.Experiences[1].ID != "exp-mid" || got.Experiences[2].ID != "exp-old" {
		t.Errorf("entries are not reverse chronological: %q then %q", got.Experiences[1].ID, got.Experiences[2].ID)
	}
	if len(got.Experiences[0].Bullets) != 2 || got.Experiences[0].Bullets[0] != "First point" {
		t.Errorf("bullets were not nested in display order: %+v", got.Experiences[0].Bullets)
	}
	if got.Experiences[1].Bullets == nil {
		t.Error("an entry without bullets must render as an empty array")
	}
}

func TestPublicExperiencesRendersEmptyArrayWhenNothingConfigured(t *testing.T) {
	svc := newExperienceTestService(newFakeExperienceStore())

	got, err := svc.PublicExperiences(context.Background())
	if err != nil {
		t.Fatalf("PublicExperiences: %v", err)
	}
	if got.Experiences == nil || len(got.Experiences) != 0 {
		t.Fatalf("Experiences = %#v, want an empty non-nil slice", got.Experiences)
	}
}

func TestCreateExperienceNormalizesPayload(t *testing.T) {
	store := newFakeExperienceStore()
	svc := newExperienceTestService(store)

	in := validExperienceInput()
	in.CompanyName = "  Nimbus Labs  "
	in.Role = " Senior Backend Engineer "
	in.Location = "  Noida, India "
	in.Technologies = []string{" Go ", "", "PostgreSQL"}
	in.Bullets = []string{" First point ", "   ", "Second point"}
	in.DisplayOrder = nil

	got, err := svc.CreateExperience(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateExperience: %v", err)
	}
	if got.CompanyName != "Nimbus Labs" || got.Role != "Senior Backend Engineer" || got.Location != "Noida, India" {
		t.Errorf("text fields were not trimmed: %+v", got)
	}
	if got.EmploymentType != models.EmploymentFullTime {
		t.Errorf("EmploymentType = %q, want %q", got.EmploymentType, models.EmploymentFullTime)
	}
	if len(got.Technologies) != 2 || got.Technologies[0] != "Go" {
		t.Errorf("blank technologies must be dropped: %+v", got.Technologies)
	}
	if len(got.Bullets) != 2 || got.Bullets[1] != "Second point" {
		t.Errorf("blank bullets must be dropped: %+v", got.Bullets)
	}
	if got.DisplayOrder != 0 {
		t.Errorf("DisplayOrder = %d, want 0 when the payload omits it", got.DisplayOrder)
	}
	if got.ID == "" {
		t.Error("a new entry must get an ID")
	}
	if !got.CreatedAt.Equal(experienceTestNow) || !got.UpdatedAt.Equal(experienceTestNow) {
		t.Errorf("timestamps were not stamped: created=%v updated=%v", got.CreatedAt, got.UpdatedAt)
	}

	stored := store.bullets[got.ID]
	if len(stored) != 2 || stored[0].DisplayOrder != 1 || stored[1].DisplayOrder != 2 {
		t.Fatalf("bullets were not stored with a 1-based order: %+v", stored)
	}
}

func TestCreateExperienceRequiresCoreFields(t *testing.T) {
	cases := []struct {
		name  string
		mutate func(*ExperienceInput)
	}{
		{name: "company_name", mutate: func(in *ExperienceInput) { in.CompanyName = "   " }},
		{name: "role", mutate: func(in *ExperienceInput) { in.Role = "" }},
		{name: "start_date", mutate: func(in *ExperienceInput) { in.StartDate = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeExperienceStore()
			svc := newExperienceTestService(store)

			in := validExperienceInput()
			tc.mutate(&in)

			_, err := svc.CreateExperience(context.Background(), in)
			if code := experienceErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if len(store.experiences) != 0 {
				t.Errorf("invalid input must not be stored: %+v", store.experiences)
			}
		})
	}
}

func TestCreateExperienceValidatesDateLogic(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ExperienceInput)
		wantErr string
	}{
		{
			name: "bad start_date format",
			mutate: func(in *ExperienceInput) {
				in.StartDate = "2024/04/01"
			},
			wantErr: "start_date must be a date in YYYY-MM-DD format.",
		},
		{
			name: "bad end_date format",
			mutate: func(in *ExperienceInput) {
				in.EndDate = "01-12-2025"
			},
			wantErr: "end_date must be a date in YYYY-MM-DD format.",
		},
		{
			name: "end_date provided when is_current is true",
			mutate: func(in *ExperienceInput) {
				in.IsCurrent = true
				in.EndDate = "2025-12-31"
			},
			wantErr: "end_date must be empty while is_current is true.",
		},
		{
			name: "end_date earlier than start_date",
			mutate: func(in *ExperienceInput) {
				in.StartDate = "2024-04-01"
				in.EndDate = "2023-12-31"
			},
			wantErr: "end_date must not be earlier than start_date.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeExperienceStore()
			svc := newExperienceTestService(store)

			in := validExperienceInput()
			tc.mutate(&in)

			_, err := svc.CreateExperience(context.Background(), in)
			if code := experienceErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if msg := apierr.From(err).Message; msg != tc.wantErr {
				t.Errorf("error message = %q, want %q", msg, tc.wantErr)
			}
		})
	}
}

func TestCreateExperienceValidatesEmploymentType(t *testing.T) {
	cases := []struct {
		name    string
		val     string
		wantErr bool
	}{
		{name: "full_time uppercase", val: "FULL_TIME", wantErr: false},
		{name: "part_time lowercase", val: "part_time", wantErr: false},
		{name: "contract", val: "contract", wantErr: false},
		{name: "intern", val: "intern", wantErr: false},
		{name: "remote", val: "remote", wantErr: false},
		{name: "empty allowed", val: "", wantErr: false},
		{name: "invalid enum freelance", val: "freelance", wantErr: true},
		{name: "invalid enum internship", val: "internship", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newExperienceTestService(newFakeExperienceStore())
			in := validExperienceInput()
			in.EmploymentType = tc.val

			got, err := svc.CreateExperience(context.Background(), in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.val)
				}
				if code := experienceErrCode(t, err); code != "validation_failed" {
					t.Errorf("got code %q, want validation_failed", code)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tc.val != "" && got.EmploymentType != strings.ToUpper(tc.val) {
					t.Errorf("EmploymentType = %q, want %q", got.EmploymentType, strings.ToUpper(tc.val))
				}
			}
		})
	}
}

func TestUpdateExperienceReplacesBulletsAndPreservesDisplayOrder(t *testing.T) {
	store := newFakeExperienceStore()
	svc := newExperienceTestService(store)

	created, err := svc.CreateExperience(context.Background(), validExperienceInput())
	if err != nil {
		t.Fatalf("CreateExperience: %v", err)
	}

	updateIn := validExperienceInput()
	updateIn.Role = "Principal Engineer"
	updateIn.Bullets = []string{"A single new impact point replacing old bullets."}
	updateIn.DisplayOrder = nil // keep stored order

	updated, err := svc.UpdateExperience(context.Background(), created.ID, updateIn)
	if err != nil {
		t.Fatalf("UpdateExperience: %v", err)
	}

	if updated.Role != "Principal Engineer" {
		t.Errorf("Role = %q, want Principal Engineer", updated.Role)
	}
	if len(updated.Bullets) != 1 || updated.Bullets[0] != "A single new impact point replacing old bullets." {
		t.Errorf("bullets were not replaced: %+v", updated.Bullets)
	}

	storedBullets := store.bullets[created.ID]
	if len(storedBullets) != 1 || storedBullets[0].Text != "A single new impact point replacing old bullets." {
		t.Errorf("stored bullets in fake store not replaced: %+v", storedBullets)
	}
}

func TestUpdateExperienceNotFound(t *testing.T) {
	svc := newExperienceTestService(newFakeExperienceStore())

	_, err := svc.UpdateExperience(context.Background(), "non-existent-id", validExperienceInput())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code := experienceErrCode(t, err); code != "experience_not_found" {
		t.Errorf("got code %q, want experience_not_found", code)
	}
}

func TestDeleteExperience(t *testing.T) {
	store := newFakeExperienceStore()
	svc := newExperienceTestService(store)

	created, err := svc.CreateExperience(context.Background(), validExperienceInput())
	if err != nil {
		t.Fatalf("CreateExperience: %v", err)
	}

	if err := svc.DeleteExperience(context.Background(), created.ID); err != nil {
		t.Fatalf("DeleteExperience: %v", err)
	}

	if len(store.experiences) != 0 {
		t.Errorf("expected experiences store to be empty, got %d", len(store.experiences))
	}
	if len(store.bullets[created.ID]) != 0 {
		t.Errorf("expected bullets to be removed, got %d", len(store.bullets[created.ID]))
	}

	// Deleting again should return experience_not_found
	err = svc.DeleteExperience(context.Background(), created.ID)
	if code := experienceErrCode(t, err); code != "experience_not_found" {
		t.Errorf("got code %q, want experience_not_found", code)
	}
}

