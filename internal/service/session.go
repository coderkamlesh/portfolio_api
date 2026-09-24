package service

import (
	"context"
	"errors"
	"log"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
	"github.com/coderkamlesh/portfolio_api/internal/security"
)

// issueSession mints an access token plus a rotating opaque refresh token.
// Only the SHA-256 hash of the refresh token is persisted.
func (s *AuthService) issueSession(ctx context.Context, admin *models.AdminUser, meta RequestMeta) (*TokenPair, error) {
	accessToken, accessExpiresAt, err := s.tokens.IssueAccessToken(admin)
	if err != nil {
		return nil, wrapErr(err, 500, "internal_error", "Could not create a session.")
	}

	refreshToken, err := security.GenerateRefreshToken()
	if err != nil {
		return nil, wrapErr(err, 500, "internal_error", "Could not create a session.")
	}

	now := s.now()
	refreshExpiresAt := now.Add(s.cfg.RefreshTokenTTL)
	record := &models.RefreshToken{
		ID:        ids.New(),
		AdminID:   admin.ID,
		TokenHash: security.HashToken(refreshToken),
		ExpiresAt: refreshExpiresAt,
		UserAgent: sanitizeUserAgent(meta.UserAgent),
		IPAddress: meta.IPAddress,
		CreatedAt: now,
	}
	if err := s.refresh.Create(ctx, record); err != nil {
		return nil, err
	}

	return &TokenPair{
		TokenType:        "Bearer",
		AccessToken:      accessToken,
		ExpiresIn:        int(s.tokens.AccessTTL().Seconds()),
		ExpiresAt:        accessExpiresAt,
		RefreshToken:     refreshToken,
		RefreshExpiresIn: int(s.cfg.RefreshTokenTTL.Seconds()),
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

// Refresh rotates a refresh token: the presented token is revoked and a fresh
// pair is issued. Replaying an already-revoked token revokes every session of
// that admin, which is the standard protection against a stolen token.
func (s *AuthService) Refresh(ctx context.Context, in RefreshInput, meta RequestMeta) (*LoginResult, error) {
	if in.RefreshToken == "" {
		return nil, errInvalidRefreshToken()
	}

	record, err := s.refresh.FindByHash(ctx, security.HashToken(in.RefreshToken))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errInvalidRefreshToken()
		}
		return nil, err
	}

	now := s.now()

	if record.RevokedAt != nil {
		s.revokeAllSessions(ctx, record.AdminID, "refresh_token_reuse")
		return nil, errRefreshTokenReused()
	}
	if !now.Before(record.ExpiresAt) {
		if _, err := s.refresh.Revoke(ctx, record.ID, now); err != nil {
			log.Printf("⚠️  auth: could not revoke expired token %s: %v", record.ID, err)
		}
		return nil, errInvalidRefreshToken()
	}

	admin, err := s.adminFor(ctx, record.AdminID)
	if err != nil {
		return nil, err
	}
	if !admin.IsActive {
		s.revokeAllSessions(ctx, admin.ID, "account_disabled")
		return nil, errAccountDisabled()
	}

	rotated, err := s.refresh.Revoke(ctx, record.ID, now)
	if err != nil {
		return nil, err
	}
	if !rotated {
		// Someone else rotated it a moment ago — treat as reuse.
		s.revokeAllSessions(ctx, admin.ID, "refresh_token_race")
		return nil, errRefreshTokenReused()
	}

	tokens, err := s.issueSession(ctx, admin, meta)
	if err != nil {
		return nil, err
	}
	s.auditEvent(ctx, admin.ID, models.AuditTokenRefreshed, "", nil)
	return &LoginResult{Tokens: tokens, Admin: adminView(admin)}, nil
}

// Logout revokes one session, or every session of the admin.
func (s *AuthService) Logout(ctx context.Context, adminID string, in LogoutInput) error {
	now := s.now()

	if in.AllDevices || in.RefreshToken == "" {
		if _, err := s.refresh.RevokeAllForAdmin(ctx, adminID, now); err != nil {
			return err
		}
		s.auditEvent(ctx, adminID, models.AuditLoggedOut, "all_devices", nil)
		return nil
	}

	record, err := s.refresh.FindByHash(ctx, security.HashToken(in.RefreshToken))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil // already gone — logging out is idempotent
		}
		return err
	}
	if record.AdminID != adminID {
		return errInvalidRefreshToken()
	}
	if _, err := s.refresh.Revoke(ctx, record.ID, now); err != nil {
		return err
	}
	s.auditEvent(ctx, adminID, models.AuditLoggedOut, "single_device", nil)
	return nil
}

// CurrentAdmin powers GET /api/auth/me.
func (s *AuthService) CurrentAdmin(ctx context.Context, adminID string) (*MeView, error) {
	admin, err := s.admins.FindByID(ctx, adminID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errInvalidRefreshToken()
		}
		return nil, err
	}
	if !admin.IsActive {
		return nil, errAccountDisabled()
	}

	status, err := s.TwoFAStatus(ctx, admin)
	if err != nil {
		return nil, err
	}
	sessions, err := s.refresh.CountActive(ctx, admin.ID, s.now())
	if err != nil {
		log.Printf("⚠️  auth: could not count sessions for %s: %v", admin.ID, err)
		sessions = 0
	}

	return &MeView{Admin: adminView(admin), TwoFactor: status, ActiveSession: sessions}, nil
}

// revokeAllSessions revokes every refresh token of the admin and records why.
func (s *AuthService) revokeAllSessions(ctx context.Context, adminID, reason string) {
	if _, err := s.refresh.RevokeAllForAdmin(ctx, adminID, s.now()); err != nil {
		log.Printf("⚠️  auth: revoke all sessions failed for %s: %v", adminID, err)
	}
	s.auditEvent(ctx, adminID, models.AuditTokenReuseDetected, reason, nil)
}
