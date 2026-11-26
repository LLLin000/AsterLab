package labio

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image/png"
	"strings"
)

// DocxDocument describes a structured scientific experiment report to export as `.docx`.
type DocxDocument struct {
	Title     string        `json:"title"`
	Subtitle  string        `json:"subtitle,omitempty"`
	Metadata  []DocxMetaKV  `json:"metadata,omitempty"`
	Sections  []DocxSection `json:"sections,omitempty"`
	Footnotes []string      `json:"footnotes,omitempty"`
}

// DocxMetaKV represents a key/value header metadata item (e.g. Date, Operator, Stage).
type DocxMetaKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// DocxSection represents one logical section or step inside the exported `.docx`.
type DocxSection struct {
	Heading    string       `json:"heading,omitempty"`
	Level      int          `json:"level,omitempty"` // 1, 2, or 3 (defaults to 1)
	Paragraphs []DocxPara   `json:"paragraphs,omitempty"`
	Bullets    []string     `json:"bullets,omitempty"`
	Tables     []DocxTable  `json:"tables,omitempty"`
	Figures    []DocxFigure `json:"figures,omitempty"`
}

// DocxPara represents a paragraph with optional 1-based footnote indices (referencing DocxDocument.Footnotes).
type DocxPara struct {
	Text        string `json:"text"`
	FootnoteIDs []int  `json:"footnoteIds,omitempty"`
}

// DocxTable represents a structured table (headers + data rows).
type DocxTable struct {
	Caption string     `json:"caption,omitempty"`
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// DocxFigure represents an optional embedded PNG figure.
type DocxFigure struct {
	Caption  string `json:"caption,omitempty"`
	PNGBytes []byte `json:"pngBytes"`
}

type embeddedImage struct {
	relID     string
	mediaName string
	pngBytes  []byte
	widthEMU  int64
	heightEMU int64
}

const (
	emuPerPixel = 9525
	maxDocxWidthEMU = 5486400 // 6.0 inches at 914400 EMU/inch
)

// ExportDocx generates a standards-compliant OOXML `.docx` archive in memory
// using Go stdlib `archive/zip` + `encoding/xml`, supporting sections, tables,
// footnotes, and embedded PNG figures.
func ExportDocx(doc DocxDocument) ([]byte, error) {
	if strings.TrimSpace(doc.Title) == "" && len(doc.Sections) == 0 {
		return nil, errors.New("labio: docx document has no title or sections")
	}

	var images []embeddedImage
	// Pre-validate and register embedded PNG figures.
	figRefMap := map[string]int{} // "secIdx:figIdx" -> index in images
	for sIdx, sec := range doc.Sections {
		for fIdx, fig := range sec.Figures {
			if len(fig.PNGBytes) == 0 {
				continue
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(fig.PNGBytes))
			if err != nil {
				return nil, fmt.Errorf("labio: invalid PNG figure at section %d figure %d: %w", sIdx, fIdx, err)
			}
			wEMU := int64(cfg.Width) * emuPerPixel
			hEMU := int64(cfg.Height) * emuPerPixel
			if wEMU <= 0 {
				wEMU = maxDocxWidthEMU / 2
			}
			if hEMU <= 0 {
				hEMU = maxDocxWidthEMU / 3
			}
			if wEMU > maxDocxWidthEMU {
				hEMU = (hEMU * maxDocxWidthEMU) / wEMU
				wEMU = maxDocxWidthEMU
			}
			imgIdx := len(images)
			relID := fmt.Sprintf("rIdImg%d", imgIdx+1)
			mediaName := fmt.Sprintf("image%d.png", imgIdx+1)
			images = append(images, embeddedImage{
				relID:     relID,
				mediaName: mediaName,
				pngBytes:  fig.PNGBytes,
				widthEMU:  wEMU,
				heightEMU: hEMU,
			})
			figRefMap[fmt.Sprintf("%d:%d", sIdx, fIdx)] = imgIdx
		}
	}

	hasFootnotes := len(doc.Footnotes) > 0

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := []struct {
		name string
		data []byte
	}{
		{name: "[Content_Types].xml", data: buildContentTypesXML(len(images) > 0, hasFootnotes)},
		{name: "_rels/.rels", data: buildRootRelsXML()},
		{name: "word/_rels/document.xml.rels", data: buildDocRelsXML(images, hasFootnotes)},
		{name: "word/styles.xml", data: buildStylesXML()},
		{name: "word/document.xml", data: buildDocumentXML(doc, images, figRefMap)},
	}
	if hasFootnotes {
		files = append(files, struct {
			name string
			data []byte
		}{name: "word/footnotes.xml", data: buildFootnotesXML(doc.Footnotes)})
	}
	for _, img := range images {
		files = append(files, struct {
			name string
			data []byte
		}{name: "word/media/" + img.mediaName, data: img.pngBytes})
	}

	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildContentTypesXML(hasPNG, hasFootnotes bool) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	b.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	b.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	if hasPNG {
		b.WriteString(`<Default Extension="png" ContentType="image/png"/>`)
	}
	b.WriteString(`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>`)
	b.WriteString(`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>`)
	if hasFootnotes {
		b.WriteString(`<Override PartName="/word/footnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"/>`)
	}
	b.WriteString(`</Types>`)
	return []byte(b.String())
}

func buildRootRelsXML() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`</Relationships>`)
}

func buildDocRelsXML(images []embeddedImage, hasFootnotes bool) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	b.WriteString(`<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	if hasFootnotes {
		b.WriteString(`<Relationship Id="rIdFootnotes" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/>`)
	}
	for _, img := range images {
		fmt.Fprintf(&b, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/%s"/>`,
			xmlEsc(img.relID), xmlEsc(img.mediaName))
	}
	b.WriteString(`</Relationships>`)
	return []byte(b.String())
}

func buildStylesXML() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:docDefaults><w:rPrDefault><w:rPr>` +
		`<w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Microsoft YaHei"/>` +
		`<w:sz w:val="22"/>` +
		`</w:rPr></w:rPrDefault></w:docDefaults>` +
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:rPr><w:b/><w:sz w:val="36"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:rPr><w:b/><w:sz w:val="30"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:rPr><w:b/><w:sz w:val="26"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:rPr><w:b/><w:sz w:val="24"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="FootnoteText"><w:name w:val="footnote text"/><w:rPr><w:sz w:val="18"/></w:rPr></w:style>` +
		`</w:styles>`)
}

func buildFootnotesXML(footnotes []string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	b.WriteString(`<w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote>`)
	b.WriteString(`<w:footnote w:type="continuationSeparator" w:id="0"><w:p><w:r><w:continuationSeparator/></w:r></w:p></w:footnote>`)
	for idx, note := range footnotes {
		id := idx + 1
		fmt.Fprintf(&b, `<w:footnote w:id="%d"><w:p><w:pPr><w:pStyle w:val="FootnoteText"/></w:pPr><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p></w:footnote>`,
			id, xmlEsc(note))
	}
	b.WriteString(`</w:footnotes>`)
	return []byte(b.String())
}

func buildDocumentXML(doc DocxDocument, images []embeddedImage, figRefMap map[string]int) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">`)
	b.WriteString(`<w:body>`)

	if strings.TrimSpace(doc.Title) != "" {
		writeStyledParagraph(&b, "Title", doc.Title)
	}
	if strings.TrimSpace(doc.Subtitle) != "" {
		writeStyledParagraph(&b, "Normal", doc.Subtitle)
	}
	if len(doc.Metadata) > 0 {
		rows := make([][]string, 0, len(doc.Metadata))
		for _, kv := range doc.Metadata {
			rows = append(rows, []string{kv.Key, kv.Value})
		}
		writeTableXML(&b, DocxTable{Headers: []string{"Field", "Value"}, Rows: rows})
	}

	docPrID := 1
	for sIdx, sec := range doc.Sections {
		if strings.TrimSpace(sec.Heading) != "" {
			style := "Heading1"
			switch sec.Level {
			case 2:
				style = "Heading2"
			case 3:
				style = "Heading3"
			}
			writeStyledParagraph(&b, style, sec.Heading)
		}
		for _, p := range sec.Paragraphs {
			b.WriteString(`<w:p>`)
			fmt.Fprintf(&b, `<w:r><w:t xml:space="preserve">%s</w:t></w:r>`, xmlEsc(p.Text))
			for _, fnID := range p.FootnoteIDs {
				if fnID >= 1 && fnID <= len(doc.Footnotes) {
					fmt.Fprintf(&b, `<w:r><w:rPr><w:vertAlign w:val="superscript"/></w:rPr><w:footnoteReference w:id="%d"/></w:r>`, fnID)
				}
			}
			b.WriteString(`</w:p>`)
		}
		for _, bullet := range sec.Bullets {
			writeStyledParagraph(&b, "Normal", "• "+bullet)
		}
		for _, tbl := range sec.Tables {
			if strings.TrimSpace(tbl.Caption) != "" {
				writeStyledParagraph(&b, "Normal", tbl.Caption)
			}
			writeTableXML(&b, tbl)
		}
		for fIdx, fig := range sec.Figures {
			imgIdx, ok := figRefMap[fmt.Sprintf("%d:%d", sIdx, fIdx)]
			if !ok {
				continue
			}
			img := images[imgIdx]
			writeImageParagraphXML(&b, docPrID, img)
			docPrID++
			if strings.TrimSpace(fig.Caption) != "" {
				writeStyledParagraph(&b, "Normal", fig.Caption)
			}
		}
	}

	b.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>`)
	b.WriteString(`</w:body></w:document>`)
	return []byte(b.String())
}

func writeStyledParagraph(b *strings.Builder, styleID, text string) {
	b.WriteString(`<w:p>`)
	if styleID != "" && styleID != "Normal" {
		fmt.Fprintf(b, `<w:pPr><w:pStyle w:val="%s"/></w:pPr>`, xmlEsc(styleID))
	}
	fmt.Fprintf(b, `<w:r><w:t xml:space="preserve">%s</w:t></w:r>`, xmlEsc(text))
	b.WriteString(`</w:p>`)
}

func writeTableXML(b *strings.Builder, tbl DocxTable) {
	b.WriteString(`<w:tbl><w:tblPr><w:tblBorders>` +
		`<w:top w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>` +
		`<w:left w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>` +
		`<w:bottom w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>` +
		`<w:right w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>` +
		`<w:insideH w:val="single" w:sz="4" w:space="0" w:color="E2E8F0"/>` +
		`<w:insideV w:val="single" w:sz="4" w:space="0" w:color="E2E8F0"/>` +
		`</w:tblBorders></w:tblPr>`)

	if len(tbl.Headers) > 0 {
		b.WriteString(`<w:tr>`)
		for _, h := range tbl.Headers {
			b.WriteString(`<w:tc><w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="F1F5F9"/></w:tcPr>`)
			fmt.Fprintf(b, `<w:p><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, xmlEsc(h))
			b.WriteString(`</w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	for _, row := range tbl.Rows {
		b.WriteString(`<w:tr>`)
		for _, cell := range row {
			b.WriteString(`<w:tc>`)
			fmt.Fprintf(b, `<w:p><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, xmlEsc(cell))
			b.WriteString(`</w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
}

func writeImageParagraphXML(b *strings.Builder, docPrID int, img embeddedImage) {
	fmt.Fprintf(b,
		`<w:p><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:docPr id="%d" name="%s"/>`+
			`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
			`<pic:pic>`+
			`<pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr>`+
			`<pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>`+
			`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm>`+
			`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>`+
			`</pic:pic></a:graphicData></a:graphic>`+
			`</wp:inline></w:drawing></w:r></w:p>`,
		img.widthEMU, img.heightEMU,
		docPrID, xmlEsc(img.mediaName),
		docPrID, xmlEsc(img.mediaName),
		xmlEsc(img.relID),
		img.widthEMU, img.heightEMU,
	)
}

func xmlEsc(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
