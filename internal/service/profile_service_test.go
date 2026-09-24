package service

import (
	"context"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

type fakeProfileStore struct {
	profile *models.Profile
}

func (s *fakeProfileStore) Find(context.Context) (*models.Profile, error) {
	if s.profile == nil {
		return nil, models.ErrNotFound
	}
	profile := *s.profile
	return &profile, nil
}

func (s *fakeProfileStore) Upsert(_ context.Context, profile *models.Profile) error {
	stored := *profile
	s.profile = &stored
	return nil
}

func TestProfileNotFound(t *testing.T) {
	svc := NewProfileService(ProfileDeps{Profiles: &fakeProfileStore{}})

	if _, err := svc.PublicProfile(context.Background()); apierr.From(err).Code != CodeProfileNotFound {
		t.Fatalf("PublicProfile returned %v, want %q", err, CodeProfileNotFound)
	}
	if _, err := svc.AdminProfile(context.Background()); apierr.From(err).Code != CodeProfileNotFound {
		t.Fatalf("AdminProfile returned %v, want %q", err, CodeProfileNotFound)
	}
}

func TestUpdateProfileCreatesSingletonAndNormalizesInput(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	store := &fakeProfileStore{}
	svc := NewProfileService(ProfileDeps{Profiles: store, Now: func() time.Time { return now }})

	got, err := svc.UpdateProfile(context.Background(), ProfileInput{
		FullName:        " Kamlesh Kumar ",
		Title:           " Backend Engineer ",
		Tagline:         " Builds reliable APIs ",
		Email:           " KAMLESH@EXAMPLE.COM ",
		Location:        " Bengaluru ",
		ExperienceLevel: " MID ",
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if got.ID == "" {
		t.Error("a new singleton profile must have an ID")
	}
	if got.FullName != "Kamlesh Kumar" || got.Title != "Backend Engineer" || got.Email != "kamlesh@example.com" {
		t.Errorf("profile was not normalized: %+v", got)
	}
	if !got.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, now)
	}
	if store.profile == nil || store.profile.SingletonKey != singletonProfileKey {
		t.Fatalf("singleton profile was not stored: %+v", store.profile)
	}
}

func TestUpdateProfilePreservesExistingID(t *testing.T) {
	now := time.Date(2026, 9, 24, 13, 0, 0, 0, time.UTC)
	store := &fakeProfileStore{profile: &models.Profile{
		ID:           "existing-profile-id",
		SingletonKey: singletonProfileKey,
		FullName:     "Old Name",
		Title:        "Old Title",
		Email:        "old@example.com",
	}}
	svc := NewProfileService(ProfileDeps{Profiles: store, Now: func() time.Time { return now }})

	got, err := svc.UpdateProfile(context.Background(), ProfileInput{
		FullName: "Kamlesh Kumar",
		Title:    "Senior Backend Engineer",
		Email:    "kamlesh@example.com",
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if got.ID != "existing-profile-id" || store.profile.ID != "existing-profile-id" {
		t.Fatalf("existing profile ID was not preserved: response=%q stored=%q", got.ID, store.profile.ID)
	}
	if got.FullName != "Kamlesh Kumar" || got.Title != "Senior Backend Engineer" {
		t.Errorf("existing profile was not updated: %+v", got)
	}
}

func TestUpdateProfileRequiresCoreFields(t *testing.T) {
	store := &fakeProfileStore{}
	svc := NewProfileService(ProfileDeps{Profiles: store})

	_, err := svc.UpdateProfile(context.Background(), ProfileInput{
		Title: "Backend Engineer",
		Email: "kamlesh@example.com",
	})
	if err == nil || apierr.From(err).Code != "validation_failed" {
		t.Fatalf("UpdateProfile returned %v, want validation_failed", err)
	}
	if store.profile != nil {
		t.Errorf("invalid input must not be stored: %+v", store.profile)
	}
}

func TestUpdateProfileRejectsInvalidEmail(t *testing.T) {
	store := &fakeProfileStore{}
	svc := NewProfileService(ProfileDeps{Profiles: store})

	_, err := svc.UpdateProfile(context.Background(), ProfileInput{
		FullName: "Kamlesh Kumar",
		Title:    "Backend Engineer",
		Email:    "not-an-email",
	})
	if err == nil || apierr.From(err).Code != "validation_failed" {
		t.Fatalf("UpdateProfile returned %v, want validation_failed", err)
	}
	if store.profile != nil {
		t.Errorf("invalid email must not be stored: %+v", store.profile)
	}
}
