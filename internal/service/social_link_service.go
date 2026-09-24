package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
)

// SocialLinkDeps wires the social link service to persistence.
type SocialLinkDeps struct {
	SocialLinks SocialLinkStore
}

// SocialLinkService owns the social link use-cases of the portfolio.
type SocialLinkService struct {
	socialLinks SocialLinkStore
}

// NewSocialLinkService builds the service.
func NewSocialLinkService(deps SocialLinkDeps) *SocialLinkService {
	return &SocialLinkService{socialLinks: deps.SocialLinks}
}

// maxSocialURLLen caps the url column. The database keeps it as unbounded TEXT.
const maxSocialURLLen = 500

// SocialLinkInput is one entry of the admin payload. The admin panel always
// sends the complete set, because the write replaces every stored row.
type SocialLinkInput struct {
	Platform     string `json:"platform"`
	URL          string `json:"url"`
	DisplayOrder *int   `json:"display_order"`
}

// SocialLinkReplaceInput is the body of PUT /api/admin/social-links.
type SocialLinkReplaceInput struct {
	Links []SocialLinkInput `json:"links"`
}

// SocialLinkView is one link as rendered on the public site. There is no
// created_at or updated_at because the table has neither column.
type SocialLinkView struct {
	ID           string `json:"id"`
	Platform     string `json:"platform"`
	URL          string `json:"url"`
	DisplayOrder int    `json:"display_order"`
}

// PublicSocialLinkListView is the payload of GET /api/public/social-links.
type PublicSocialLinkListView struct {
	SocialLinks []SocialLinkView `json:"social_links"`
}

// SocialLinkReplaceView is the payload returned by the admin write, so the
// panel can refresh from the server's view instead of trusting its own payload.
type SocialLinkReplaceView struct {
	SocialLinks []SocialLinkView `json:"social_links"`
}

// PublicSocialLinks returns the links in display order.
func (s *SocialLinkService) PublicSocialLinks(ctx context.Context) (*PublicSocialLinkListView, error) {
	links, err := s.socialLinks.ListSocialLinks(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]SocialLinkView, 0, len(links))
	for i := range links {
		views = append(views, socialLinkView(&links[i]))
	}
	return &PublicSocialLinkListView{SocialLinks: views}, nil
}

// ReplaceSocialLinks validates the whole payload and then swaps the stored set
// atomically. An empty payload is rejected: the write deletes every row first,
// so allowing an empty list would let a buggy admin panel wipe the section.
func (s *SocialLinkService) ReplaceSocialLinks(ctx context.Context, in SocialLinkReplaceInput) (*SocialLinkReplaceView, error) {
	built, err := buildSocialLinks(in.Links)
	if err != nil {
		return nil, err
	}
	if err := s.socialLinks.ReplaceSocialLinks(ctx, built); err != nil {
		if errors.Is(err, models.ErrConflict) {
			// The duplicate check in buildSocialLinks should catch this first;
			// this is the safety net for a race between the check and the write.
			return nil, errSocialLinkValidation("links must not repeat the same platform.")
		}
		return nil, err
	}

	views := make([]SocialLinkView, 0, len(built))
	for i := range built {
		views = append(views, socialLinkView(&built[i]))
	}
	return &SocialLinkReplaceView{SocialLinks: views}, nil
}

// maxSocialLinks caps the payload size. The enum only allows four platforms and
// duplicates are rejected, so this is a second line of defence rather than the
// primary check: without it a buggy or malicious admin call could push thousands
// of entries through the per-entry URL validation. It tracks the enum size, so
// adding a platform lifts the cap automatically.
var maxSocialLinks = len(models.SocialPlatforms)

// buildSocialLinks validates every entry and assigns IDs. seen guards the
// UNIQUE(platform) index so a duplicate is reported per platform instead of
// failing the whole write with a bare 409.
func buildSocialLinks(links []SocialLinkInput) ([]models.SocialLink, error) {
	if len(links) == 0 {
		return nil, errSocialLinkValidation("links must contain at least one entry.")
	}
	if len(links) > maxSocialLinks {
		return nil, errSocialLinkValidation(
			fmt.Sprintf("links must contain at most %d entries.", maxSocialLinks))
	}

	seen := make(map[string]bool, len(links))
	out := make([]models.SocialLink, 0, len(links))
	for i := range links {
		platform, err := normalizeSocialPlatform(links[i].Platform)
		if err != nil {
			return nil, err
		}
		if seen[platform] {
			return nil, errSocialLinkExists(platform)
		}
		seen[platform] = true

		trimmed := strings.TrimSpace(links[i].URL)
		if trimmed == "" {
			return nil, errSocialLinkRequired("url")
		}
		if err := validateSocialURL(trimmed); err != nil {
			return nil, err
		}

		displayOrder := i
		if links[i].DisplayOrder != nil {
			if *links[i].DisplayOrder < 0 {
				return nil, errSocialLinkValidation("display_order must not be negative.")
			}
			displayOrder = *links[i].DisplayOrder
		}

		out = append(out, models.SocialLink{
			ID:           ids.New(),
			Platform:     platform,
			URL:          trimmed,
			DisplayOrder: displayOrder,
		})
	}
	return out, nil
}

// normalizeSocialPlatform lower-cases the value and checks it against the enum.
// Unlike the other content modules these stay lowercase, matching the schema
// column comment.
func normalizeSocialPlatform(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", errSocialLinkRequired("platform")
	}
	for _, candidate := range models.SocialPlatforms {
		if normalized == candidate {
			return normalized, nil
		}
	}
	return "", errSocialLinkValidation(
		"platform must be one of " + strings.Join(models.SocialPlatforms, ", ") + ".")
}

// validateSocialURL checks the length, rejects whitespace and control characters,
// and requires an absolute http(s) URL. The column stores TEXT, so a malformed
// link would only surface when a visitor clicks it.
//
// Whitespace matters here: only the ends are trimmed before this point, so an
// inner space such as "https://github.com/kam lesh" would otherwise be stored and
// then percent-encoded by the browser into a dead link.
func validateSocialURL(value string) error {
	if utf8.RuneCountInString(value) > maxSocialURLLen {
		return errSocialLinkValidation(fmt.Sprintf("url must be at most %d characters.", maxSocialURLLen))
	}
	for _, r := range value {
		switch {
		case unicode.IsControl(r):
			return errSocialLinkValidation("url must not contain control characters.")
		case unicode.IsSpace(r):
			return errSocialLinkValidation("url must not contain whitespace.")
		}
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return errSocialLinkValidation("url must be a valid URL.")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errSocialLinkValidation("url must be an http or https URL.")
	}
	if parsed.Host == "" {
		return errSocialLinkValidation("url must be a valid URL.")
	}
	return nil
}

func socialLinkView(link *models.SocialLink) SocialLinkView {
	return SocialLinkView{
		ID:           link.ID,
		Platform:     link.Platform,
		URL:          link.URL,
		DisplayOrder: link.DisplayOrder,
	}
}