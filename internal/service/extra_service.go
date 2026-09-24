package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coderkamlesh/portfolio_api/internal/models"
	"github.com/coderkamlesh/portfolio_api/internal/pkg/ids"
)

// ExtraDeps wires the extra service to persistence. Now is injectable for
// deterministic tests; it defaults to UTC time.
type ExtraDeps struct {
	Extras ExtraStore
	Now    func() time.Time
}

// ExtraService owns the extras use-cases of the portfolio.
type ExtraService struct {
	extras ExtraStore
	now    func() time.Time
}

// NewExtraService builds the service.
func NewExtraService(deps ExtraDeps) *ExtraService {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &ExtraService{extras: deps.Extras, now: now}
}

// Input limits. db_schema.sql keeps these columns as unbounded TEXT, so the caps
// live here: generous enough for real content, tight enough that nothing absurd
// reaches the database or the future PDF renderer.
const (
	maxExtraTitleLen       = 200
	maxExtraIssuerLen      = 200
	maxExtraURLLen         = 500
	maxExtraDescriptionLen = 2000
)

// extraIssuedDateMonthLayout and extraIssuedDateDayLayout are the two accepted
// shapes of extras.issued_date. The schema comment allows both, because a
// certification is usually month-level while an award carries a full date.
const (
	extraIssuedDateMonthLayout = "2006-01"
	extraIssuedDateDayLayout   = "2006-01-02"
)

// ExtraInput is the create/update payload. DisplayOrder keeps the stored order
// when nil, so an edit never silently reorders an entry.
type ExtraInput struct {
	Category      string
	Title         string
	Issuer        string
	IssuedDate    string
	CredentialURL string
	Description   string
	DisplayOrder  *int
}

// ExtraView is one entry as rendered on the public site.
type ExtraView struct {
	ID            string `json:"id"`
	Category      string `json:"category"`
	Title         string `json:"title"`
	Issuer        string `json:"issuer,omitempty"`
	IssuedDate    string `json:"issued_date,omitempty"`
	CredentialURL string `json:"credential_url,omitempty"`
	Description   string `json:"description,omitempty"`
	DisplayOrder  int    `json:"display_order"`
}

// ExtraCategoryView is one category with its entries nested, used by the public
// grouped response.
type ExtraCategoryView struct {
	Category string     `json:"category"`
	Extras   []ExtraView `json:"extras"`
}

// PublicExtraListView is the payload of GET /api/public/extras. Only categories
// that actually have entries are present, in the order of
// models.ExtraCategories.
type PublicExtraListView struct {
	Categories []ExtraCategoryView `json:"categories"`
}

// AdminExtraView adds persistence metadata for the admin panel. The extras table
// has no updated_at column, so only created_at is exposed.
type AdminExtraView struct {
	ExtraView
	CreatedAt time.Time `json:"created_at"`
}

// AdminExtraListView is the payload of GET /api/admin/extras. The admin panel
// edits rows, not groups, so it gets a flat list.
type AdminExtraListView struct {
	Extras []AdminExtraView `json:"extras"`
}

// PublicExtras groups every entry by category. Empty categories are omitted so
// the frontend never has to render an empty section.
func (s *ExtraService) PublicExtras(ctx context.Context) (*PublicExtraListView, error) {
	entries, err := s.extras.ListExtras(ctx)
	if err != nil {
		return nil, err
	}

	grouped := make(map[string][]ExtraView, len(entries))
	for i := range entries {
		grouped[entries[i].Category] = append(grouped[entries[i].Category], extraView(&entries[i]))
	}

	categories := make([]ExtraCategoryView, 0, len(grouped))
	for _, category := range models.ExtraCategories {
		items := grouped[category]
		if len(items) == 0 {
			continue
		}
		categories = append(categories, ExtraCategoryView{Category: category, Extras: items})
	}
	return &PublicExtraListView{Categories: categories}, nil
}

// AdminExtras returns a flat list of every entry with its created_at stamp.
func (s *ExtraService) AdminExtras(ctx context.Context) (*AdminExtraListView, error) {
	entries, err := s.extras.ListExtras(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]AdminExtraView, 0, len(entries))
	for i := range entries {
		views = append(views, *adminExtraView(&entries[i]))
	}
	return &AdminExtraListView{Extras: views}, nil
}

// CreateExtra stores a new entry.
func (s *ExtraService) CreateExtra(ctx context.Context, in ExtraInput) (*AdminExtraView, error) {
	extra, err := buildExtra(in, nil, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.extras.CreateExtra(ctx, extra); err != nil {
		return nil, err
	}
	return adminExtraView(extra), nil
}

// UpdateExtra replaces every editable field of an entry.
func (s *ExtraService) UpdateExtra(ctx context.Context, id string, in ExtraInput) (*AdminExtraView, error) {
	existing, err := s.findExtra(ctx, id)
	if err != nil {
		return nil, err
	}

	extra, err := buildExtra(in, existing, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.extras.UpdateExtra(ctx, extra); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errExtraNotFound()
		}
		return nil, err
	}
	return adminExtraView(extra), nil
}

// DeleteExtra removes one entry.
func (s *ExtraService) DeleteExtra(ctx context.Context, id string) error {
	if err := s.extras.DeleteExtra(ctx, id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return errExtraNotFound()
		}
		return err
	}
	return nil
}

func (s *ExtraService) findExtra(ctx context.Context, id string) (*models.Extra, error) {
	extra, err := s.extras.FindExtraByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errExtraNotFound()
		}
		return nil, err
	}
	return extra, nil
}

// buildExtra validates the payload and produces the row. existing is nil on
// create; its ID and created_at are carried over on update.
func buildExtra(in ExtraInput, existing *models.Extra, now time.Time) (*models.Extra, error) {
	// category is NOT NULL in the schema and the public endpoint groups by it, so
	// an empty value is rejected here rather than being stored as a row that
	// would silently disappear from the grouped response.
	if strings.TrimSpace(in.Category) == "" {
		return nil, errExtraRequired("category")
	}
	category, err := normalizeEnum("category", in.Category, models.ExtraCategories)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, errExtraRequired("title")
	}
	if err := validateExtraText("title", title, maxExtraTitleLen); err != nil {
		return nil, err
	}

	issuer := strings.TrimSpace(in.Issuer)
	if err := validateOptionalExtraText("issuer", issuer, maxExtraIssuerLen); err != nil {
		return nil, err
	}

	description := strings.TrimSpace(in.Description)
	if err := validateOptionalExtraText("description", description, maxExtraDescriptionLen); err != nil {
		return nil, err
	}

	issuedDate, err := normalizeIssuedDate(in.IssuedDate)
	if err != nil {
		return nil, err
	}

	credentialURL, err := normalizeExtraURL("credential_url", in.CredentialURL)
	if err != nil {
		return nil, err
	}

	displayOrder := 0
	if existing != nil {
		displayOrder = existing.DisplayOrder
	}
	if in.DisplayOrder != nil {
		if *in.DisplayOrder < 0 {
			return nil, errExtraValidation("display_order must not be negative.")
		}
		displayOrder = *in.DisplayOrder
	}

	extra := &models.Extra{
		ID:            ids.New(),
		Category:      category,
		Title:         title,
		Issuer:        issuer,
		IssuedDate:    issuedDate,
		CredentialURL: credentialURL,
		Description:   description,
		DisplayOrder:  displayOrder,
		CreatedAt:     now,
	}
	if existing != nil {
		extra.ID = existing.ID
		extra.CreatedAt = existing.CreatedAt
	}
	return extra, nil
}

// normalizeIssuedDate accepts the two shapes the schema allows: a month-level
// YYYY-MM and a full YYYY-MM-DD. Anything else is rejected, so a typo cannot
// reach the PDF renderer.
func normalizeIssuedDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) == len(extraIssuedDateMonthLayout) {
		if _, err := time.Parse(extraIssuedDateMonthLayout, trimmed); err != nil {
			return "", errExtraValidation("issued_date must be a date in YYYY-MM or YYYY-MM-DD format.")
		}
		return trimmed, nil
	}
	if _, err := time.Parse(extraIssuedDateDayLayout, trimmed); err != nil {
		return "", errExtraValidation("issued_date must be a date in YYYY-MM or YYYY-MM-DD format.")
	}
	return trimmed, nil
}

// normalizeExtraURL trims the value and checks that it is an absolute http(s)
// URL. The database stores TEXT, so a malformed credential link would only
// surface when a visitor clicks it.
func normalizeExtraURL(field, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if err := validateExtraText(field, trimmed, maxExtraURLLen); err != nil {
		return "", err
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", errExtraValidation(field + " must be a valid URL.")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errExtraValidation(field + " must be an http or https URL.")
	}
	if parsed.Host == "" {
		return "", errExtraValidation(field + " must be a valid URL.")
	}
	return trimmed, nil
}

// validateExtraText rejects over-long and control-character values. Tabs and
// newlines count as control characters, so a stray newline in a single-line
// field is rejected instead of silently breaking the PDF layout.
func validateExtraText(field, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return errExtraValidation(fmt.Sprintf("%s must be at most %d characters.", field, max))
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return errExtraValidation(fmt.Sprintf("%s must not contain control characters.", field))
		}
	}
	return nil
}

// validateOptionalExtraText applies validateExtraText but treats an empty value
// as "clear the column".
func validateOptionalExtraText(field, value string, max int) error {
	if value == "" {
		return nil
	}
	return validateExtraText(field, value, max)
}

func extraView(extra *models.Extra) ExtraView {
	return ExtraView{
		ID:            extra.ID,
		Category:      extra.Category,
		Title:         extra.Title,
		Issuer:        extra.Issuer,
		IssuedDate:    extra.IssuedDate,
		CredentialURL: extra.CredentialURL,
		Description:   extra.Description,
		DisplayOrder:  extra.DisplayOrder,
	}
}

func adminExtraView(extra *models.Extra) *AdminExtraView {
	return &AdminExtraView{
		ExtraView: extraView(extra),
		CreatedAt: extra.CreatedAt,
	}
}