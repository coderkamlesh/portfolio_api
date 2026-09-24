package service

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// fakeSocialLinkStore is an in-memory SocialLinkStore. ReplaceSocialLinks
// mirrors the repository: the stored set is swapped wholesale, and
// conflictOnWrite stands in for the UNIQUE index firing late.
type fakeSocialLinkStore struct {
	links           []models.SocialLink
	conflictOnWrite bool
}

func newFakeSocialLinkStore() *fakeSocialLinkStore {
	return &fakeSocialLinkStore{}
}

func (s *fakeSocialLinkStore) ListSocialLinks(context.Context) ([]models.SocialLink, error) {
	out := append([]models.SocialLink(nil), s.links...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DisplayOrder != out[j].DisplayOrder {
			return out[i].DisplayOrder < out[j].DisplayOrder
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *fakeSocialLinkStore) ReplaceSocialLinks(_ context.Context, links []models.SocialLink) error {
	if s.conflictOnWrite {
		return models.ErrConflict
	}
	s.links = append([]models.SocialLink(nil), links...)
	return nil
}

func newSocialLinkTestService(store *fakeSocialLinkStore) *SocialLinkService {
	return NewSocialLinkService(SocialLinkDeps{SocialLinks: store})
}

func socialLinkErrCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return apierr.From(err).Code
}

func validSocialLinkInput() SocialLinkReplaceInput {
	return SocialLinkReplaceInput{Links: []SocialLinkInput{
		{Platform: "linkedin", URL: "https://linkedin.com/in/kamlesh"},
		{Platform: "github", URL: "https://github.com/kamlesh"},
	}}
}

func TestPublicSocialLinksReturnsAnEmptyArrayWhenNoneExist(t *testing.T) {
	svc := newSocialLinkTestService(newFakeSocialLinkStore())

	got, err := svc.PublicSocialLinks(context.Background())
	if err != nil {
		t.Fatalf("PublicSocialLinks: %v", err)
	}
	if got.SocialLinks == nil || len(got.SocialLinks) != 0 {
		t.Fatalf("SocialLinks = %#v, want an empty non-nil slice", got.SocialLinks)
	}
}

func TestReplaceSocialLinksStoresNormalizedEntries(t *testing.T) {
	store := newFakeSocialLinkStore()
	svc := newSocialLinkTestService(store)

	got, err := svc.ReplaceSocialLinks(context.Background(), validSocialLinkInput())
	if err != nil {
		t.Fatalf("ReplaceSocialLinks: %v", err)
	}
	if len(got.SocialLinks) != 2 {
		t.Fatalf("got %d links, want 2", len(got.SocialLinks))
	}
	for i, link := range got.SocialLinks {
		if link.ID == "" {
			t.Errorf("link %d has no id", i)
		}
	}
	if got.SocialLinks[0].DisplayOrder != 0 || got.SocialLinks[1].DisplayOrder != 1 {
		t.Errorf("display_order should default to the payload position: %d, %d",
			got.SocialLinks[0].DisplayOrder, got.SocialLinks[1].DisplayOrder)
	}
	if len(store.links) != 2 {
		t.Errorf("store was not updated: %+v", store.links)
	}
}

func TestReplaceSocialLinksLowercasesPlatformsAndTrimsURLs(t *testing.T) {
	svc := newSocialLinkTestService(newFakeSocialLinkStore())

	in := SocialLinkReplaceInput{Links: []SocialLinkInput{
		{Platform: "  LinkedIn  ", URL: "  https://linkedin.com/in/kamlesh  "},
		{Platform: "GITHUB", URL: "https://github.com/kamlesh"},
	}}

	got, err := svc.ReplaceSocialLinks(context.Background(), in)
	if err != nil {
		t.Fatalf("ReplaceSocialLinks: %v", err)
	}
	if got.SocialLinks[0].Platform != models.SocialPlatformLinkedIn {
		t.Errorf("Platform = %q, want %q", got.SocialLinks[0].Platform, models.SocialPlatformLinkedIn)
	}
	if got.SocialLinks[0].URL != "https://linkedin.com/in/kamlesh" {
		t.Errorf("URL = %q, want the trimmed value", got.SocialLinks[0].URL)
	}
	if got.SocialLinks[1].Platform != models.SocialPlatformGitHub {
		t.Errorf("Platform = %q, want %q", got.SocialLinks[1].Platform, models.SocialPlatformGitHub)
	}
}

func TestReplaceSocialLinksRejectsAnEmptyPayload(t *testing.T) {
	store := newFakeSocialLinkStore()
	store.links = []models.SocialLink{{ID: "existing", Platform: models.SocialPlatformLinkedIn, URL: "https://linkedin.com/in/kamlesh"}}
	svc := newSocialLinkTestService(store)

	// An empty set is refused on purpose: the write deletes every row first, so
	// accepting it would let a buggy admin panel wipe the section.
	_, err := svc.ReplaceSocialLinks(context.Background(), SocialLinkReplaceInput{Links: nil})
	if code := socialLinkErrCode(t, err); code != "validation_failed" {
		t.Fatalf("got %v (code %q), want validation_failed", err, code)
	}
	if msg := apierr.From(err).Message; msg != "links must contain at least one entry." {
		t.Errorf("error message = %q", msg)
	}
	if len(store.links) != 1 {
		t.Error("a rejected payload must not touch the stored links")
	}
}

func TestReplaceSocialLinksRejectsDuplicatePlatforms(t *testing.T) {
	store := newFakeSocialLinkStore()
	store.links = []models.SocialLink{{ID: "existing", Platform: models.SocialPlatformLinkedIn, URL: "https://linkedin.com/in/kamlesh"}}
	svc := newSocialLinkTestService(store)

	// The same platform twice, differing only by case, still collides.
	in := SocialLinkReplaceInput{Links: []SocialLinkInput{
		{Platform: "github", URL: "https://github.com/kamlesh"},
		{Platform: "GitHub", URL: "https://github.com/kamlesh/other"},
	}}

	_, err := svc.ReplaceSocialLinks(context.Background(), in)
	if code := socialLinkErrCode(t, err); code != "social_link_conflict" {
		t.Fatalf("got %v (code %q), want social_link_conflict", err, code)
	}
	if len(store.links) != 1 {
		t.Error("a rejected payload must not touch the stored links")
	}
}

func TestReplaceSocialLinksMapsAWriteConflictToAValidationError(t *testing.T) {
	store := newFakeSocialLinkStore()
	store.conflictOnWrite = true
	svc := newSocialLinkTestService(store)

	// The in-memory duplicate check passes, so only the UNIQUE index firing can
	// produce this — the safety net for a check/insert race.
	_, err := svc.ReplaceSocialLinks(context.Background(), validSocialLinkInput())
	if code := socialLinkErrCode(t, err); code != "validation_failed" {
		t.Fatalf("got %v (code %q), want validation_failed", err, code)
	}
	if msg := apierr.From(err).Message; msg != "links must not repeat the same platform." {
		t.Errorf("error message = %q", msg)
	}
}

func TestReplaceSocialLinksValidatesPlatform(t *testing.T) {
	cases := []struct {
		name     string
		platform string
		wantErr  string
	}{
		{name: "linkedin lowercase", platform: "linkedin", wantErr: ""},
		{name: "github mixed case", platform: "GitHub", wantErr: ""},
		{name: "twitter", platform: "twitter", wantErr: ""},
		{name: "medium", platform: "MEDIUM", wantErr: ""},
		{name: "empty platform", platform: "   ", wantErr: "platform is required."},
		{
			name:     "unknown platform",
			platform: "instagram",
			wantErr:  "platform must be one of linkedin, github, twitter, medium.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newSocialLinkTestService(newFakeSocialLinkStore())

			in := SocialLinkReplaceInput{Links: []SocialLinkInput{
				{Platform: tc.platform, URL: "https://example.com/kamlesh"},
			}}

			_, err := svc.ReplaceSocialLinks(context.Background(), in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if code := socialLinkErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if msg := apierr.From(err).Message; msg != tc.wantErr {
				t.Errorf("error message = %q, want %q", msg, tc.wantErr)
			}
		})
	}
}

func TestReplaceSocialLinksValidatesURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr string
	}{
		{name: "https", url: "https://github.com/kamlesh", wantErr: ""},
		{name: "http", url: "http://example.com", wantErr: ""},
		{name: "url with path and query", url: "https://medium.com/@kamlesh?source=post", wantErr: ""},
		{name: "empty url", url: "   ", wantErr: "url is required."},
		{
			name:    "no scheme",
			url:     "github.com/kamlesh",
			wantErr: "url must be an http or https URL.",
		},
		{
			name:    "ftp scheme",
			url:     "ftp://example.com",
			wantErr: "url must be an http or https URL.",
		},
		{
			name:    "no host",
			url:     "https://",
			wantErr: "url must be a valid URL.",
		},
		{
			name:    "control characters",
			url:     "https://github.com/kam\nlesh",
			wantErr: "url must not contain control characters.",
		},
		{
			name:    "inner space in the path",
			url:     "https://github.com/kam lesh",
			wantErr: "url must not contain whitespace.",
		},
		{
			name:    "leading space inside the host",
			url:     "https://exa mple.com/kamlesh",
			wantErr: "url must not contain whitespace.",
		},
		{
			name:    "tab inside the path",
			url:     "https://github.com/kam\tsh",
			wantErr: "url must not contain control characters.",
		},
		{
			name:    "non-breaking space",
			url:     "https://github.com/kam lesh",
			wantErr: "url must not contain whitespace.",
		},
		{
			name:    "url too long",
			url:     "https://example.com/" + strings.Repeat("a", maxSocialURLLen),
			wantErr: "url must be at most 500 characters.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeSocialLinkStore()
			store.links = []models.SocialLink{{ID: "existing", Platform: models.SocialPlatformLinkedIn, URL: "https://linkedin.com/in/kamlesh"}}
			svc := newSocialLinkTestService(store)

			in := SocialLinkReplaceInput{Links: []SocialLinkInput{
				{Platform: "github", URL: tc.url},
			}}

			_, err := svc.ReplaceSocialLinks(context.Background(), in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if code := socialLinkErrCode(t, err); code != "validation_failed" {
				t.Fatalf("got %v (code %q), want validation_failed", err, code)
			}
			if msg := apierr.From(err).Message; msg != tc.wantErr {
				t.Errorf("error message = %q, want %q", msg, tc.wantErr)
			}
			if len(store.links) != 1 {
				t.Error("a rejected payload must not touch the stored links")
			}
		})
	}
}

func TestReplaceSocialLinksAcceptsExplicitDisplayOrder(t *testing.T) {
	svc := newSocialLinkTestService(newFakeSocialLinkStore())

	order := 5
	in := SocialLinkReplaceInput{Links: []SocialLinkInput{
		{Platform: "github", URL: "https://github.com/kamlesh", DisplayOrder: &order},
	}}

	got, err := svc.ReplaceSocialLinks(context.Background(), in)
	if err != nil {
		t.Fatalf("ReplaceSocialLinks: %v", err)
	}
	if got.SocialLinks[0].DisplayOrder != 5 {
		t.Errorf("DisplayOrder = %d, want 5", got.SocialLinks[0].DisplayOrder)
	}
}

func TestReplaceSocialLinksRejectsNegativeDisplayOrder(t *testing.T) {
	store := newFakeSocialLinkStore()
	store.links = []models.SocialLink{{ID: "existing", Platform: models.SocialPlatformLinkedIn, URL: "https://linkedin.com/in/kamlesh"}}
	svc := newSocialLinkTestService(store)

	order := -1
	in := SocialLinkReplaceInput{Links: []SocialLinkInput{
		{Platform: "github", URL: "https://github.com/kamlesh", DisplayOrder: &order},
	}}

	_, err := svc.ReplaceSocialLinks(context.Background(), in)
	if code := socialLinkErrCode(t, err); code != "validation_failed" {
		t.Fatalf("got %v (code %q), want validation_failed", err, code)
	}
	if msg := apierr.From(err).Message; msg != "display_order must not be negative." {
		t.Errorf("error message = %q", msg)
	}
	if len(store.links) != 1 {
		t.Error("a rejected payload must not touch the stored links")
	}
}

func TestReplaceSocialLinksRemovesPlatformsMissingFromThePayload(t *testing.T) {
	store := newFakeSocialLinkStore()
	store.links = []models.SocialLink{
		{ID: "old-1", Platform: models.SocialPlatformLinkedIn, URL: "https://linkedin.com/in/old", DisplayOrder: 0},
		{ID: "old-2", Platform: models.SocialPlatformTwitter, URL: "https://twitter.com/old", DisplayOrder: 1},
	}
	svc := newSocialLinkTestService(store)

	// Only github is sent, so the stored linkedin and twitter rows go away.
	if _, err := svc.ReplaceSocialLinks(context.Background(), SocialLinkReplaceInput{Links: []SocialLinkInput{
		{Platform: "github", URL: "https://github.com/kamlesh"},
	}}); err != nil {
		t.Fatalf("ReplaceSocialLinks: %v", err)
	}

	if len(store.links) != 1 {
		t.Fatalf("got %d stored links, want 1", len(store.links))
	}
	if store.links[0].Platform != models.SocialPlatformGitHub {
		t.Errorf("stored platform = %q, want github", store.links[0].Platform)
	}
}

func TestReplaceSocialLinksDropsAPlatformWhenTheWholeSetIsReplaced(t *testing.T) {
	store := newFakeSocialLinkStore()
	svc := newSocialLinkTestService(store)

	if _, err := svc.ReplaceSocialLinks(context.Background(), validSocialLinkInput()); err != nil {
		t.Fatalf("first ReplaceSocialLinks: %v", err)
	}
	if len(store.links) != 2 {
		t.Fatalf("setup failed: got %d stored links", len(store.links))
	}

	// A second write with one link leaves exactly one row.
	if _, err := svc.ReplaceSocialLinks(context.Background(), SocialLinkReplaceInput{Links: []SocialLinkInput{
		{Platform: "medium", URL: "https://medium.com/@kamlesh"},
	}}); err != nil {
		t.Fatalf("second ReplaceSocialLinks: %v", err)
	}
	if len(store.links) != 1 || store.links[0].Platform != models.SocialPlatformMedium {
		t.Errorf("stored links = %+v, want only the medium link", store.links)
	}
}

func TestReplaceSocialLinksRejectsMoreEntriesThanTheEnumHas(t *testing.T) {
	store := newFakeSocialLinkStore()
	store.links = []models.SocialLink{{ID: "existing", Platform: models.SocialPlatformLinkedIn, URL: "https://linkedin.com/in/kamlesh"}}
	svc := newSocialLinkTestService(store)

	// The cap tracks the enum size, so a payload with more entries than the
	// enum can ever accept is refused before any per-entry URL validation runs.
	overCap := make([]SocialLinkInput, maxSocialLinks+1)
	for i := range overCap {
		overCap[i] = SocialLinkInput{
			Platform: "github",
			URL:      "https://github.com/kamlesh",
		}
	}

	_, err := svc.ReplaceSocialLinks(context.Background(), SocialLinkReplaceInput{Links: overCap})
	if code := socialLinkErrCode(t, err); code != "validation_failed" {
		t.Fatalf("got %v (code %q), want validation_failed", err, code)
	}
	if msg := apierr.From(err).Message; msg != "links must contain at most 4 entries." {
		t.Errorf("error message = %q", msg)
	}
	if len(store.links) != 1 {
		t.Error("a rejected payload must not touch the stored links")
	}
}

func TestReplaceSocialLinksAcceptsExactlyTheEnumSize(t *testing.T) {
	svc := newSocialLinkTestService(newFakeSocialLinkStore())

	// One entry per accepted platform, each with a distinct platform.
	in := SocialLinkReplaceInput{Links: []SocialLinkInput{
		{Platform: "linkedin", URL: "https://linkedin.com/in/kamlesh"},
		{Platform: "github", URL: "https://github.com/kamlesh"},
		{Platform: "twitter", URL: "https://twitter.com/kamlesh"},
		{Platform: "medium", URL: "https://medium.com/@kamlesh"},
	}}

	got, err := svc.ReplaceSocialLinks(context.Background(), in)
	if err != nil {
		t.Fatalf("ReplaceSocialLinks: %v", err)
	}
	if len(got.SocialLinks) != maxSocialLinks {
		t.Errorf("got %d links, want %d", len(got.SocialLinks), maxSocialLinks)
	}
}

func TestPublicSocialListsWhatTheAdminStored(t *testing.T) {
	store := newFakeSocialLinkStore()
	svc := newSocialLinkTestService(store)

	replaced, err := svc.ReplaceSocialLinks(context.Background(), validSocialLinkInput())
	if err != nil {
		t.Fatalf("ReplaceSocialLinks: %v", err)
	}

	got, err := svc.PublicSocialLinks(context.Background())
	if err != nil {
		t.Fatalf("PublicSocialLinks: %v", err)
	}
	if len(got.SocialLinks) != len(replaced.SocialLinks) {
		t.Fatalf("got %d public links, want %d", len(got.SocialLinks), len(replaced.SocialLinks))
	}
	for i := range got.SocialLinks {
		if got.SocialLinks[i].ID != replaced.SocialLinks[i].ID {
			t.Errorf("link %d id = %q, want %q", i, got.SocialLinks[i].ID, replaced.SocialLinks[i].ID)
		}
	}
}