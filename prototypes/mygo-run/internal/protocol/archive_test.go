package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/modules/runs"
)

func archiveProjFixture() (record.Projection, []record.TaskStatus) {
	proj := record.Projection{
		SchemaVersion: 1,
		RunID:         "run-1",
		Design: record.DesignSummary{
			DesignID: "design-1", DesignRevision: 3, Title: "HUVEC 成管",
		},
		Parameters: []record.ParamCompare{
			{ScopeID: "root/cure", Slot: "cure_s", Label: new("固化秒数"), Unit: new("s"),
				Planned: json.RawMessage(`60`),
				Actual:  []record.ActualParam{{Value: json.RawMessage(`75`), Unit: new("s"), EventID: "e1"}},
				Status:  "differs"},
		},
		Images: record.ImageMatrix{
			FOVPerWell: 1, PlannedSlots: 1, FilledSlots: 1,
			Plates: []record.ImagePlateRow{{
				PlateID: "p1", Timepoints: []*string{new("6h")},
				PlannedSlots: 1, FilledSlots: 1,
				Wells: []record.ImageWellRow{{
					Coordinate: "A1", GroupLabel: new("3%"), PlannedSlots: 1, FilledSlots: 1,
					Slots: []record.ImageSlot{{Timepoint: new("6h"), FOVIndex: 1, AssetID: new("img-1")}},
				}},
			}},
		},
		Assets:   []record.RecordAsset{{AssetID: "img-1", TaskKey: "task-cure"}},
		Timeline: record.TimelineSummary{SkippedByRule: 0},
		Notes:    []record.NoteView{{NoteID: "n1", Markdown: "# 研判\n\n结论见对比 @matrix(6h)\n"}},
		Archive:  record.ArchiveState{Status: "recording"},
	}
	tasks := record.SnapshotTasks([]runs.RunTask{
		{Key: "task-cure", Title: "405nm 固化", Timepoint: "0h", Status: "completed"},
	})
	return proj, tasks
}

func TestBuildArchiveFlattensRefs(t *testing.T) {
	proj, tasks := archiveProjFixture()
	prose := "# 研判\n\n结论见对比 @matrix(6h)\n"
	arch := BuildArchive(proj, tasks, prose, time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	if len(arch.SourceRefs) != 1 || arch.SourceRefs[0].Raw != "@matrix(6h)" {
		t.Fatalf("source refs = %+v", arch.SourceRefs)
	}
	doc := arch.Markdown()
	for _, want := range []string{
		"# 实验记录归档", "run-1", "设计：", "冻结 rev 3",
		"计划 vs 实际", "405nm 固化", "代表视野矩阵", "研判记录",
		"签核清单", "未签核",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("archive missing %q:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "@matrix") {
		t.Fatal("dynamic reference leaked into the archive")
	}
}

func TestArchiveShowsSignature(t *testing.T) {
	proj, tasks := archiveProjFixture()
	proj.Archive.Status = "archived"
	proj.Archive.Signature = &record.ArchiveSignature{SignedBy: "lin", SignedAtUTC: "2026-11-16T12:00:00Z"}
	doc := BuildArchive(proj, tasks, "# 研判\n", time.Date(2026, 11, 16, 13, 0, 0, 0, time.UTC)).Markdown()
	if !strings.Contains(doc, "- 签核：lin") || strings.Contains(doc, "- 签核：未签核") {
		t.Fatalf("signature block missing:\n%s", doc)
	}
}

func TestArchiveStripsLegacyTrailer(t *testing.T) {
	proj, tasks := archiveProjFixture()
	trailer := "<!-- record-sign run=run-1 rev=3 hash=" + strings.Repeat("a", 64) + " at=2026-11-16T12:00:00Z actor=lin -->"
	doc := BuildArchive(proj, tasks, "# 研判\n\n贴壁良好\n"+trailer, time.Now()).Markdown()
	if strings.Contains(doc, "record-sign") {
		t.Fatalf("legacy trailer leaked into the archive:\n%s", doc)
	}
}

// TestArchivePreviewStampIsStable pins the render-stable preview: a zero
// export time renders the same document on every frame with an explicit
// preview marker, and the stamp is applied only to real exports.
func TestArchivePreviewStampIsStable(t *testing.T) {
	proj, tasks := archiveProjFixture()
	first := BuildArchive(proj, tasks, "# 研判\n", time.Time{})
	second := BuildArchive(proj, tasks, "# 研判\n", time.Time{})
	if first.Markdown() != second.Markdown() {
		t.Fatal("preview archive is not render-stable")
	}
	if !strings.Contains(first.Markdown(), "未导出（预览）") {
		t.Fatalf("preview stamp missing:\n%s", first.Markdown())
	}
	stamped := BuildArchive(proj, tasks, "# 研判\n", time.Date(2026, 11, 16, 13, 0, 0, 0, time.UTC))
	if !strings.Contains(stamped.Markdown(), "2026-11-16T13:00:00Z") {
		t.Fatalf("export stamp missing:\n%s", stamped.Markdown())
	}
}

func TestArchiveCountsShared(t *testing.T) {
	proj, tasks := archiveProjFixture()
	arch := BuildArchive(proj, tasks, "# 研判\n", time.Now())
	if arch.Counts() != record.Summarize(proj, tasks) {
		t.Fatal("archive counts diverged from the shared counting")
	}
	if !strings.Contains(arch.Markdown(), record.Headline(arch.Counts())) {
		t.Fatal("archive header does not carry the shared headline")
	}
}

// pngLoader serves one tiny valid PNG for any placed image.
func pngLoader() (ImageLoader, []byte) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	raw := buf.Bytes()
	return func(record.PlacedImage) ([]byte, error) { return raw, nil }, raw
}

// pdfImageFixture builds a projection with two filled FOV slots at A1·6h.
func pdfImageFixture(t *testing.T) (record.Projection, []record.TaskStatus, string, ImageLoader) {
	t.Helper()
	proj, tasks := archiveProjFixture()
	row := &proj.Images.Plates[0].Wells[0]
	row.Slots = append(row.Slots, record.ImageSlot{Timepoint: new("6h"), FOVIndex: 2, AssetID: new("img-2")})
	row.PlannedSlots = 2
	row.FilledSlots = 2
	proj.Images.PlannedSlots = 2
	proj.Images.FilledSlots = 2
	proj.Images.Plates[0].PlannedSlots = 2
	proj.Images.Plates[0].FilledSlots = 2
	proj.Assets = append(proj.Assets, record.RecordAsset{AssetID: "img-2", TaskKey: "task-cure"})
	load, _ := pngLoader()
	return proj, tasks, "# note\n\nsee @matrix(6h)\n", load
}

func placedImageCount(proj record.Projection) int {
	n := 0
	for _, cell := range record.MatrixCells(proj) {
		n += cell.Filled
	}
	return n
}

func pdfImageObjects(data []byte) int {
	return bytes.Count(data, []byte("/Subtype /Image"))
}

func TestArchivePDFDeterministic(t *testing.T) {
	proj, tasks, prose, load := pdfImageFixture(t)
	stamp := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	var a, b bytes.Buffer
	if err := BuildArchive(proj, tasks, prose, stamp).PDF(&a, load); err != nil {
		t.Fatal(err)
	}
	if err := BuildArchive(proj, tasks, prose, stamp).PDF(&b, load); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("same archive produced different PDF bytes")
	}
	if !bytes.HasPrefix(a.Bytes(), []byte("%PDF-")) {
		t.Fatal("output is not a PDF")
	}
}

func TestArchivePDFEmbedsImages(t *testing.T) {
	proj, tasks, prose, load := pdfImageFixture(t)
	if got := placedImageCount(proj); got != 2 {
		t.Fatalf("fixture placed %d images, want 2", got)
	}
	var buf bytes.Buffer
	arch := BuildArchive(proj, tasks, prose, time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
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
	proj, tasks, prose, _ := pdfImageFixture(t)
	arch := BuildArchive(proj, tasks, prose, time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	err := arch.PDF(io.Discard, func(img record.PlacedImage) ([]byte, error) {
		if img.AssetID == "img-1" {
			return nil, errors.New("file gone")
		}
		_, raw := pngLoader()
		return raw, nil
	})
	if err == nil || !strings.Contains(err.Error(), "img-1") {
		t.Fatalf("missing asset must abort naming it, got %v", err)
	}
}

// TestArchivePDFSkipsUnparsableFontCandidate pins the TTC-poisoning case: a
// readable-but-unusable candidate (a .ttc collection, corrupt .ttf bytes)
// must be skipped, not registered — gofpdf reports the damage only later as
// a sticky 'undefined font'. The export still succeeds on Helvetica.
func TestArchivePDFSkipsUnparsableFontCandidate(t *testing.T) {
	dir := t.TempDir()
	ttc := filepath.Join(dir, "fake.ttc")
	if err := os.WriteFile(ttc, []byte("ttcf\x00\x01\x00\x00garbage"), 0644); err != nil {
		t.Fatal(err)
	}
	corrupt := filepath.Join(dir, "corrupt.ttf")
	if err := os.WriteFile(corrupt, []byte("not a font at all"), 0644); err != nil {
		t.Fatal(err)
	}
	old := recordPDFFontCandidates
	recordPDFFontCandidates = []string{ttc, corrupt}
	defer func() { recordPDFFontCandidates = old }()
	proj, tasks, prose, load := pdfImageFixture(t)
	var buf bytes.Buffer
	arch := BuildArchive(proj, tasks, prose, time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	if err := arch.PDF(&buf, load); err != nil {
		t.Fatalf("export with unusable font candidates failed: %v", err)
	}
	if got := pdfImageObjects(buf.Bytes()); got != 2 {
		t.Fatalf("export embedded %d image objects, want 2", got)
	}
}

// bundleManifest builds a valid manifest over one note plus one asset,
// with the canonical digest Core's verifier recomputes.
func bundleManifest(t *testing.T, noteData, assetData []byte) string {
	t.Helper()
	noteSum := sha256.Sum256(noteData)
	assetSum := sha256.Sum256(assetData)
	entries := []record.ManifestEntry{
		{Kind: "note", LogicalPath: "notes/n1.md", SHA256: hex.EncodeToString(noteSum[:]), Size: new(uint64(len(noteData)))},
		{Kind: "asset", LogicalPath: "run-1/a1.png", SHA256: hex.EncodeToString(assetSum[:]), Size: new(uint64(len(assetData)))},
	}
	paths := []string{entries[0].LogicalPath, entries[1].LogicalPath}
	sort.Strings(paths)
	byPath := map[string]record.ManifestEntry{entries[0].LogicalPath: entries[0], entries[1].LogicalPath: entries[1]}
	hasher := sha256.New()
	for _, p := range paths {
		fmt.Fprintf(hasher, "%s  %s\n", strings.ToLower(byPath[p].SHA256), p)
	}
	manifest := record.Manifest{
		SchemaVersion: 1, RunID: "run-1", DesignVersionID: "dv-3",
		GeneratedAtUTC: "2026-11-17T12:00:00Z", Entries: entries,
		ManifestSHA256: hex.EncodeToString(hasher.Sum(nil)),
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestWriteBundleVerifiesEveryMember(t *testing.T) {
	proj, tasks := archiveProjFixture()
	noteData := []byte(proj.Notes[0].Markdown)
	_, assetData := pngLoader()
	manifestJSON := bundleManifest(t, noteData, assetData)
	arch := BuildArchive(proj, tasks, "# 研判\n\n结论见对比 @matrix(6h)\n", time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	load, _ := pngLoader()
	var buf bytes.Buffer
	if err := arch.WriteBundle(&buf, manifestJSON, load, func(logicalPath string) ([]byte, error) {
		if logicalPath == "run-1/a1.png" {
			return assetData, nil
		}
		return nil, fmt.Errorf("unknown member %q", logicalPath)
	}); err != nil {
		t.Fatalf("bundle: %v", err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("PK")) {
		t.Fatal("bundle is not a zip")
	}
	// A tampered member aborts loudly instead of shipping a fake package.
	var bad bytes.Buffer
	if err := arch.WriteBundle(&bad, manifestJSON, load, func(string) ([]byte, error) {
		return []byte("tampered"), nil
	}); err == nil || !strings.Contains(err.Error(), "run-1/a1.png") {
		t.Fatalf("tampered member must abort naming it, got %v", err)
	}
}
