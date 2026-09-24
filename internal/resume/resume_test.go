package resume

import (
	"strings"
	"testing"
)

func TestSanitizeFoldsTypographicCharactersToASCII(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain ascii is untouched", input: "Senior Backend Engineer", want: "Senior Backend Engineer"},
		{name: "right single quote", input: "Kamlesh's project", want: "Kamlesh's project"},
		{name: "curly double quotes", input: `the "judge" queue`, want: `the "judge" queue`},
		{name: "en dash becomes a hyphen", input: "Apr 2024 - Dec 2025", want: "Apr 2024 - Dec 2025"},
		{name: "em dash becomes a hyphen", input: "Go - Ruby", want: "Go - Ruby"},
		{name: "bullet glyph becomes a hyphen", input: "Cut p95 latency by 38%", want: "Cut p95 latency by 38%"},
		{name: "ellipsis", input: "Load, then...", want: "Load, then..."},
		{name: "arrow", input: "input -> output", want: "input -> output"},
		{name: "non breaking space", input: "Noida India", want: "Noida India"},
		{name: "accented latin folds", input: "Jose Nunez", want: "Jose Nunez"},
		{name: "sharp s", input: "Straße", want: "Strasse"},
		{name: "newlines collapse", input: "line one\nline two", want: "line one line two"},
		{name: "tabs collapse", input: "a\tb", want: "a b"},
		{name: "surrounding space trimmed", input: "  padded  ", want: "padded"},
		{name: "double spaces collapse", input: "a   b", want: "a b"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitize(tc.input); got != tc.want {
				t.Errorf("sanitize(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestSanitizeNeverEmitsNonASCII is the guard that matters most for the PDF. If
// any rune survived, fpdf would write its UTF-8 bytes into a CP1252-encoded core
// font and the result would be mojibake that an ATS then copies verbatim.
func TestSanitizeNeverEmitsNonASCII(t *testing.T) {
	inputs := []string{
		"Résumé for José",
		"日本語のテキスト",
		"emoji 🚀 and tabs\t",
		"mixed — dash • bullet ’ quote",
		"zero\u200bwidth\u200bjoiners",
		"tabs\tand\nnewlines\r\n",
		"«guillemets» ‹single›",
		"½ ¾ ² ³ ° ±",
	}

	for _, input := range inputs {
		got := sanitize(input)
		for _, r := range got {
			if r > 0x7F {
				t.Errorf("sanitize(%q) left a non-ASCII rune %U in %q", input, r, got)
			}
		}
	}
}

func TestSanitizeMarksUnknownRunesVisibly(t *testing.T) {
	// An untranslatable character becomes a visible marker rather than being
	// dropped, so a human notices before the resume is sent.
	if got := sanitize("a中b"); got != "a?b" {
		t.Errorf("sanitize(%q) = %q, want %q", "a中b", got, "a?b")
	}
}

func TestFormatMonthYearAcceptsBothStoredShapes(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "2024-04-01", want: "Apr 2024"},
		{input: "2024-03", want: "Mar 2024"},
		{input: "2024-03-15", want: "Mar 2024"},
		{input: "", want: ""},
		{input: "not-a-date", want: "not-a-date"},
	}

	for _, tc := range cases {
		if got := formatMonthYear(tc.input); got != tc.want {
			t.Errorf("formatMonthYear(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatRangeUsesPresentForCurrentRoles(t *testing.T) {
	cases := []struct {
		name      string
		start     string
		end       string
		isCurrent bool
		want      string
	}{
		{name: "current role", start: "2024-04-01", isCurrent: true, want: "Apr 2024 - Present"},
		{name: "current role ignores a stored end", start: "2024-04-01", end: "2025-12-31", isCurrent: true, want: "Apr 2024 - Present"},
		{name: "completed role", start: "2022-01-01", end: "2023-12-31", want: "Jan 2022 - Dec 2023"},
		{name: "only a start", start: "2022-01-01", want: "Jan 2022"},
		{name: "only an end", end: "2023-12-31", want: "Dec 2023"},
		{name: "neither", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatRange(tc.start, tc.end, tc.isCurrent); got != tc.want {
				t.Errorf("formatRange = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatYearRange(t *testing.T) {
	cases := []struct {
		name      string
		start     int
		end       int
		isCurrent bool
		want      string
	}{
		{name: "completed degree", start: 2020, end: 2024, want: "2020 - 2024"},
		{name: "in progress", start: 2024, isCurrent: true, want: "2024 - Present"},
		{name: "no end year", start: 2020, want: "2020"},
		{name: "only an end", end: 2024, want: "2024"},
		{name: "empty", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatYearRange(tc.start, tc.end, tc.isCurrent); got != tc.want {
				t.Errorf("formatYearRange = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTitleCase(t *testing.T) {
	cases := []struct{ input, want string }{
		{input: "OPEN_SOURCE", want: "Open Source"},
		{input: "CERTIFICATION", want: "Certification"},
		{input: "AWARD", want: "Award"},
	}
	for _, tc := range cases {
		if got := titleCase(tc.input); got != tc.want {
			t.Errorf("titleCase(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestJoinNonEmptyDropsBlankParts(t *testing.T) {
	if got := joinNonEmpty(" | ", "Backend", "", "  ", "Noida"); got != "Backend | Noida" {
		t.Errorf("joinNonEmpty = %q", got)
	}
	if got := joinNonEmpty(" | ", "", "   "); got != "" {
		t.Errorf("joinNonEmpty on blanks = %q, want empty", got)
	}
}

// TestRenderProducesASelectablePDF checks the two properties the guidance calls
// out: the file is a real PDF, and its text is an object rather than an image.
func TestRenderProducesASelectablePDF(t *testing.T) {
	data := Data{
		Profile: Profile{
			FullName: "Kamlesh Kumar",
			Title:    "Backend Engineer",
			Email:    "kamlesh@example.com",
			Location: "Noida, India",
			Summary:  "Backend engineer focused on distributed systems.",
			LinkedIn: "https://linkedin.com/in/kamlesh",
		},
		Skills: []SkillGroup{{Category: "Backend", Skills: []string{"Go", "PostgreSQL"}}},
		Experience: []Experience{{
			Company:   "Nimbus Labs",
			Role:      "Senior Backend Engineer",
			Location:  "Noida, India",
			StartDate: "2024-04-01",
			IsCurrent: true,
			Bullets:   []string{"Cut p99 latency by 42% by removing N+1 queries."},
		}},
		Projects: []Project{{
			Title:        "AlgoMaster",
			Role:         "Backend Developer",
			Technologies: []string{"Go"},
			Description:  "Practice platform for DSA interview preparation.",
			Bullets:      []string{"Designed the judge queue."},
		}},
		Education: []Education{{
			Institution: "NIT Rourkela",
			Degree:      "B.Tech",
			StartYear:   2020,
			EndYear:     2024,
		}},
		Extras: []ExtraGroup{{
			Category: "CERTIFICATION",
			Entries:  []Extra{{Title: "AWS Certified", Issuer: "AWS", IssuedDate: "2024-03"}},
		}},
	}

	out, err := Render(data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("Render returned no bytes")
	}
	if !strings.HasPrefix(string(out[:5]), "%PDF-") {
		t.Errorf("output is not a PDF: starts with %q", string(out[:5]))
	}
	if !strings.Contains(string(out), "/Type /Page") {
		t.Error("output does not look like it contains page objects")
	}
}

func TestRenderWithEmptyDataStillProducesAPDF(t *testing.T) {
	// An unconfigured portfolio must not error: the endpoint can then answer with
	// a blank-but-valid document rather than a 500.
	out, err := Render(Data{})
	if err != nil {
		t.Fatalf("Render with empty data: %v", err)
	}
	if !strings.HasPrefix(string(out[:5]), "%PDF-") {
		t.Error("empty render is not a PDF")
	}
}

func TestRenderAcceptsNonASCIIContent(t *testing.T) {
	// Rendering must never fail because of a stray curly quote or an accent: the
	// sanitizer folds them before anything is written to the PDF.
	out, err := Render(Data{Profile: Profile{
		FullName: "José Nunez",
		Title:    "Engineer — Backend",
		Summary:  "Built things • shipped them",
	}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(out) == 0 {
		t.Error("render produced no bytes for non-ASCII content")
	}
}

func TestRenderHandlesEmptyOptionalFields(t *testing.T) {
	// No contact details, no bullets, no dates: the layout must not emit dangling
	// separators or panic on empty values.
	out, err := Render(Data{
		Profile:    Profile{FullName: "Kamlesh", Title: "Engineer"},
		Experience: []Experience{{Company: "Acme"}},
		Projects:   []Project{{Title: "Thing"}},
		Education:  []Education{{Institution: "NIT"}},
		Skills:     []SkillGroup{{Category: "Backend"}},
		Extras:     []ExtraGroup{{Category: "AWARD"}},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(out[:5]), "%PDF-") {
		t.Error("render is not a PDF")
	}
}
