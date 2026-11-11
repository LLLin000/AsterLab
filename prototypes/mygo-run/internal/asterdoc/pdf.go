package asterdoc

import (
	"fmt"
	"io"
	"strings"
)

// ExportPDF writes a minimal, deterministic PDF from a document revision and
// rendered module previews. It uses only the standard library.
func ExportPDF(w io.Writer, title, revision, sourceHash string, previews []string) error {
	var body strings.Builder
	body.WriteString("%PDF-1.4\n")
	objects := []string{}
	add := func(s string) int {
		objects = append(objects, s)
		return len(objects)
	}
	// 1: catalog
	cat := add("<< /Type /Catalog /Pages 2 0 R >>")
	// 2: pages (placeholder, filled later)
	pagesIdx := add("PAGES")
	// 3: font
	fontIdx := add("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	// content stream
	var content strings.Builder
	content.WriteString("BT\n/F1 11 Tf\n50 780 Td\n")
	writeLine := func(line string) {
		line = pdfEscape(line)
		content.WriteString(fmt.Sprintf("(%s) Tj\n0 -14 Td\n", line))
	}
	writeLine(sanitize(title))
	writeLine(fmt.Sprintf("Revision: %s", revision))
	writeLine(fmt.Sprintf("Source hash: %s", sourceHash))
	writeLine("")
	for _, preview := range previews {
		for _, line := range strings.Split(preview, "\n") {
			writeLine(line)
		}
		writeLine("")
	}
	content.WriteString("ET\n")
	stream := content.String()
	pageContent := fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream)
	contentIdx := add(pageContent)
	page := fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", fontIdx, contentIdx)
	pageIdx := add(page)
	// fix pages
	objects[pagesIdx-1] = fmt.Sprintf("<< /Type /Pages /Kids [%d 0 R] /Count 1 >>", pageIdx)
	// write objects
	offsets := make([]int, len(objects)+1)
	for i, obj := range objects {
		offsets[i+1] = body.Len()
		fmt.Fprintf(&body, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xrefPos := body.Len()
	fmt.Fprintf(&body, "xref\n0 %d\n", len(objects)+1)
	body.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&body, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&body, "trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, cat, xrefPos)
	_, err := io.WriteString(w, body.String())
	return err
}

func pdfEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
