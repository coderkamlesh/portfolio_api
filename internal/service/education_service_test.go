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

// educationTestNow is the fixed clock every education test runs on. The year
// checks are relative to it, so 2026 keeps the bounds deterministic.
var educationTestNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// fakeEducationStore is an in-memory EducationStore mirroring the repository:
// entries are listed in display order and updates replace the stored row.
type fakeEducationStore struct {
	entries  []models.Education
	createErr error
	updateErr error
	deleteErr error
}

func newFakeEducationStore() *fakeEducationStore {
	return &fakeEducationStore{}
}

func (s *fakeEducationStore) ListEducations(context.Context) ([]models.Education, error) {
	out := append([]models.Education(nil), s.entries...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DisplayOrder != out[j].DisplayOrder {
			return out[i].DisplayOrder < out[j].DisplayOrder
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *fakeEducationStore) FindEducationByID(_ context.Context, id string) (*models.Education, error) {
	for i := range s.entries {
		if s.entries[i].ID == id {
			entry := s.entries[i]
			return &entry, nil
		}
	}
	return nil, models.ErrNotFound
}

func (s *fakeEducationStore) CreateEducation(_ context.Context, education *models.Education) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.entries = append(s.entries, *education)
	return nil
}

func (s *fakeEducationStore) UpdateEducation(_ context.Context, education *models.Education) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	for i := range s.entries {
		if s.entries[i].ID == education.ID {
			s.entries[i] = *education
			return nil
		}
	}
	return models.ErrNotFound
}

func (s *fakeEducationStore) DeleteEducation(_ context.Context, id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for i := range s.entries {
		if s.entries[i].ID == id {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			return nil
		}
	}
	return models.ErrNotFound
}

func newEducationTestService(store *fakeEducationStore) *EducationService {
	return NewEducationService(EducationDeps{
		Educations: store,
		Now:        func() time.Time { return educationTestNow },
	})
}

func educationErrCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return apierr.From(err).Code
}

func validEducationInput() EducationInput {
	endYear := 2026
	return EducationInput{
		Institution:  "NIT Rourkela",
		Degree:       "B.Tech",
		FieldOfStudy: "Computer Science and Engineering",
		StartYear:    2020,
		EndYear:      &endYear,
		Grade:        "First Class",
		GPA:          "8.5/10",
		Coursework:   []string{"Data Structures", "Operating Systems", "DBMS"},
		Honors:       "Dean's List",
	}
}

func TestPublicEducationsListsInDisplayOrder(t *testing.T) {
	store := newFakeEducationStore()
	store.entries = []models.Education{
		{ID: "e-second", Institution: "Second", Degree: "B.Tech", StartYear: 2021, DisplayOrder: 1},
		{ID: "e-first", Institution: "First", Degree: "B.Tech", StartYear: 2020, DisplayOrder: 0},
	}
	svc := newEducationTestService(store)

	got, err := svc.PublicEducations(context.Background())
	if err != nil {
		t.Fatalf("PublicEducations: %v", err)
	}
	if len(got.Education) != 2 {
		t.Fatalf("got %d entries, want 2", len(got.Education))
	}
	if got.Education[0].ID != "e-first" || got.Education[1].ID != "e-second" {
		t.Errorf("entries are not in display order: %q then %q", got.Education[0].ID, got.Education[1].ID)
	}
	if got.Education[0].Coursework == nil {
		t.Error("coursework must be an empty array, not null")
	}
	if got.Education[0].EndYear != nil {
		t.Errorf("end_year must be omitted for an in-progress degree, got %v", *got.Education[0].EndYear)
	}
}

func TestPublicEducationsRendersEmptyArrayWhenNothingConfigured(t *testing.T) {
	svc := newEducationTestService(newFakeEducationStore())

	got, err := svc.PublicEducations(context.Background())
	if err != nil {
		t.Fatalf("PublicEducations: %v", err)
	}
	if got.Education == nil || len(got.Education) != 0 {
		t.Fatalf("Education = %#v, want an empty non-nil slice", got.Education)
	}
}

func TestCreateEducationNormalizesPayload(t *testing.T) {
	store := newFakeEducationStore()
	svc := newEducationTestService(store)

	in := validEducationInput()
	in.Institution = "  NIT Rourkela  "
	in.Coursework = []string{" Data Structures ", "   ", "DBMS"}
	in.DisplayOrder = nil

	got, err := svc.CreateEducation(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateEducation: %v", err)
	}
	if got.Institution != "NIT Rourkela" {
		t.Errorf("Institution = %q, want the trimmed value", got.Institution)
	}
	if len(got.Coursework) != 2 || got.Coursework[0] != "Data Structures" {
		t.Errorf("blank coursework must be dropped: %+v", got.Coursework)
	}
	if got.DisplayOrder != 0 {
		t.Errorf("DisplayOrder = %d, want 0 when the payload omits it", got.DisplayOrder)
	}
	if got.ID == "" {
		t.Error("a new entry must get an ID")
	}
	if !got.CreatedAt.Equal(educationTestNow) {
		t.Errorf("created_at = %v, want the fixed clock", got.CreatedAt)
	}
	if got.EndYear == nil || *got.EndYear != 2026 {
		t.Errorf("EndYear = %v, want 2026", got.EndYear)
	}
}

func TestCreateEducationRequiresCoreFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*EducationInput)
	}{
		{name: "institution", mutate: func(in *EducationInput) { in.Institution = "   " }},
		{name: "degree", mutate: func(in *EducationInput) { in.Degree = "" }},
		{name: "start_year", mutate: func(in *EducationInput) { in.StartYear = 0 }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeEducationStore()
			svc := newEducationTestService(store)

			in := validEducationInput()
			tc.mutate(&in)

			_, err := svc.CreateEducation(context.Background(), in)
			if code := educationErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if len(store.entries) != 0 {
				t.Errorf("invalid input must not be stored: %+v", store.entries)
			}
		})
	}
}

func TestCreateEducationValidatesYears(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*EducationInput)
		wantErr string
	}{
		{
			name:    "start_year too old",
			mutate:  func(in *EducationInput) { in.StartYear = 1949 },
			wantErr: "start_year must be 1950 or later.",
		},
		{
			name:    "start_year in the far future",
			mutate:  func(in *EducationInput) { in.StartYear = 2050 },
			wantErr: "start_year must not be later than 2036.",
		},
		{
			name:    "end_year earlier than start_year",
			mutate:  func(in *EducationInput) { in.EndYear = intPtr(2019) },
			wantErr: "end_year must not be earlier than start_year.",
		},
		{
			name:    "end_year in the far future",
			mutate:  func(in *EducationInput) { in.EndYear = intPtr(2050) },
			wantErr: "end_year must not be later than 2036.",
		},
		{
			name:    "end_year omitted is allowed",
			mutate:  func(in *EducationInput) { in.EndYear = nil },
			wantErr: "",
		},
		{
			name:    "end_year equal to start_year is allowed",
			mutate:  func(in *EducationInput) { in.EndYear = intPtr(2020) },
			wantErr: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newEducationTestService(newFakeEducationStore())

			in := validEducationInput()
			tc.mutate(&in)

			got, err := svc.CreateEducation(context.Background(), in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if in.EndYear == nil && got.EndYear != nil {
					t.Errorf("end_year should stay omitted, got %d", *got.EndYear)
				}
				return
			}
			if code := educationErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if msg := apierr.From(err).Message; msg != tc.wantErr {
				t.Errorf("error message = %q, want %q", msg, tc.wantErr)
			}
		})
	}
}

func TestCreateEducationEnforcesCourseworkLimits(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*EducationInput)
		wantErr string
	}{
		{
			name: "too many courses",
			mutate: func(in *EducationInput) {
				courses := make([]string, maxCoursework+1)
				for i := range courses {
					courses[i] = "Course"
				}
				in.Coursework = courses
			},
			wantErr: "coursework must contain at most 6 entries.",
		},
		{
			name:    "course name too long",
			mutate:  func(in *EducationInput) { in.Coursework = []string{longEducationText(maxCourseLen + 1)} },
			wantErr: "each coursework must be at most 100 characters.",
		},
		{
			name:    "control characters in institution",
			mutate:  func(in *EducationInput) { in.Institution = "NIT\nRourkela" },
			wantErr: "institution must not contain control characters.",
		},
		{
			name:    "institution too long",
			mutate:  func(in *EducationInput) { in.Institution = longEducationText(maxInstitutionLen + 1) },
			wantErr: "institution must be at most 200 characters.",
		},
		{
			name: "negative display_order",
			mutate: func(in *EducationInput) {
				in.DisplayOrder = intPtr(-1)
			},
			wantErr: "display_order must not be negative.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeEducationStore()
			svc := newEducationTestService(store)

			in := validEducationInput()
			tc.mutate(&in)

			_, err := svc.CreateEducation(context.Background(), in)
			if code := educationErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if msg := apierr.From(err).Message; msg != tc.wantErr {
				t.Errorf("error message = %q, want %q", msg, tc.wantErr)
			}
			if len(store.entries) != 0 {
				t.Error("invalid input must not be stored")
			}
		})
	}
}

func longEducationText(n int) string {
	return strings.Repeat("a", n)
}

func TestUpdateEducationKeepsIdentityAndDisplayOrder(t *testing.T) {
	store := newFakeEducationStore()
	svc := newEducationTestService(store)

	order := 3
	createIn := validEducationInput()
	createIn.DisplayOrder = &order
	created, err := svc.CreateEducation(context.Background(), createIn)
	if err != nil {
		t.Fatalf("CreateEducation: %v", err)
	}

	updateIn := validEducationInput()
	updateIn.Degree = "B.Tech (Hons)"
	updateIn.DisplayOrder = nil

	updated, err := svc.UpdateEducation(context.Background(), created.ID, updateIn)
	if err != nil {
		t.Fatalf("UpdateEducation: %v", err)
	}

	if updated.ID != created.ID {
		t.Errorf("ID changed on update: %q -> %q", created.ID, updated.ID)
	}
	if updated.Degree != "B.Tech (Hons)" {
		t.Errorf("Degree = %q, want B.Tech (Hons)", updated.Degree)
	}
	if updated.DisplayOrder != 3 {
		t.Errorf("DisplayOrder = %d, want the stored 3", updated.DisplayOrder)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("created_at changed: %v -> %v", created.CreatedAt, updated.CreatedAt)
	}
	if len(store.entries) != 1 {
		t.Errorf("update must not insert a second row, got %d", len(store.entries))
	}
}

func TestUpdateEducationAcceptsExplicitDisplayOrder(t *testing.T) {
	store := newFakeEducationStore()
	svc := newEducationTestService(store)

	created, err := svc.CreateEducation(context.Background(), validEducationInput())
	if err != nil {
		t.Fatalf("CreateEducation: %v", err)
	}

	updateIn := validEducationInput()
	updateIn.DisplayOrder = intPtr(7)

	updated, err := svc.UpdateEducation(context.Background(), created.ID, updateIn)
	if err != nil {
		t.Fatalf("UpdateEducation: %v", err)
	}
	if updated.DisplayOrder != 7 {
		t.Errorf("DisplayOrder = %d, want 7", updated.DisplayOrder)
	}
}

func TestUpdateEducationNotFound(t *testing.T) {
	svc := newEducationTestService(newFakeEducationStore())

	_, err := svc.UpdateEducation(context.Background(), "missing", validEducationInput())
	if code := educationErrCode(t, err); code != "education_not_found" {
		t.Fatalf("got code %q, want education_not_found", code)
	}
}

func TestDeleteEducation(t *testing.T) {
	store := newFakeEducationStore()
	svc := newEducationTestService(store)

	created, err := svc.CreateEducation(context.Background(), validEducationInput())
	if err != nil {
		t.Fatalf("CreateEducation: %v", err)
	}

	if err := svc.DeleteEducation(context.Background(), created.ID); err != nil {
		t.Fatalf("DeleteEducation: %v", err)
	}
	if len(store.entries) != 0 {
		t.Errorf("expected an empty store, got %d entries", len(store.entries))
	}

	err = svc.DeleteEducation(context.Background(), created.ID)
	if code := educationErrCode(t, err); code != "education_not_found" {
		t.Errorf("got code %q, want education_not_found on the second delete", code)
	}
}

func TestAdminEducationsExposeCreatedAt(t *testing.T) {
	store := newFakeEducationStore()
	svc := newEducationTestService(store)

	created, err := svc.CreateEducation(context.Background(), validEducationInput())
	if err != nil {
		t.Fatalf("CreateEducation: %v", err)
	}

	got, err := svc.AdminEducations(context.Background())
	if err != nil {
		t.Fatalf("AdminEducations: %v", err)
	}
	if len(got.Education) != 1 {
		t.Fatalf("got %d entries, want 1", len(got.Education))
	}
	if got.Education[0].ID != created.ID {
		t.Errorf("ID = %q, want %q", got.Education[0].ID, created.ID)
	}
	if !got.Education[0].CreatedAt.Equal(educationTestNow) {
		t.Errorf("created_at = %v, want the fixed clock", got.Education[0].CreatedAt)
	}
}