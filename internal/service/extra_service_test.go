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

// extraTestNow is the fixed clock every extra test runs on.
var extraTestNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// fakeExtraStore is an in-memory ExtraStore mirroring the repository: entries are
// listed by category then display order.
type fakeExtraStore struct {
	entries  []models.Extra
	createErr error
}

func newFakeExtraStore() *fakeExtraStore {
	return &fakeExtraStore{}
}

func (s *fakeExtraStore) ListExtras(context.Context) ([]models.Extra, error) {
	out := append([]models.Extra(nil), s.entries...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if out[i].DisplayOrder != out[j].DisplayOrder {
			return out[i].DisplayOrder < out[j].DisplayOrder
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *fakeExtraStore) FindExtraByID(_ context.Context, id string) (*models.Extra, error) {
	for i := range s.entries {
		if s.entries[i].ID == id {
			extra := s.entries[i]
			return &extra, nil
		}
	}
	return nil, models.ErrNotFound
}

func (s *fakeExtraStore) CreateExtra(_ context.Context, extra *models.Extra) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.entries = append(s.entries, *extra)
	return nil
}

func (s *fakeExtraStore) UpdateExtra(_ context.Context, extra *models.Extra) error {
	for i := range s.entries {
		if s.entries[i].ID == extra.ID {
			s.entries[i] = *extra
			return nil
		}
	}
	return models.ErrNotFound
}

func (s *fakeExtraStore) DeleteExtra(_ context.Context, id string) error {
	for i := range s.entries {
		if s.entries[i].ID == id {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			return nil
		}
	}
	return models.ErrNotFound
}

func newExtraTestService(store *fakeExtraStore) *ExtraService {
	return NewExtraService(ExtraDeps{
		Extras: store,
		Now:    func() time.Time { return extraTestNow },
	})
}

func extraErrCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return apierr.From(err).Code
}

func validExtraInput() ExtraInput {
	return ExtraInput{
		Category:      "certification",
		Title:         "AWS Certified Solutions Architect",
		Issuer:        "Amazon Web Services",
		IssuedDate:    "2024-03",
		CredentialURL: "https://aws.amazon.com/certification",
		Description:   "Validated design of highly available systems on AWS.",
	}
}

func TestPublicExtrasGroupsByCategoryInStableOrder(t *testing.T) {
	store := newFakeExtraStore()
	store.entries = []models.Extra{
		{ID: "x-talk", Category: models.ExtraCategoryTalk, Title: "A talk", DisplayOrder: 0},
		{ID: "x-award", Category: models.ExtraCategoryAward, Title: "An award", DisplayOrder: 0},
		{ID: "x-cert-2", Category: models.ExtraCategoryCertification, Title: "Cert two", DisplayOrder: 1},
		{ID: "x-cert-1", Category: models.ExtraCategoryCertification, Title: "Cert one", DisplayOrder: 0},
	}
	svc := newExtraTestService(store)

	got, err := svc.PublicExtras(context.Background())
	if err != nil {
		t.Fatalf("PublicExtras: %v", err)
	}

	// Only non-empty categories are returned, in models.ExtraCategories order.
	wantCategories := []string{
		models.ExtraCategoryCertification,
		models.ExtraCategoryAward,
		models.ExtraCategoryTalk,
	}
	if len(got.Categories) != len(wantCategories) {
		t.Fatalf("got %d categories, want %d", len(got.Categories), len(wantCategories))
	}
	for i, want := range wantCategories {
		if got.Categories[i].Category != want {
			t.Errorf("category %d = %q, want %q", i, got.Categories[i].Category, want)
		}
	}

	// Within a category, entries keep their display order.
	certs := got.Categories[0].Extras
	if len(certs) != 2 || certs[0].ID != "x-cert-1" || certs[1].ID != "x-cert-2" {
		t.Errorf("certifications are not in display order: %+v", certs)
	}
}

func TestPublicExtrasOmitsEmptyCategories(t *testing.T) {
	store := newFakeExtraStore()
	store.entries = []models.Extra{
		{ID: "x-vol", Category: models.ExtraCategoryVolunteer, Title: "Volunteering"},
	}
	svc := newExtraTestService(store)

	got, err := svc.PublicExtras(context.Background())
	if err != nil {
		t.Fatalf("PublicExtras: %v", err)
	}
	if len(got.Categories) != 1 {
		t.Fatalf("got %d categories, want only the populated one", len(got.Categories))
	}
	if got.Categories[0].Category != models.ExtraCategoryVolunteer {
		t.Errorf("category = %q, want VOLUNTEER", got.Categories[0].Category)
	}
}

func TestPublicExtrasRendersEmptyArrayWhenNothingConfigured(t *testing.T) {
	svc := newExtraTestService(newFakeExtraStore())

	got, err := svc.PublicExtras(context.Background())
	if err != nil {
		t.Fatalf("PublicExtras: %v", err)
	}
	if got.Categories == nil || len(got.Categories) != 0 {
		t.Fatalf("Categories = %#v, want an empty non-nil slice", got.Categories)
	}
}

func TestCreateExtraNormalizesPayload(t *testing.T) {
	store := newFakeExtraStore()
	svc := newExtraTestService(store)

	in := validExtraInput()
	in.Title = "  AWS Certified Solutions Architect  "
	in.Issuer = "  Amazon Web Services  "
	in.DisplayOrder = nil

	got, err := svc.CreateExtra(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateExtra: %v", err)
	}
	if got.Title != "AWS Certified Solutions Architect" {
		t.Errorf("Title = %q, want the trimmed value", got.Title)
	}
	if got.Issuer != "Amazon Web Services" {
		t.Errorf("Issuer = %q, want the trimmed value", got.Issuer)
	}
	if got.Category != models.ExtraCategoryCertification {
		t.Errorf("Category = %q, want the upper-cased CERTIFICATION", got.Category)
	}
	if got.DisplayOrder != 0 {
		t.Errorf("DisplayOrder = %d, want 0 when the payload omits it", got.DisplayOrder)
	}
	if got.ID == "" {
		t.Error("a new entry must get an ID")
	}
	if !got.CreatedAt.Equal(extraTestNow) {
		t.Errorf("created_at = %v, want the fixed clock", got.CreatedAt)
	}
}

func TestCreateExtraRequiresCoreFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ExtraInput)
	}{
		{name: "category", mutate: func(in *ExtraInput) { in.Category = "   " }},
		{name: "title", mutate: func(in *ExtraInput) { in.Title = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeExtraStore()
			svc := newExtraTestService(store)

			in := validExtraInput()
			tc.mutate(&in)

			_, err := svc.CreateExtra(context.Background(), in)
			if code := extraErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if len(store.entries) != 0 {
				t.Errorf("invalid input must not be stored: %+v", store.entries)
			}
		})
	}
}

func TestCreateExtraValidatesCategory(t *testing.T) {
	cases := []struct {
		name    string
		val     string
		wantErr bool
	}{
		{name: "certification lowercase", val: "certification", wantErr: false},
		{name: "award uppercase", val: "AWARD", wantErr: false},
		{name: "publication mixed", val: "PuBlIcAtIoN", wantErr: false},
		{name: "open_source", val: "open_source", wantErr: false},
		{name: "talk", val: "talk", wantErr: false},
		{name: "volunteer", val: "volunteer", wantErr: false},
		{name: "unknown category", val: "hobby", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newExtraTestService(newFakeExtraStore())

			in := validExtraInput()
			in.Category = tc.val

			got, err := svc.CreateExtra(context.Background(), in)
			if tc.wantErr {
				if code := extraErrCode(t, err); code != "validation_failed" {
					t.Fatalf("got %v (code %q), want validation_failed", err, code)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Category != strings.ToUpper(tc.val) {
				t.Errorf("Category = %q, want %q", got.Category, strings.ToUpper(tc.val))
			}
		})
	}
}

func TestCreateExtraValidatesIssuedDate(t *testing.T) {
	cases := []struct {
		name    string
		val     string
		wantErr string
	}{
		{name: "month level", val: "2024-03", wantErr: ""},
		{name: "full date", val: "2024-03-15", wantErr: ""},
		{name: "empty is allowed", val: "", wantErr: ""},
		{name: "surrounding spaces are trimmed", val: "  2024-03  ", wantErr: ""},
		{
			name:    "slash format",
			val:     "2024/03",
			wantErr: "issued_date must be a date in YYYY-MM or YYYY-MM-DD format.",
		},
		{
			name:    "impossible month",
			val:     "2024-13",
			wantErr: "issued_date must be a date in YYYY-MM or YYYY-MM-DD format.",
		},
		{
			name:    "year only",
			val:     "2024",
			wantErr: "issued_date must be a date in YYYY-MM or YYYY-MM-DD format.",
		},
		{
			name:    "impossible day",
			val:     "2024-02-31",
			wantErr: "issued_date must be a date in YYYY-MM or YYYY-MM-DD format.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newExtraTestService(newFakeExtraStore())

			in := validExtraInput()
			in.IssuedDate = tc.val

			got, err := svc.CreateExtra(context.Background(), in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.IssuedDate != strings.TrimSpace(tc.val) {
					t.Errorf("IssuedDate = %q, want %q", got.IssuedDate, strings.TrimSpace(tc.val))
				}
				return
			}
			if code := extraErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if msg := apierr.From(err).Message; msg != tc.wantErr {
				t.Errorf("error message = %q, want %q", msg, tc.wantErr)
			}
		})
	}
}

func TestCreateExtraValidatesCredentialURL(t *testing.T) {
	cases := []struct {
		name    string
		val     string
		wantErr string
	}{
		{name: "https", val: "https://aws.amazon.com/cert", wantErr: ""},
		{name: "http", val: "http://example.com/cert", wantErr: ""},
		{name: "empty is allowed", val: "", wantErr: ""},
		{
			name:    "no scheme",
			val:     "aws.amazon.com/cert",
			wantErr: "credential_url must be an http or https URL.",
		},
		{
			name:    "ftp scheme",
			val:     "ftp://example.com/cert",
			wantErr: "credential_url must be an http or https URL.",
		},
		{
			name:    "no host",
			val:     "https://",
			wantErr: "credential_url must be a valid URL.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newExtraTestService(newFakeExtraStore())

			in := validExtraInput()
			in.CredentialURL = tc.val

			_, err := svc.CreateExtra(context.Background(), in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if code := extraErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if msg := apierr.From(err).Message; msg != tc.wantErr {
				t.Errorf("error message = %q, want %q", msg, tc.wantErr)
			}
		})
	}
}

func TestCreateExtraEnforcesLimits(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ExtraInput)
		wantErr string
	}{
		{
			name:    "title too long",
			mutate:  func(in *ExtraInput) { in.Title = strings.Repeat("a", maxExtraTitleLen+1) },
			wantErr: "title must be at most 200 characters.",
		},
		{
			name:    "control characters in title",
			mutate:  func(in *ExtraInput) { in.Title = "AWS\nCertified" },
			wantErr: "title must not contain control characters.",
		},
		{
			name:    "description too long",
			mutate:  func(in *ExtraInput) { in.Description = strings.Repeat("a", maxExtraDescriptionLen+1) },
			wantErr: "description must be at most 2000 characters.",
		},
		{
			name:    "negative display_order",
			mutate:  func(in *ExtraInput) { in.DisplayOrder = intPtr(-1) },
			wantErr: "display_order must not be negative.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeExtraStore()
			svc := newExtraTestService(store)

			in := validExtraInput()
			tc.mutate(&in)

			_, err := svc.CreateExtra(context.Background(), in)
			if code := extraErrCode(t, err); code != "validation_failed" {
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

func TestUpdateExtraKeepsIdentityAndDisplayOrder(t *testing.T) {
	store := newFakeExtraStore()
	svc := newExtraTestService(store)

	order := 4
	createIn := validExtraInput()
	createIn.DisplayOrder = &order
	created, err := svc.CreateExtra(context.Background(), createIn)
	if err != nil {
		t.Fatalf("CreateExtra: %v", err)
	}

	updateIn := validExtraInput()
	updateIn.Title = "AWS Solutions Architect Professional"
	updateIn.DisplayOrder = nil

	updated, err := svc.UpdateExtra(context.Background(), created.ID, updateIn)
	if err != nil {
		t.Fatalf("UpdateExtra: %v", err)
	}
	if updated.ID != created.ID {
		t.Errorf("ID changed on update: %q -> %q", created.ID, updated.ID)
	}
	if updated.Title != "AWS Solutions Architect Professional" {
		t.Errorf("Title = %q, want the new title", updated.Title)
	}
	if updated.DisplayOrder != 4 {
		t.Errorf("DisplayOrder = %d, want the stored 4", updated.DisplayOrder)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("created_at changed: %v -> %v", created.CreatedAt, updated.CreatedAt)
	}
	if len(store.entries) != 1 {
		t.Errorf("update must not insert a second row, got %d", len(store.entries))
	}
}

func TestUpdateExtraAcceptsExplicitDisplayOrder(t *testing.T) {
	store := newFakeExtraStore()
	svc := newExtraTestService(store)

	created, err := svc.CreateExtra(context.Background(), validExtraInput())
	if err != nil {
		t.Fatalf("CreateExtra: %v", err)
	}

	updateIn := validExtraInput()
	updateIn.DisplayOrder = intPtr(9)

	updated, err := svc.UpdateExtra(context.Background(), created.ID, updateIn)
	if err != nil {
		t.Fatalf("UpdateExtra: %v", err)
	}
	if updated.DisplayOrder != 9 {
		t.Errorf("DisplayOrder = %d, want 9", updated.DisplayOrder)
	}
}

func TestUpdateExtraNotFound(t *testing.T) {
	svc := newExtraTestService(newFakeExtraStore())

	_, err := svc.UpdateExtra(context.Background(), "missing", validExtraInput())
	if code := extraErrCode(t, err); code != "extra_not_found" {
		t.Fatalf("got code %q, want extra_not_found", code)
	}
}

func TestDeleteExtra(t *testing.T) {
	store := newFakeExtraStore()
	svc := newExtraTestService(store)

	created, err := svc.CreateExtra(context.Background(), validExtraInput())
	if err != nil {
		t.Fatalf("CreateExtra: %v", err)
	}

	if err := svc.DeleteExtra(context.Background(), created.ID); err != nil {
		t.Fatalf("DeleteExtra: %v", err)
	}
	if len(store.entries) != 0 {
		t.Errorf("expected an empty store, got %d entries", len(store.entries))
	}

	err = svc.DeleteExtra(context.Background(), created.ID)
	if code := extraErrCode(t, err); code != "extra_not_found" {
		t.Errorf("got code %q, want extra_not_found on the second delete", code)
	}
}

func TestAdminExtrasReturnAFlatListWithCreatedAt(t *testing.T) {
	store := newFakeExtraStore()
	svc := newExtraTestService(store)

	created, err := svc.CreateExtra(context.Background(), validExtraInput())
	if err != nil {
		t.Fatalf("CreateExtra: %v", err)
	}

	got, err := svc.AdminExtras(context.Background())
	if err != nil {
		t.Fatalf("AdminExtras: %v", err)
	}
	if len(got.Extras) != 1 {
		t.Fatalf("got %d entries, want 1", len(got.Extras))
	}
	if got.Extras[0].ID != created.ID {
		t.Errorf("ID = %q, want %q", got.Extras[0].ID, created.ID)
	}
	if !got.Extras[0].CreatedAt.Equal(extraTestNow) {
		t.Errorf("created_at = %v, want the fixed clock", got.Extras[0].CreatedAt)
	}
}