package service

import (
	"context"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// fakeAdminStore is an in-memory admin_users table.
type fakeAdminStore struct {
	admins map[string]*models.AdminUser
}

func newFakeAdminStore() *fakeAdminStore {
	return &fakeAdminStore{admins: map[string]*models.AdminUser{}}
}

func (f *fakeAdminStore) FindByID(_ context.Context, id string) (*models.AdminUser, error) {
	if a, ok := f.admins[id]; ok {
		return a, nil
	}
	return nil, models.ErrNotFound
}

func (f *fakeAdminStore) FindByIdentifier(_ context.Context, identifier string) (*models.AdminUser, error) {
	for _, a := range f.admins {
		if lower(a.Username) == lower(identifier) || lower(a.Email) == lower(identifier) {
			return a, nil
		}
	}
	return nil, models.ErrNotFound
}

func (f *fakeAdminStore) TouchLastLogin(_ context.Context, id string, at time.Time) error {
	if a, ok := f.admins[id]; ok {
		a.LastLoginAt = &at
	}
	return nil
}

func (f *fakeAdminStore) UpdatePassword(_ context.Context, id, passwordHash string, at time.Time) error {
	a, ok := f.admins[id]
	if !ok {
		return models.ErrNotFound
	}
	a.PasswordHash = passwordHash
	a.UpdatedAt = at
	return nil
}

// fakeTwoFAStore is an in-memory admin_2fa table.
type fakeTwoFAStore struct {
	rows []*models.TwoFactorConfig
}

func (f *fakeTwoFAStore) FindByAdminAndMethod(_ context.Context, adminID, method string) (*models.TwoFactorConfig, error) {
	for _, r := range f.rows {
		if r.AdminID == adminID && r.Method == method {
			return r, nil
		}
	}
	return nil, models.ErrNotFound
}

func (f *fakeTwoFAStore) ListByAdmin(_ context.Context, adminID string) ([]models.TwoFactorConfig, error) {
	out := []models.TwoFactorConfig{}
	for _, r := range f.rows {
		if r.AdminID == adminID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeTwoFAStore) Upsert(_ context.Context, cfg *models.TwoFactorConfig) error {
	for _, r := range f.rows {
		if r.AdminID == cfg.AdminID && r.Method == cfg.Method {
			r.IsEnabled = cfg.IsEnabled
			r.ConfirmedAt = cfg.ConfirmedAt
			return nil
		}
	}
	f.rows = append(f.rows, cfg)
	return nil
}

func (f *fakeTwoFAStore) SetEnabled(_ context.Context, adminID, method string, enabled bool, at time.Time) error {
	for _, r := range f.rows {
		if r.AdminID == adminID && r.Method == method {
			r.IsEnabled = enabled
			if enabled {
				r.ConfirmedAt = &at
			}
			return nil
		}
	}
	return models.ErrNotFound
}
