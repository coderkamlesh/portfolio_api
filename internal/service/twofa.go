package service

import (
	"context"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

// TwoFAStatus reports the mandatory email OTP state. Email OTP is the only
// second factor this API has and it cannot be disabled, so nothing is read from
// the database and there is no enabled/disabled state to report.
func (s *AuthService) TwoFAStatus(ctx context.Context, admin *models.AdminUser) (*TwoFAStatus, error) {
	return &TwoFAStatus{
		Required: true,
		Method:   models.TwoFAMethodEmailOTP,
		Email:    security.MaskEmail(admin.Email),
	}, nil
}

// TwoFAStatusByAdminID is the handler-friendly variant of TwoFAStatus.
func (s *AuthService) TwoFAStatusByAdminID(ctx context.Context, adminID string) (*TwoFAStatus, error) {
	admin, err := s.adminFor(ctx, adminID)
	if err != nil {
		return nil, err
	}
	return s.TwoFAStatus(ctx, admin)
}
