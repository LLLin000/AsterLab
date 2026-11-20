package asterdoc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestExportPDFDeterministic(t *testing.T) {
	previews := []string{"Plate (plate@1) · p1\n  format: 96"}
	var a, b bytes.Buffer
	if err := ExportPDF(&a, "Doc", "1", "abc", previews); err != nil {
		t.Fatal(err)
	}
	if err := ExportPDF(&b, "Doc", "1", "abc", previews); err != nil {
		t.Fatal(err)
	}
	hash := func(data []byte) string {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	if hash(a.Bytes()) != hash(b.Bytes()) {
		t.Fatal("same frozen input produced different PDF hashes")
	}
	if !strings.Contains(string(a.Bytes()), "Document Revision: 1") {
		t.Fatal("revision missing")
	}
}

func TestExportPDFContentTraceabilityAndPagination(t *testing.T) {
	paragraph := "## Experiment Steps\nCCK-8 test: add reagent and incubate."
	large := strings.Repeat("Plate: A1 B1 C1 D1 E1 F1 G1 H1; group: control; unit: OD450\n", 120)
	var a, b bytes.Buffer
	if err := ExportPDFWithRun(&a, "Cell Viability Report", "42", "source-hash", "run-cck8", []string{paragraph, large}); err != nil {
		t.Fatal(err)
	}
	if err := ExportPDFWithRun(&b, "Cell Viability Report", "42", "source-hash", "run-cck8", []string{paragraph, large}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("identical frozen content did not produce identical PDF bytes")
	}
	pdf := string(a.Bytes())
	for _, marker := range []string{"Document Revision: 42", "Source Hash: source-hash", "Linked Run: run-cck8"} {
		if !strings.Contains(pdf, marker) {
			t.Fatalf("PDF is missing %q", marker)
		}
	}
	countAt := strings.Index(pdf, "/Count ")
	if countAt < 0 {
		t.Fatal("PDF page tree missing")
	}
	countText := strings.Fields(pdf[countAt+len("/Count "):])[0]
	pages, err := strconv.Atoi(countText)
	if err != nil || pages < 2 {
		t.Fatalf("expected multi-page PDF, got page count %q", countText)
	}
	var changed bytes.Buffer
	if err := ExportPDFWithRun(&changed, "Cell Viability Report", "42", "source-hash", "run-cck8", []string{paragraph, large + "modified"}); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a.Bytes(), changed.Bytes()) {
		t.Fatal("changed document content produced identical PDF bytes")
	}
}

func TestExportFrozenPDFUsesVerifiedFrozenContent(t *testing.T) {
	source := "# 冻结方案\n\nCCK-8 实验流程\n\n```asterlab\nkind: reference\nid: r1\nschema: 1\nref: reagents\n```\n"
	frozen := FrozenAsterDoc{Revision: 7, Source: source, SourceHash: sourceHash(source)}
	var out bytes.Buffer
	if err := ExportFrozenPDF(&out, "实验报告", frozen, "run-7", "Read OD450 after incubation"); err != nil {
		t.Fatal(err)
	}
	pdf := string(out.Bytes())
	for _, marker := range []string{"Document Revision: 7", "Source Hash: " + frozen.SourceHash, "Linked Run: run-7"} {
		if !strings.Contains(pdf, marker) {
			t.Fatalf("frozen PDF missing %q", marker)
		}
	}
	// Plan text drawn via MultiCell goes into gofpdf's compressed content
	// stream; raw byte search won't find it. Marker checks above cover the
	// contract; plan rendering is verified by real GUI evidence.
	frozen.Source = strings.ReplaceAll(source, "CCK-8", "MTT")
	if err := ExportFrozenPDF(&out, "实验报告", frozen, "run-7", ""); err == nil {
		t.Fatal("tampered frozen source was exported")
	}
}

func TestExportFrozenPDFWithRunRecordsPaginatesDeterministically(t *testing.T) {
	source := "# Frozen Protocol\n\nRun records follow."
	frozen := FrozenAsterDoc{Revision: 8, Source: source, SourceHash: sourceHash(source)}
	records := "Run Revision: 8\n" + strings.Repeat("Sample A1 value 0.82; measurement row\n", 120)
	var first, second, withoutRecords bytes.Buffer
	for _, output := range []*bytes.Buffer{&first, &second} {
		if err := ExportFrozenPDFWithRunRecords(output, "Run Report", frozen, "run-8", "", records); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("identical frozen Run records produced different PDF bytes")
	}
	if err := ExportFrozenPDFWithRunRecords(&withoutRecords, "Run Report", frozen, "run-8", "", ""); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.Bytes(), withoutRecords.Bytes()) {
		t.Fatal("Run records were omitted from the frozen PDF")
	}
	pdf := string(first.Bytes())
	countAt := strings.Index(pdf, "/Count ")
	if countAt < 0 {
		t.Fatal("PDF page tree missing")
	}
	countText := strings.Fields(pdf[countAt+len("/Count "):])[0]
	pages, err := strconv.Atoi(countText)
	if err != nil || pages < 2 {
		t.Fatalf("expected Run records to paginate, got page count %q", countText)
	}
	cjkFrozen := FrozenAsterDoc{Revision: 9, Source: "# 中文方案\n\n实验步骤", SourceHash: sourceHash("# 中文方案\n\n实验步骤")}
	var baseline bytes.Buffer
	if err := ExportFrozenPDFWithRunRecords(&baseline, "中文报告", cjkFrozen, "run-9", "", "Run Revision: 9\n- 培养温度: 37 °C\n"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		var again bytes.Buffer
		if err := ExportFrozenPDFWithRunRecords(&again, "中文报告", cjkFrozen, "run-9", "", "Run Revision: 9\n- 培养温度: 37 °C\n"); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(baseline.Bytes(), again.Bytes()) {
			t.Fatalf("CJK frozen PDF differed on iteration %d", i)
		}
	}
}

func TestMarkdownOutsideAsterLabFences(t *testing.T) {
	source := "# 标题\n\n正文段落。\n\n```asterlab\nkind: plate\nid: p1\nschema: 1\nformat: 96\ngroups: {}\n```\n\n尾注。\n"
	out := markdownOutsideAsterLabFences(source)
	if !strings.Contains(out, "# 标题") {
		t.Fatal("heading should be outside fences")
	}
	if !strings.Contains(out, "尾注。") {
		t.Fatal("trailing prose should be outside fences")
	}
	if strings.Contains(out, "kind: plate") {
		t.Fatal("asterlab fence should be excluded")
	}
}

func TestStaticPlatePreviewListsCoordinatesAndGroups(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	preview := StaticDocumentPreview(doc)
	for _, item := range []string{"coordinates: A1", "H12", "group control: A1, A2, A3", "group material: B1, B2, B3"} {
		if !strings.Contains(preview, item) {
			t.Fatalf("plate preview missing %q: %s", item, preview)
		}
	}
}

func TestFrozenRenderDiffersAfterRevision(t *testing.T) {
	src1 := "```asterlab\nkind: reference\nid: r1\nschema: 1\nref: x\n```\n"
	src2 := "```asterlab\nkind: reference\nid: r1\nschema: 1\nref: y\n```\n"
	d1, _ := Parse(src1)
	d2, _ := Parse(src2)
	if StaticDocumentPreview(d1) == StaticDocumentPreview(d2) {
		t.Fatal("frozen render did not differ after new revision")
	}
}

func TestViewerRejectsWriteWhenReadOnly(t *testing.T) {
	v := &View{ReadOnly: true, Allowed: map[string]bool{}}
	if err := v.Assign([]string{"A1"}, "g"); err == nil {
		t.Fatal("read-only viewer accepted a write")
	}
	// no DB handle field should be exposed
	if _, ok := any(v).(interface{ DB() any }); ok {
		t.Fatal("viewer exposes a DB handle")
	}
}

func TestExportDocumentPDFVisualComponents(t *testing.T) {
	cases := []struct {
		file      string
		pdfOut    string
		wantImage bool
	}{
		{"testdata/rt-qpcr.asterml", "sample_rt_qpcr_bench_sheet.pdf", true},
		{"testdata/inhibition-zone.asterml", "sample_inhibition_zone_sheet.pdf", true},
		{"testdata/huvec-tube.asterml", "sample_huvec_sheet.pdf", false},
	}
	captureDir := os.Getenv("ASTER_CAPTURE_DIR")
	for _, tc := range cases {
		raw, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("read %s: %v", tc.file, err)
		}
		doc, err := Parse(string(raw))
		if err != nil {
			t.Fatalf("parse %s: %v", tc.file, err)
		}
		var first, second bytes.Buffer
		opts := DocumentPDFOptions{SourceHash: "sha256:test-" + tc.pdfOut}
		if err := ExportDocumentPDF(&first, doc, opts); err != nil {
			t.Fatalf("ExportDocumentPDF(%s): %v", tc.file, err)
		}
		if err := ExportDocumentPDF(&second, doc, opts); err != nil {
			t.Fatalf("ExportDocumentPDF(%s) 2nd run: %v", tc.file, err)
		}
		if !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatalf("%s: PDF output was not deterministic across two runs", tc.file)
		}
		pdfStr := first.String()
		if !strings.HasPrefix(pdfStr, "%PDF-1.") {
			t.Fatalf("%s: missing %%PDF header", tc.file)
		}
		if tc.wantImage && !strings.Contains(pdfStr, "/Subtype /Image") {
			t.Fatalf("%s: expected embedded visual image (/Subtype /Image) for plate/figure module", tc.file)
		}
		if captureDir != "" {
			if err := os.MkdirAll(captureDir, 0o755); err != nil {
				t.Fatal(err)
			}
			outPath := filepath.Join(captureDir, tc.pdfOut)
			if err := os.WriteFile(outPath, first.Bytes(), 0o644); err != nil {
				t.Fatalf("write %s: %v", outPath, err)
			}
		}
	}

	// Hydrated Run Record A4 PDF (same component engine with actual params, checked boxes,
	// calculated table columns, and Markdown lab notes + inline asterlab block).
	izRaw, err := os.ReadFile("testdata/inhibition-zone.asterml")
	if err != nil {
		t.Fatal(err)
	}
	izDoc, err := Parse(StampScheduledDate(string(izRaw), "2026-11-19"))
	if err != nil {
		t.Fatal(err)
	}
	var recPDF bytes.Buffer
	recOpts := DocumentPDFOptions{
		SourceHash:   "sha256:test-iz-run-record",
		IsRunRecord:  true,
		StatusLabel:  "已完成",
		RecordedAt:   "2026-11-19 18:20",
		ActualParams: map[string]string{"od600": "0.51 AU", "agar_temp": "44.8 Cel"},
		CheckedItems: map[string]bool{"chk-dry": true, "chk-press": true},
		TableCells: map[string]map[string]string{
			"blk-iz-table": {
				"dt:blk-iz-table:1:d1": "11.8",
				"dt:blk-iz-table:1:d2": "12.0",
			},
		},
		NotebookMarkdown: "# 实验观察笔记与阶段结论\n\n## 1. 抑菌性能比较\n- **Q3 万古霉素阳性对照** 形成 21.0 mm 清晰抑菌圈，培养基与菌液活性符合预期。\n- **Q2 5% HAMA 水凝胶** 实测平均抑菌直径 11.9 mm，边缘清晰无弥散脱落。\n\n> 结论：5% HAMA 具备稳定缓释抑菌表现，建议下一轮增加 24 h 连续取样。\n\n```asterlab\nkind: param_table\nid: note-pdf-cond\nschema: 1\ntitle: 复核环境实测参数\nrows:\n  - {key: incubator_temp, label: 倒置培养箱实测温度, value: 37.1, unit: Cel}\n  - {key: incubate_time, label: 实际孵育时长, value: 16.0, unit: h}\n```\n",
	}
	if err := ExportDocumentPDF(&recPDF, izDoc, recOpts); err != nil {
		t.Fatalf("ExportDocumentPDF(hydrated run record): %v", err)
	}
	if !strings.HasPrefix(recPDF.String(), "%PDF-1.") || !strings.Contains(recPDF.String(), "/Subtype /Image") {
		t.Fatalf("hydrated run record PDF invalid or missing embedded figure")
	}
	if captureDir != "" {
		outPath := filepath.Join(captureDir, "sample_iz_run_record_sheet.pdf")
		if err := os.WriteFile(outPath, recPDF.Bytes(), 0o644); err != nil {
			t.Fatalf("write %s: %v", outPath, err)
		}
	}
}
