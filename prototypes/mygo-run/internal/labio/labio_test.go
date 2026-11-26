package labio

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"
)

func TestImportPlateMatrixCSVAndTSVAndLongTable(t *testing.T) {
	// 1. Instrument-style CSV with metadata rows above 96-well matrix (A..H x 1..12).
	csvInput := strings.Join([]string{
		"Instrument,BioTek Synergy H1",
		"Wavelength,450 nm",
		",1,2,3,4,5,6,7,8,9,10,11,12",
		"A,0.102,0.105,0.110,0.450,0.462,0.455,0.890,0.910,0.905,0.045,0.046,OVRFLW",
		"B,0.101,0.104,0.108,0.448,0.460,0.451,0.885,0.902,0.899,0.044,0.045,0.046",
		"C,0.100,0.103,0.109,0.440,0.452,0.449,0.870,0.895,0.891,0.043,0.044,0.045",
		"D,0.099,0.102,0.107,0.435,0.448,0.442,0.865,0.888,0.884,0.042,0.043,0.044",
		"E,0.103,0.106,0.111,0.441,0.455,0.450,0.872,0.899,0.895,0.045,0.046,0.047",
		"F,0.104,0.107,0.112,0.442,0.456,0.452,0.875,0.901,0.896,0.046,0.047,0.048",
		"G,0.105,0.108,0.113,0.445,0.458,0.453,0.878,0.903,0.898,0.047,0.048,0.049",
		"H,0.106,0.109,0.114,0.446,0.459,0.454,0.880,0.905,0.900,0.048,0.049,0.050",
	}, "\n")

	res, err := ImportPlateMatrix([]byte(csvInput))
	if err != nil {
		t.Fatalf("ImportPlateMatrix CSV failed: %v", err)
	}
	if res.Format != 96 || res.Rows != 8 || res.Cols != 12 {
		t.Fatalf("unexpected plate dimensions: fmt=%d %dx%d", res.Format, res.Rows, res.Cols)
	}
	if w := res.Wells["A1"]; w.Value == nil || *w.Value != 0.102 {
		t.Fatalf("A1 mismatch: %+v", w)
	}
	if w := res.Wells["A12"]; w.Value != nil || w.Raw != "OVRFLW" {
		t.Fatalf("A12 expected non-numeric OVRFLW, got %+v", w)
	}
	if len(res.Ordered) != 96 {
		t.Fatalf("expected 96 ordered wells, got %d", len(res.Ordered))
	}

	// 2. TSV clipboard paste format.
	tsvInput := strings.ReplaceAll(csvInput, ",", "\t")
	resTSV, err := ImportPlateMatrix([]byte(tsvInput))
	if err != nil {
		t.Fatalf("ImportPlateMatrix TSV failed: %v", err)
	}
	if resTSV.Table.Format != "tsv" || *resTSV.Wells["H12"].Value != 0.050 {
		t.Fatalf("TSV mismatch: fmt=%s H12=%+v", resTSV.Table.Format, resTSV.Wells["H12"])
	}

	// 3. Long-table format with Well column (including 384-well coord P24).
	longCSV := strings.Join([]string{
		"Well,Group,Timepoint,Replicate,OD450",
		"A01,Control,24h,1,0.123",
		"B2,Drug-10uM,24h,2,0.456",
		"P24,Blank,24h,1,0.041",
	}, "\n")
	resLong, err := ImportPlateMatrix([]byte(longCSV))
	if err != nil {
		t.Fatalf("ImportPlateMatrix long table failed: %v", err)
	}
	if resLong.Format != 384 {
		t.Fatalf("expected 384-well inferred for P24, got %d", resLong.Format)
	}
	if w := resLong.Wells["A1"]; w.Value == nil || *w.Value != 0.123 || w.Group != "Control" {
		t.Fatalf("A1 long-table mismatch: %+v", w)
	}
	if w := resLong.Wells["P24"]; w.Value == nil || *w.Value != 0.041 {
		t.Fatalf("P24 long-table mismatch: %+v", w)
	}
}

func TestImportPlateMatrixMinimalXLSX(t *testing.T) {
	xlsxBytes := buildMinimalXLSX(t)
	res, err := ImportPlateMatrix(xlsxBytes)
	if err != nil {
		t.Fatalf("ImportPlateMatrix XLSX failed: %v", err)
	}
	if res.Table == nil || res.Table.Format != "xlsx" || res.Table.Sheet != "OD450" {
		t.Fatalf("unexpected XLSX table metadata: %+v", res.Table)
	}
	if w := res.Wells["A1"]; w.Value == nil || *w.Value != 0.25 {
		t.Fatalf("XLSX A1 mismatch: %+v", w)
	}
	if w := res.Wells["B2"]; w.Value == nil || *w.Value != 0.875 {
		t.Fatalf("XLSX B2 mismatch: %+v", w)
	}
}

func TestExportTidyCSV(t *testing.T) {
	obs := []TidyObservation{
		{
			Experiment: "20261126_CCK8筛选",
			Group:      "Control, DMSO",
			Role:       "control",
			Timepoint:  "24h",
			Replicate:  1,
			Well:       "A1",
			SampleID:   "s-1",
			Metric:     "OD450",
			Value:      0.8125,
			Unit:       "AU",
			Extra:      map[string]string{"passage": "P4", "operator": "Lin"},
		},
		{
			Experiment: "20261126_CCK8筛选",
			Group:      "Drug-10uM",
			Role:       "treatment",
			Timepoint:  "24h",
			Replicate:  2,
			Well:       "B4",
			Metric:     "OD450",
			Missing:    true,
			Unit:       "AU",
			Extra:      map[string]string{"passage": "P4"},
		},
	}

	csvBytes, err := ExportTidyCSV(obs, TidyCSVOptions{IncludeUTF8BOM: true, Precision: 3})
	if err != nil {
		t.Fatalf("ExportTidyCSV failed: %v", err)
	}
	if !bytes.HasPrefix(csvBytes, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatalf("expected UTF-8 BOM prefix")
	}
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(csvBytes, []byte{0xef, 0xbb, 0xbf})))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("re-parse tidy CSV failed: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 rows (header + 2 obs), got %d", len(records))
	}
	// Extra columns sorted alphabetically: operator, passage
	if records[0][10] != "operator" || records[0][11] != "passage" {
		t.Fatalf("unexpected sorted extra headers: %v", records[0])
	}
	if records[1][1] != "Control, DMSO" || records[1][8] != "0.812" && records[1][8] != "0.813" {
		t.Fatalf("unexpected row 1 values: %v", records[1])
	}
	if records[2][8] != "NA" {
		t.Fatalf("expected NA for missing value, got %q", records[2][8])
	}
}

func TestExportDocxValidOOXMLWithTablesFootnotesAndPNG(t *testing.T) {
	pngBytes := buildTestPNG(t, 120, 80)
	doc := DocxDocument{
		Title:    "20261126 CCK-8 细胞活力实验报告",
		Subtitle: "HUVEC 细胞药物梯度筛选",
		Metadata: []DocxMetaKV{
			{Key: "日期", Value: "2026-11-26"},
			{Key: "实验员", Value: "Lin"},
		},
		Footnotes: []string{
			"所有读数已扣除空白孔（Blank）本底 OD450。",
		},
		Sections: []DocxSection{
			{
				Heading: "1. 实验设计与步骤",
				Level:   1,
				Paragraphs: []DocxPara{
					{Text: "细胞接种密度 5000 cells/well，加药孵育 24h 后测定吸光度。", FootnoteIDs: []int{1}},
				},
				Bullets: []string{"Control: 0.1% DMSO", "Treatment: Drug 10 uM"},
				Tables: []DocxTable{
					{
						Caption: "表 1. OD450 汇总",
						Headers: []string{"组别", "均值", "SD"},
						Rows: [][]string{
							{"Control", "0.902", "0.014"},
							{"Drug-10uM", "0.451", "0.009"},
						},
					},
				},
				Figures: []DocxFigure{
					{Caption: "图 1. 荧光显色缩略图", PNGBytes: pngBytes},
				},
			},
		},
	}

	docxBytes, err := ExportDocx(doc)
	if err != nil {
		t.Fatalf("ExportDocx failed: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
	if err != nil {
		t.Fatalf("ExportDocx output is not valid zip: %v", err)
	}

	parts := map[string][]byte{}
	for _, f := range zr.File {
		rc, oErr := f.Open()
		if oErr != nil {
			t.Fatal(oErr)
		}
		raw, rErr := io.ReadAll(rc)
		_ = rc.Close()
		if rErr != nil {
			t.Fatal(rErr)
		}
		parts[f.Name] = raw
	}

	requiredParts := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"word/_rels/document.xml.rels",
		"word/styles.xml",
		"word/footnotes.xml",
		"word/document.xml",
		"word/media/image1.png",
	}
	for _, name := range requiredParts {
		if _, ok := parts[name]; !ok {
			t.Fatalf("missing required OOXML part %q", name)
		}
	}

	// Validate every XML part is strictly well-formed XML.
	for name, raw := range parts {
		if !strings.HasSuffix(name, ".xml") && !strings.HasSuffix(name, ".rels") {
			continue
		}
		dec := xml.NewDecoder(bytes.NewReader(raw))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("invalid XML in %q: %v\n%s", name, err, string(raw))
			}
		}
	}

	if !bytes.Equal(parts["word/media/image1.png"], pngBytes) {
		t.Fatalf("embedded PNG bytes mismatch")
	}
	docXML := string(parts["word/document.xml"])
	for _, want := range []string{"CCK-8 细胞活力实验报告", "Drug-10uM", "w:footnoteReference", "rIdImg1"} {
		if !strings.Contains(docXML, want) {
			t.Fatalf("word/document.xml missing %q", want)
		}
	}
}

func buildMinimalXLSX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="OD450" sheetId="1" r:id="rId1"/></sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`,
		"xl/sharedStrings.xml": `<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <si><t>A</t></si>
  <si><t>B</t></si>
</sst>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="B1"><v>1</v></c><c r="C1"><v>2</v></c>
    </row>
    <row r="2">
      <c r="A2" t="s"><v>0</v></c><c r="B2"><v>0.25</v></c><c r="C2"><v>0.30</v></c>
    </row>
    <row r="3">
      <c r="A3" t="s"><v>1</v></c><c r="B3"><v>0.80</v></c><c r="C3"><v>0.875</v></c>
    </row>
  </sheetData>
</worksheet>`,
	}
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func buildTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 30, G: 130, B: 220, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
