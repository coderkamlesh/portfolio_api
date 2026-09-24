package resume

import (
	"strings"
	"time"
)

// replacements maps the non-ASCII runes that reach this package in practice.
// The typography entries are the common ones: text copied out of Google Docs,
// Word or a CMS is full of curly quotes and en-dashes, so without this mapping a
// perfectly ordinary resume would render as mojibake.
var replacements = map[rune]string{
	'\u2018': "'", '\u2019': "'", '\u201A': "'", '\u201B': "'",
	'\u201C': `"`, '\u201D': `"`, '\u201E': `"`, '\u201F': `"`,
	'\u2013': "-", '\u2014': "-", '\u2015': "-", '\u2212': "-",
	'\u2022': "-", '\u2023': "-", '\u2043': "-",
	'\u2026': "...",
	'\u00A0': " ", '\u2007': " ", '\u202F': " ", '\u2009': " ",
	'\u00B7': "-",
	'\u2192': "->", '\u21D2': "=>", '\u00D7': "x",
	'\u200B': "", '\u200C': "", '\u200D': "", '\uFEFF': "",
	// Accented Latin-1 letters are common in real names. CP1252 can represent
	// them, but fpdf writes UTF-8 bytes, so they must be folded to ASCII here.
	'\u00E0': "a", '\u00E1': "a", '\u00E2': "a", '\u00E3': "a", '\u00E4': "a", '\u00E5': "a",
	'\u00E7': "c", '\u00E8': "e", '\u00E9': "e", '\u00EA': "e", '\u00EB': "e",
	'\u00EC': "i", '\u00ED': "i", '\u00EE': "i", '\u00EF': "i",
	'\u00F1': "n", '\u00F2': "o", '\u00F3': "o", '\u00F4': "o", '\u00F5': "o", '\u00F6': "o",
	'\u00F9': "u", '\u00FA': "u", '\u00FB': "u", '\u00FC': "u",
	'\u00C0': "A", '\u00C1': "A", '\u00C2': "A", '\u00C3': "A", '\u00C4': "A", '\u00C5': "A",
	'\u00C7': "C", '\u00C8': "E", '\u00C9': "E", '\u00CA': "E", '\u00CB': "E",
	'\u00CC': "I", '\u00CD': "I", '\u00CE': "I", '\u00CF': "I",
	'\u00D1': "N", '\u00D2': "O", '\u00D3': "O", '\u00D4': "O", '\u00D5': "O", '\u00D6': "O",
	'\u00D9': "U", '\u00DA': "U", '\u00DB': "U", '\u00DC': "U",
	'\u00DF': "ss", '\u00D8': "O", '\u00A9': "(c)", '\u00AE': "(r)",
}

// sanitize folds text to plain ASCII.
//
// Anything not in the table and outside printable ASCII becomes a visible '?'
// rather than being dropped: silently deleting a character would glue two words
// together, while '?' is obvious enough that a human fixes it before sending the
// resume. Control characters and newlines are dropped outright because they
// would break the PDF text operators.
func sanitize(value string) string {
	if value == "" {
		return ""
	}

	var out strings.Builder
	out.Grow(len(value))
	for _, r := range value {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			// Collapse any whitespace run to a single space.
			if out.Len() > 0 && !strings.HasSuffix(out.String(), " ") {
				out.WriteByte(' ')
			}
		case r < 0x80:
			out.WriteRune(r)
		default:
			// Every non-ASCII rune is looked up, not just a narrow slice of the
			// code space: the interesting characters (NBSP at U+00A0 through the
			// Latin-1 supplement) all live at or above 0xA0.
			if replacement, ok := replacements[r]; ok {
				out.WriteString(replacement)
			} else {
				out.WriteByte('?')
			}
		}
	}
	return strings.TrimSpace(collapseSpaces(out.String()))
}

// collapseSpaces squeezes runs of spaces left behind by folded characters.
func collapseSpaces(value string) string {
	for strings.Contains(value, "  ") {
		value = strings.ReplaceAll(value, "  ", " ")
	}
	return value
}

// monthNames are ASCII, so a formatted date is always safe to write.
var monthNames = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// formatMonthYear renders a stored date as "Apr 2024". It accepts both stored
// shapes used across the schema: full YYYY-MM-DD (experience, projects) and
// month-level YYYY-MM (extras). An unparseable value is returned trimmed so an
// odd stored value still shows up rather than vanishing.
func formatMonthYear(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	layouts := []string{"2006-01-02", "2006-01"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return monthNames[int(parsed.Month())-1] + " " + parsed.Format("2006")
		}
	}
	return value
}

// formatRange renders a start/end pair as "Apr 2024 - Present". The hyphen is
// ASCII: an en-dash would be written as mojibake by the core font path.
func formatRange(start, end string, isCurrent bool) string {
	startText := formatMonthYear(start)
	endText := formatMonthYear(end)

	if isCurrent {
		endText = "Present"
	}
	switch {
	case startText == "" && endText == "":
		return ""
	case startText == "":
		return endText
	case endText == "":
		return startText
	default:
		return startText + " - " + endText
	}
}

// formatYearRange renders education years. An open-ended degree shows
// "2020 - Present" rather than a missing value, which reads as an oversight.
func formatYearRange(startYear, endYear int, isCurrent bool) string {
	if startYear == 0 && endYear == 0 {
		return ""
	}
	start := ""
	if startYear > 0 {
		start = itoa(startYear)
	}
	end := ""
	if isCurrent {
		end = "Present"
	} else if endYear > 0 {
		end = itoa(endYear)
	}
	switch {
	case start == "" && end == "":
		return ""
	case start == "":
		return end
	case end == "":
		return start
	default:
		return start + " - " + end
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := make([]byte, 0, 12)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	if negative {
		digits = append(digits, '-')
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
