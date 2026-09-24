package resume

import (
	"bytes"
	"strings"

	"github.com/go-pdf/fpdf"
)

// Page geometry in millimetres: US Letter with comfortable margins, which keeps
// a single wide text column - the layout a parser reads most reliably.
const (
	pageWidthMM   = 215.9
	pageHeightMM  = 279.4
	marginMM      = 15.0
	bulletIndent  = 4.0
	gapAfterEntry = 1.4
)

// fontSize is the body size. The guidance is to stay at or above 10pt, so 10.5
// leaves a little headroom without wasting a page.
const fontSize = 10.5

// typeStyle is a font family, weight and size. Storing the three parts instead
// of a packed string keeps the calls readable and avoids a string round-trip.
type typeStyle struct {
	style string // "", "B", "I"
	size  float64
}

// Render produces the PDF bytes for data.
//
// Arial maps to the Helvetica core font, so no font asset is shipped. The
// trade-off is that the core font is encoded in CP1252 while fpdf writes string
// bytes verbatim, which means any non-ASCII rune would be emitted as mojibake and
// then copied into an ATS as garbage. sanitize() prevents that.
func Render(data Data) ([]byte, error) {
	pdf := fpdf.NewCustom(&fpdf.InitType{
		UnitStr: "mm",
		Size:    fpdf.SizeType{Wd: pageWidthMM, Ht: pageHeightMM},
	})
	pdf.SetMargins(marginMM, marginMM, marginMM)
	pdf.SetAutoPageBreak(true, marginMM)

	// Page header and footer stay empty on purpose: some parsers skip them
	// entirely, so contact information is rendered in the body flow instead.
	pdf.SetHeaderFunc(func() {})
	pdf.SetFooterFunc(func() {})

	pdf.AddPage()
	width := pageWidthMM - 2*marginMM

	writeHeader(pdf, width, data.Profile)
	if summary := sanitize(data.Profile.Summary); summary != "" {
		writeSection(pdf, width, "Summary")
		writeParagraph(pdf, width, summary)
	}
	writeExperience(pdf, width, data.Experience)
	writeProjects(pdf, width, data.Projects)
	writeSkills(pdf, width, data.Skills)
	writeEducation(pdf, width, data.Education)
	writeExtras(pdf, width, data.Extras)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeHeader renders the name, title and contact line. The contact line is
// filtered so that empty values do not leave dangling separators behind.
func writeHeader(pdf *fpdf.Fpdf, width float64, profile Profile) {
	pdf.SetFont("Arial", "B", 18)
	pdf.MultiCell(width, 7, sanitize(profile.FullName), "", "C", false)
	pdf.Ln(1)

	pdf.SetFont("Arial", "", 11.5)
	pdf.MultiCell(width, 5, sanitize(profile.Title), "", "C", false)

	contact := joinNonEmpty(" | ", profile.Email, profile.Phone, profile.Location)
	if contact != "" {
		pdf.Ln(0.5)
		pdf.SetFont("Arial", "", 9.5)
		pdf.MultiCell(width, 4.5, contact, "", "C", false)
	}

	links := joinNonEmpty(" | ", profile.LinkedIn, profile.Github, profile.Portfolio, profile.Twitter)
	if links != "" {
		pdf.SetFont("Arial", "", 9.5)
		pdf.MultiCell(width, 4.5, links, "", "C", false)
	}
	pdf.Ln(3)
}

// writeSection renders an uppercase section heading followed by a thin rule, which
// separates sections without a graphic element.
func writeSection(pdf *fpdf.Fpdf, width float64, title string) {
	pdf.Ln(1)
	pdf.SetFont("Arial", "B", 11)
	pdf.MultiCell(width, 5, sanitize(title), "", "L", false)

	y := pdf.GetY()
	pdf.SetDrawColor(160, 160, 160)
	pdf.SetLineWidth(0.2)
	pdf.Line(marginMM, y, marginMM+width, y)
	pdf.Ln(1.5)
}

// writeEntryTitle prints a bold headline on the left and a meta line on the right
// of the same row, which is the conventional role/project/degree header. Two
// sequential CellFormat calls share the line because each advances the cursor.
func writeEntryTitle(pdf *fpdf.Fpdf, width float64, headline, meta string) {
	headline = sanitize(headline)
	meta = sanitize(meta)
	if meta == "" {
		pdf.SetFont("Arial", "B", fontSize)
		pdf.MultiCell(width, 5, headline, "", "L", false)
		return
	}

	pdf.SetFont("Arial", "B", fontSize)
	headlineWidth := pdf.GetStringWidth(headline) + 2
	// If the headline is too long to leave room, stack the meta underneath rather
	// than letting the two overlap.
	if headlineWidth > width/2 {
		pdf.MultiCell(width, 5, headline, "", "L", false)
		pdf.SetFont("Arial", "", 9.5)
		pdf.MultiCell(width, 4.5, meta, "", "L", false)
		return
	}

	pdf.SetFont("Arial", "B", fontSize)
	pdf.CellFormat(headlineWidth, 5, headline, "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9.5)
	pdf.CellFormat(width-headlineWidth, 5, meta, "", 0, "R", false, 0, "")
	pdf.Ln(5)
}

func writeMeta(pdf *fpdf.Fpdf, width float64, meta string) {
	if meta == "" {
		return
	}
	pdf.SetFont("Arial", "", 9.5)
	pdf.MultiCell(width, 4.5, sanitize(meta), "", "L", false)
}

func writeParagraph(pdf *fpdf.Fpdf, width float64, text string) {
	pdf.SetFont("Arial", "", fontSize)
	pdf.MultiCell(width, lineGap, sanitize(text), "", "L", false)
}

// writeBullets renders each entry as a hyphen-prefixed line. The hyphen is ASCII
// on purpose: the bullet glyph U+2022 is not representable in the core font
// encoding and would be emitted as mojibake, and a symbol font such as
// ZapfDingbats would look right while extracting as garbage in an ATS.
func writeBullets(pdf *fpdf.Fpdf, width float64, bullets []string) {
	for i := range bullets {
		text := sanitize(bullets[i])
		if text == "" {
			continue
		}
		pdf.SetFont("Arial", "", fontSize)
		pdf.MultiCell(width, lineGap, "- "+text, "", "L", false)
	}
}

const lineGap = 4.6

func writeExperience(pdf *fpdf.Fpdf, width float64, entries []Experience) {
	if len(entries) == 0 {
		return
	}
	writeSection(pdf, width, "Experience")
	for i := range entries {
		entry := entries[i]
		writeEntryTitle(pdf, width, entry.Company, formatRange(entry.StartDate, entry.EndDate, entry.IsCurrent))
		writeMeta(pdf, width, joinNonEmpty(" | ", entry.Role, entry.Location))
		writeBullets(pdf, width, entry.Bullets)
		pdf.Ln(gapAfterEntry)
	}
}

func writeProjects(pdf *fpdf.Fpdf, width float64, entries []Project) {
	if len(entries) == 0 {
		return
	}
	writeSection(pdf, width, "Projects")
	for i := range entries {
		entry := entries[i]
		writeEntryTitle(pdf, width, entry.Title, formatRange(entry.StartDate, entry.EndDate, entry.IsCurrent))
		writeMeta(pdf, width, joinNonEmpty(" | ", entry.Role, strings.Join(entry.Technologies, ", ")))

		if description := sanitize(entry.Description); description != "" {
			writeParagraph(pdf, width, description)
		}
		writeBullets(pdf, width, entry.Bullets)
		pdf.Ln(gapAfterEntry)
	}
}

func writeSkills(pdf *fpdf.Fpdf, width float64, groups []SkillGroup) {
	if len(groups) == 0 {
		return
	}
	writeSection(pdf, width, "Skills")
	for i := range groups {
		group := groups[i]
		if group.Category == "" || len(group.Skills) == 0 {
			continue
		}
		line := sanitize(group.Category) + ": " + strings.Join(sanitizeAll(group.Skills), ", ")
		pdf.SetFont("Arial", "B", fontSize)
		pdf.MultiCell(width, lineGap, line, "", "L", false)
	}
}

func writeEducation(pdf *fpdf.Fpdf, width float64, entries []Education) {
	if len(entries) == 0 {
		return
	}
	writeSection(pdf, width, "Education")
	for i := range entries {
		entry := entries[i]
		writeEntryTitle(pdf, width, entry.Institution, formatYearRange(entry.StartYear, entry.EndYear, entry.IsCurrent))
		meta := joinNonEmpty(" | ", joinNonEmpty(" - ", entry.Degree, entry.FieldOfStudy), entry.GPA)
		writeMeta(pdf, width, meta)
		if honors := sanitize(entry.Honors); honors != "" {
			pdf.SetFont("Arial", "", fontSize)
			pdf.MultiCell(width, lineGap, "Honors: "+honors, "", "L", false)
		}
		pdf.Ln(gapAfterEntry)
	}
}

// writeExtras renders each populated category under its own sub-heading. Empty
// categories are skipped so the resume never shows a bare heading.
func writeExtras(pdf *fpdf.Fpdf, width float64, groups []ExtraGroup) {
	written := 0
	for i := range groups {
		group := groups[i]
		if group.Category == "" || len(group.Entries) == 0 {
			continue
		}
		if written == 0 {
			writeSection(pdf, width, "Certifications and Awards")
		}
		written++

		pdf.SetFont("Arial", "B", fontSize)
		pdf.MultiCell(width, lineGap, sanitize(titleCase(group.Category)), "", "L", false)

		for j := range group.Entries {
			entry := group.Entries[j]
			headline := sanitize(entry.Title)
			if issuer := sanitize(entry.Issuer); issuer != "" {
				headline += " - " + issuer
			}
			pdf.SetFont("Arial", "", fontSize)
			pdf.MultiCell(width, lineGap, "- "+headline, "", "L", false)
			if issued := sanitize(entry.IssuedDate); issued != "" {
				pdf.SetFont("Arial", "", 9.5)
				pdf.MultiCell(width, 4.2, "  "+issued, "", "L", false)
			}
		}
	}
}

func joinNonEmpty(sep string, values ...string) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := sanitize(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return strings.Join(out, sep)
}

func sanitizeAll(values []string) []string {
	out := make([]string, 0, len(values))
	for i := range values {
		if cleaned := sanitize(values[i]); cleaned != "" {
			out = append(out, cleaned)
		}
	}
	return out
}

// titleCase turns OPEN_SOURCE into "Open Source" for the extras sub-heading.
func titleCase(value string) string {
	words := strings.Fields(strings.ToLower(strings.ReplaceAll(value, "_", " ")))
	for i, word := range words {
		runes := []rune(word)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}
