package protocol

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/modules/runs"
)

func archiveFixture() (record.RecordProjection, string) {
	snap := runs.RunSnapshot{
		RunID: "run-1", Title: "HUVEC 成管",
		DesignID: "design-1", DesignRevision: 3, Status: "completed",
		Tasks: []runs.RunTask{
			{
				Key: "task-cure", Title: "405nm 固化", Timepoint: "0h",
				Status: "completed", DesignStepID: "step-2",
				ParametersJSON: `[{"key":"cure_s","label":"固化秒数","value":60,"unit":"s"}]`,
			},
		},
		Events: []runs.RunEvent{
			{
				ID: "evt-cure", EventType: "operation", TaskKey: "task-cure",
				Timepoint: "0h", OccurredAt: "2026-11-10T10:00:00+08:00",
				RecordedAt: "2026-11-10T10:01:00+08:00",
				DataJSON:   `{"actual_parameters":[{"key":"cure_s","value":75,"unit":"s"}]}`,
			},
			{
				ID: "evt-read", EventType: "measurement", TaskKey: "task-cure",
				Timepoint: "6h", BatchID: "batch-1",
				OccurredAt: "2026-11-10T16:00:00+08:00", RecordedAt: "2026-11-10T16:01:00+08:00",
				ResultCells: []runs.ResultCell{
					{Row: 0, Column: 0, SampleID: "S1", Field: "OD450", Value: new(0.82)},
				},
			},
		},
	}
	assets := []runs.Asset{
		{ID: "img-1", RelativePath: "run-1/6h/S1-fov1.png", MediaType: "image",
			Link: runs.AssetLink{RunID: "run-1", TaskKey: "task-cure", SampleID: "S1"}},
	}
	proj := record.Project(snap, assets)
	return proj, "# 研判\n\n结论见对比 @matrix(6h)\n"
}

func TestBuildArchiveFlattensRefs(t *testing.T) {
	proj, prose := archiveFixture()
	arch := BuildArchive(proj, prose, time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	if arch.Sealed {
		t.Fatal("unsigned prose must not read sealed")
	}
	if len(arch.SourceRefs) != 1 || arch.SourceRefs[0].Raw != "@matrix(6h)" {
		t.Fatalf("source refs = %+v", arch.SourceRefs)
	}
	doc := arch.Markdown()
	for _, want := range []string{
		"# 实验记录归档", "run-1", "设计：design-1 · 冻结 rev 3",
		"计划 vs 实际", "405nm 固化", "代表视野矩阵", "研判记录",
		"派生设计参数", "cure_s", "未签核",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("archive missing %q:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "@matrix") {
		t.Fatal("dynamic reference leaked into the archive")
	}
}

func TestBuildArchiveKeepsSignature(t *testing.T) {
	proj, prose := archiveFixture()
	signed, sig := record.Sign(prose, "run-1", proj.DesignRevision, "lin",
		time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	arch := BuildArchive(proj, signed, time.Date(2026, 11, 16, 13, 0, 0, 0, time.UTC))
	if !arch.Sealed || arch.Signature == nil || arch.Signature.Hash != sig.Hash {
		t.Fatalf("sealed = %v sig = %+v", arch.Sealed, arch.Signature)
	}
	doc := arch.Markdown()
	if !strings.Contains(doc, "lin") || strings.Contains(doc, "record-sign") {
		t.Fatal("signature block leaked or missing from the archive")
	}
}

// TestArchivePreviewStampIsStable pins the render-stable preview: a zero
// export time renders the same document on every frame with an explicit
// preview marker, and the stamp is applied only to real exports.
func TestArchivePreviewStampIsStable(t *testing.T) {
	proj, prose := archiveFixture()
	first := BuildArchive(proj, prose, time.Time{})
	second := BuildArchive(proj, prose, time.Time{})
	if first.Markdown() != second.Markdown() {
		t.Fatal("preview archive is not render-stable")
	}
	if !strings.Contains(first.Markdown(), "未导出（预览）") {
		t.Fatalf("preview stamp missing:\n%s", first.Markdown())
	}
	stamped := BuildArchive(proj, prose, time.Date(2026, 11, 16, 13, 0, 0, 0, time.UTC))
	if !strings.Contains(stamped.Markdown(), "2026-11-16T13:00:00Z") {
		t.Fatalf("export stamp missing:\n%s", stamped.Markdown())
	}
}

// TestArchiveHeaderCountsMatrixValuesOnly pins the truthful label: the header
// count covers matrix-cell values, and the label says so instead of claiming
// every measurement.
func TestArchiveHeaderCountsMatrixValuesOnly(t *testing.T) {
	proj, prose := archiveFixture()
	doc := BuildArchive(proj, prose, time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)).Markdown()
	if !strings.Contains(doc, "矩阵实测值") {
		t.Fatalf("header label is not narrow:\n%s", doc)
	}
	if strings.Contains(doc, "· 实测值 ") {
		t.Fatalf("broad measurement claim leaked:\n%s", doc)
	}
}

// pdfImageFixture builds a projection with two coordinated images placed in
// A1·6h FOV slots (Core coordinates only: WellID/Timepoint/FOVIndex, never
// filenames) plus a tiny valid PNG the loader serves for any placed image.
func pdfImageFixture(t *testing.T) (record.RecordProjection, string, ImageLoader) {
	t.Helper()
	snap := runs.RunSnapshot{
		RunID: "run-pdf", Title: "HUVEC", DesignID: "design-1", DesignRevision: 3,
		Status: "completed",
		Tasks: []runs.RunTask{
			{Key: "task-cure", Title: "cure", Timepoint: "0h", Status: "completed",
				ParametersJSON: `[{"key":"cure_s","label":"cure_s","value":60,"unit":"s"}]`},
		},
		Events: []runs.RunEvent{
			{ID: "evt-read", EventType: "measurement", TaskKey: "task-cure", Timepoint: "6h",
				BatchID: "batch-1", RecordedAt: "2026-11-10T16:01:00+08:00",
				ResultCells: []runs.ResultCell{
					{Row: 0, Column: 0, SampleID: "S1", Field: "OD450", Value: new(0.82)},
				}},
		},
	}
	assets := []runs.Asset{
		{ID: "img-a1", RootID: "root-1", RelativePath: "a1-fov1.png", MediaType: "image",
			Link: runs.AssetLink{RunID: "run-pdf", TaskKey: "task-cure", SampleID: "S1",
				WellID: "A1", Timepoint: "6h", FOVIndex: 1}},
		{ID: "img-a2", RootID: "root-1", RelativePath: "a1-fov2.png", MediaType: "image",
			Link: runs.AssetLink{RunID: "run-pdf", TaskKey: "task-cure", SampleID: "S1",
				WellID: "A1", Timepoint: "6h", FOVIndex: 2}},
	}
	proj := record.Project(snap, assets)
	if got := placedImageCount(proj); got != 2 {
		t.Fatalf("fixture placed %d images, want 2", got)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, img); err != nil {
		t.Fatal(err)
	}
	raw := pngBytes.Bytes()
	return proj, "# note\n\nsee @matrix(6h)\n", func(record.MatrixImage) ([]byte, error) {
		return raw, nil
	}
}

func placedImageCount(proj record.RecordProjection) int {
	n := 0
	for _, cell := range proj.Matrix {
		n += cell.FilledSlots()
	}
	return n
}

func pdfImageObjects(data []byte) int {
	return bytes.Count(data, []byte("/Subtype /Image"))
}

// TestArchivePDFDeterministic pins byte-stable output: the same archive
// (same ExportedAt) renders identical PDF bytes, so exports are comparable.
func TestArchivePDFDeterministic(t *testing.T) {
	proj, prose, load := pdfImageFixture(t)
	stamp := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	var a, b bytes.Buffer
	if err := BuildArchive(proj, prose, stamp).PDF(&a, load); err != nil {
		t.Fatal(err)
	}
	if err := BuildArchive(proj, prose, stamp).PDF(&b, load); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("same archive produced different PDF bytes")
	}
	if !bytes.HasPrefix(a.Bytes(), []byte("%PDF-")) {
		t.Fatal("output is not a PDF")
	}
}

// TestArchivePDFEmbedsImages parses the PDF instead of trusting file size:
// every placed matrix image must arrive as an embedded image object.
func TestArchivePDFEmbedsImages(t *testing.T) {
	proj, prose, load := pdfImageFixture(t)
	var buf bytes.Buffer
	arch := BuildArchive(proj, prose, time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	if err := arch.PDF(&buf, load); err != nil {
		t.Fatal(err)
	}
	if got := pdfImageObjects(buf.Bytes()); got != 2 {
		t.Fatalf("embedded image objects = %d, want 2", got)
	}
}

// TestArchivePDFMissingAssetNamesIt: an unavailable image aborts the export
// with the asset named — never a silent gap in a signed record.
func TestArchivePDFMissingAssetNamesIt(t *testing.T) {
	proj, prose, _ := pdfImageFixture(t)
	arch := BuildArchive(proj, prose, time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	err := arch.PDF(io.Discard, func(img record.MatrixImage) ([]byte, error) {
		if img.AssetID == "img-a2" {
			return nil, errors.New("file gone")
		}
		var pngBytes bytes.Buffer
		if err := png.Encode(&pngBytes, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
			t.Fatal(err)
		}
		return pngBytes.Bytes(), nil
	})
	if err == nil {
		t.Fatal("missing image did not fail the export")
	}
	if !strings.Contains(err.Error(), "img-a2") || !strings.Contains(err.Error(), "A1") {
		t.Fatalf("error does not name the asset: %v", err)
	}
}

// TestArchivePDFNilLoaderWithImagesFails: building the board without any way
// to load files is a wiring bug, and it must surface with the asset named.
func TestArchivePDFNilLoaderWithImagesFails(t *testing.T) {
	proj, prose, _ := pdfImageFixture(t)
	arch := BuildArchive(proj, prose, time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	err := arch.PDF(io.Discard, nil)
	if err == nil || !strings.Contains(err.Error(), "img-a1") {
		t.Fatalf("nil loader did not name the first asset: %v", err)
	}
}
