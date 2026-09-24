package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// SocialLinkRepository reads and writes social_links. The admin write replaces
// the whole set, so it runs delete + inserts inside one transaction: a visitor
// can never see a half-written set of links.
type SocialLinkRepository struct {
	base
}

// NewSocialLinkRepository binds the repository to the database connection.
func NewSocialLinkRepository(db *database.DB) *SocialLinkRepository {
	return &SocialLinkRepository{base{db: db}}
}

// socialLinkOrder lists links by display_order with id as a deterministic
// tie-breaker.
const socialLinkOrder = ` ORDER BY display_order, id`

// ListSocialLinks returns every social link in display order.
func (r *SocialLinkRepository) ListSocialLinks(ctx context.Context) ([]models.SocialLink, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, platform, url, display_order FROM social_links`+socialLinkOrder)
	if err != nil {
		return nil, fmt.Errorf("social link: list: %w", err)
	}
	defer rows.Close()

	var links []models.SocialLink
	for rows.Next() {
		link, err := scanSocialLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, *link)
	}
	return links, rows.Err()
}

// ReplaceSocialLinks deletes every existing row and inserts the given set in one
// transaction. A UNIQUE violation on platform is mapped to models.ErrConflict so
// the service can answer 409.
func (r *SocialLinkRepository) ReplaceSocialLinks(ctx context.Context, links []models.SocialLink) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("social link: replace: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM social_links`); err != nil {
		return fmt.Errorf("social link: replace: clear: %w", err)
	}

	const insert = `INSERT INTO social_links (id, platform, url, display_order)
	                 VALUES (?, ?, ?, ?)`
	for i := range links {
		if _, err := tx.ExecContext(ctx, insert,
			links[i].ID, links[i].Platform, links[i].URL, links[i].DisplayOrder); err != nil {
			return conflictOr("social link: replace: insert", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("social link: replace: commit: %w", err)
	}
	return nil
}

func scanSocialLink(row rowScanner) (*models.SocialLink, error) {
	var link models.SocialLink
	if err := row.Scan(&link.ID, &link.Platform, &link.URL, &link.DisplayOrder); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("social link: scan: %w", err)
	}
	return &link, nil
}