package asterdoc

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

var chineseFontCandidates = []string{
	"C:/Windows/Fonts/simhei.ttf",
	"C:/Windows/Fonts/simsunb.ttf",
	"C:/Windows/Fonts/NotoSansSC-VF.ttf",
	"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
}

// ExportPDF writes a deterministic, multi-page PDF from a frozen document's
// rendered Markdown and module previews.
func ExportPDF(w io.Writer, title, revision, sourceHash string, previews []string) error {
	return ExportPDFWithRun(w, title, revision, sourceHash, "", previews)
}

// ExportPDFWithRun includes the linked Run identifier in the traceability stamp.
// gofpdf is used instead of the former text-only writer because it supports
// paginated wrapping and embedded UTF-8 TrueType fonts for Chinese output.
func ExportPDFWithRun(w io.Writer, title, revision, sourceHash, runID string, previews []string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCreationDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	pdf.SetModificationDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	pdf.SetMargins(16, 16, 16)
	pdf.SetAutoPageBreak(true, 16)
	font := "Helvetica"
	needsCJK := containsCJK(title, revision, sourceHash, runID, previews)
	if needsCJK {
		for _, path := range chineseFontCandidates {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			pdf.AddUTF8FontFromBytes("AsterCJK", "", data)
			if err := pdf.Error(); err != nil {
				return fmt.Errorf("load CJK font %q: %w", path, err)
			}
			font = "AsterCJK"
			break
		}
		if font == "Helvetica" {
			return fmt.Errorf("cannot export Chinese text: no CJK TrueType font found")
		}
	}
	pdf.SetTitle(title, true)
	traceability := "Document Revision: " + revision + "; Source Hash: " + sourceHash
	if runID != "" {
		traceability += "; Linked Run: " + runID
	}
	pdf.SetSubject(traceability, false)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont(font, "", 8)
		pdf.CellFormat(0, 8, fmt.Sprintf("Page %d", pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	pdf.AddPage()
	pdf.SetFont(font, "", 18)
	pdf.MultiCell(0, 9, title, "", "", false)
	pdf.SetFont(font, "", 9)
	pdf.MultiCell(0, 5, "Document Revision: "+revision, "", "", false)
	pdf.MultiCell(0, 5, "Source Hash: "+sourceHash, "", "", false)
	if runID != "" {
		pdf.MultiCell(0, 5, "Linked Run: "+runID, "", "", false)
	}
	pdf.Ln(5)
	for _, preview := range previews {
		for _, line := range strings.Split(preview, "\n") {
			renderPDFLine(pdf, font, line)
		}
		pdf.Ln(4)
	}
	return pdf.Output(w)
}

// ExportFrozenPDF exports a verified frozen document to PDF. It refuses to
// export if the source no longer matches the pinned source_hash.
func ExportFrozenPDF(w io.Writer, title string, frozen FrozenAsterDoc, runID, plan string) error {
	return ExportFrozenPDFWithRunRecords(w, title, frozen, runID, plan, "")
}

// ExportFrozenPDFWithRunRecords renders a verified frozen document into a
// component-level A4 PDF with optional Run records appended.
func ExportFrozenPDFWithRunRecords(w io.Writer, title string, frozen FrozenAsterDoc, runID, plan, runRecords string) error {
	if !frozen.VerifySourceHash(frozen.Source) {
		return fmt.Errorf("frozen source hash mismatch: document tampered after freeze")
	}
	doc, err := Parse(frozen.Source)
	if err != nil {
		return fmt.Errorf("frozen document parse failed: %w", err)
	}
	return ExportDocumentPDF(w, doc, DocumentPDFOptions{
		Title:      title,
		Revision:   fmt.Sprint(frozen.Revision),
		SourceHash: frozen.SourceHash,
		RunID:      runID,
		Plan:       plan,
		RunRecords: runRecords,
	})
}

func markdownOutsideAsterLabFences(source string) string {
	var markdown strings.Builder
	inAsterLab := false
	for _, line := range strings.SplitAfter(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inAsterLab || strings.TrimSpace(strings.TrimPrefix(trimmed, "```")) == "asterlab" {
				inAsterLab = !inAsterLab
				continue
			}
		}
		if !inAsterLab {
			markdown.WriteString(line)
		}
	}
	return markdown.String()
}

func containsCJK(title, revision, sourceHash, runID string, previews []string) bool {
	text := strings.Join(append([]string{title, revision, sourceHash, runID}, previews...), "\n")
	for _, r := range text {
		if r >= 0x3400 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

func renderPDFLine(pdf *gofpdf.Fpdf, font, line string) {
	line = strings.TrimRight(line, " \t")
	if strings.TrimSpace(line) == "" {
		pdf.Ln(3)
		return
	}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "|---") || strings.HasPrefix(trimmed, "| ---") {
		return
	}
	size := 10.0
	if strings.HasPrefix(trimmed, "#") {
		level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
		trimmed = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		size = 16 - float64(level)
		if size < 11 {
			size = 11
		}
	} else if strings.HasPrefix(trimmed, "|") {
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		trimmed = strings.Join(cells, "   │   ")
	} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		trimmed = "- " + strings.TrimSpace(trimmed[2:])
	}
	trimmed = strings.ReplaceAll(trimmed, "**", "")
	trimmed = strings.ReplaceAll(trimmed, "`", "")
	pdf.SetFont(font, "", size)
	pdf.MultiCell(0, 5, trimmed, "", "", false)
}
