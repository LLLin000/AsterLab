package main

import (
	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
	"bytes"
	"context"
	"github.com/egoist/mygo/ui"
	"golang.org/x/image/tiff"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotebookPageLoadsLatestMarkdown(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{
		RunID: "run-1", Title: "CCK-8",
		Events: []runs.RunEvent{
			{ID: "older", Stage: "notebook", Content: "# 旧记录", RecordedAt: "2026-11-10T09:00:00Z"},
			{ID: "latest", Stage: "notebook", Content: "# 今日观察\n\n- 细胞贴壁良好", RecordedAt: "2026-11-10T10:00:00Z"},
		},
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if !tt.HasText("Notebook") || !tt.HasText("实时") || !tt.HasText("编辑") || !tt.HasText("阅读") || !tt.HasText("文档") {
		t.Fatalf("notebook page is missing its toolbar or modes: %q", tt.Texts())
	}
	if a.noteDoc == nil || a.noteDoc.Text().Markdown() != "# 今日观察\n\n- 细胞贴壁良好" {
		t.Fatalf("live document did not load the latest revision: %q", a.noteDoc.Text().Markdown())
	}
	if err := tt.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.noteMode != 1 || a.noteDraft != "# 今日观察\n\n- 细胞贴壁良好" {
		t.Fatalf("source mode did not carry the latest revision: mode=%d draft=%q", a.noteMode, a.noteDraft)
	}
	if err := tt.Click("阅读"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("今日观察") {
		t.Fatalf("reading mode did not render the document: %q", tt.Texts())
	}
	if a.noteDraft != "# 今日观察\n\n- 细胞贴壁良好" || a.noteDirty {
		t.Fatalf("loaded draft = %q dirty=%v", a.noteDraft, a.noteDirty)
	}
}

func TestNotebookAsterDocModeRendersPlateModule(t *testing.T) {
	source := "# D3\n\n```asterlab\nkind: group\nid: g-control\nschema: 1\nname: Control\nrole: control\n```\n\n```asterlab\nkind: plate\nid: plate-d3\nschema: 1\nformat: 96\ngroups:\n  g-control: [A1]\n```\n"
	a := &app{snap: runs.RunSnapshot{
		RunID: "run-1", Title: "CCK-8",
		Events: []runs.RunEvent{{ID: "note", Stage: "notebook", Content: source, RecordedAt: "2026-11-10T10:00:00Z"}},
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("文档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.noteMode != 3 {
		t.Fatalf("document mode not selected: %d", a.noteMode)
	}
	if !tt.HasText("96 孔板") || !tt.HasText("plate-d3") {
		t.Fatalf("asterlab module did not render: %q", tt.Texts())
	}
}

func TestNotebookDocumentModeCanInsertPlateModule(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{RunID: "run-1", Title: "CCK-8"}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("文档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("插入孔板模块"); err != nil {
		t.Fatalf("insert plate action: %v", err)
	}
	tt.Frame()
	tt.Frame()
	if !strings.Contains(a.noteDraft, "kind: plate") {
		t.Fatalf("plate module was not inserted: %q", a.noteDraft)
	}
	if !tt.HasText("96 孔板") {
		t.Fatalf("inserted module did not render: %q", tt.Texts())
	}
}

// sealStubSvc persists Notebook prose in memory: the sign path is the same
// SaveNotebook call the GUI uses, with no native service involved.
type sealStubSvc struct {
	runs.RunService
	snap runs.RunSnapshot
}

func (s *sealStubSvc) SaveNotebook(_ context.Context, in runs.NotebookInput) (runs.RunSnapshot, error) {
	snap := s.snap
	snap.Events = append(append([]runs.RunEvent{}, snap.Events...), runs.RunEvent{
		ID: "evt-note-test", EventType: "observation", Stage: "notebook",
		Content: in.Markdown, RecordedAt: "2026-11-16T12:00:00Z",
	})
	snap.Revision++
	s.snap = snap
	return snap, nil
}

func boardTestSnap(runID string) (runs.RunSnapshot, []runs.Asset) {
	snap := runs.RunSnapshot{
		RunID: runID, Title: "HUVEC 成管",
		DesignID: "design-1", DesignRevision: 3, Status: "in_progress",
		Tasks: []runs.RunTask{
			{
				Key: "task-cure", Title: "405nm 固化", Timepoint: "0h",
				Status: "completed", DesignStepID: "step-2",
				ParametersJSON: `[{"key":"cure_s","label":"固化秒数","value":60,"unit":"s"}]`,
			},
		},
		Events: []runs.RunEvent{
			{
				ID: "evt-cure", EventType: "operation", TaskKey: "task-cure", Timepoint: "0h",
				OccurredAt: "2026-11-10T10:00:00+08:00", RecordedAt: "2026-11-10T10:01:00+08:00",
				DataJSON: `{"actual_parameters":[{"key":"cure_s","value":75,"unit":"s"}]}`,
			},
			{
				ID: "evt-read", EventType: "measurement", TaskKey: "task-cure", Timepoint: "6h",
				BatchID: "batch-1", OccurredAt: "2026-11-10T16:00:00+08:00",
				RecordedAt: "2026-11-10T16:01:00+08:00",
				ResultCells: []runs.ResultCell{
					{Row: 0, Column: 0, SampleID: "S1", Group: "3%", Field: "OD450", Value: new(0.82)},
					{Row: 0, Column: 1, SampleID: "S2", Group: "5%", Field: "OD450", Value: new(0.91)},
					{Row: 0, Column: 2, SampleID: "S3", Field: "OD450", Value: new(0.77)},
				},
			},
			{ID: "note-1", Stage: "notebook", Content: "# 研判\n\n贴壁良好 @matrix(6h)", RecordedAt: "2026-11-10T17:00:00Z"},
		},
	}
	assets := []runs.Asset{
		{ID: "img-1", RelativePath: "run/6h/S1-fov1.png", MediaType: "image",
			Link: runs.AssetLink{RunID: runID, TaskKey: "task-cure", SampleID: "S1", WellID: "A1", Timepoint: "6h", FOVIndex: 1}},
		{ID: "img-2", RelativePath: "run/6h/S1-fov2.png", MediaType: "image",
			Link: runs.AssetLink{RunID: runID, TaskKey: "task-cure", SampleID: "S1", WellID: "A1", Timepoint: "6h", FOVIndex: 2}},
	}
	return snap, assets
}

func TestNotebookBoardRendersSharedProjection(t *testing.T) {
	snap, assets := boardTestSnap("run-board-1")
	a := &app{snap: snap, runAssets: assets}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("图板"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{
		"记录图板", "计划 vs 实际", "任务完成 1/1", "矩阵实测值", "代表视野（分组",
		"代表视野矩阵", "3%", "5%", "未分组", "6h",
		"固化秒数", "FOV 1", "FOV 6", "缺", "派生设计", "签核归档", "@matrix(6h)",
	} {
		if !tt.HasText(want) {
			t.Fatalf("board missing %q: %q", want, tt.Texts())
		}
	}
	// FOV 1 shows the coordinated image; switching layers follows the slot.
	if !tt.HasText("S1-fov1.png") {
		t.Fatalf("FOV 1 representative missing: %q", tt.Texts())
	}
	if err := tt.Click("FOV 2"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("S1-fov2.png") {
		t.Fatalf("FOV 2 representative missing: %q", tt.Texts())
	}
}

// TestNotebookGroupBoardDecodesPreviews pins the inline image path: a
// coordinated asset whose file exists decodes into the board cache and
// renders in the group matrix, while a missing file stays an explicit gap
// instead of failing the frame.
func TestNotebookGroupBoardDecodesPreviews(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "fov1.png")
	file, err := os.Create(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	snap, assets := boardTestSnap("run-board-img")
	assets = append(assets, runs.Asset{
		ID: "img-real", RootID: "root-1", RelativePath: "fov1.png", MediaType: "image",
		Link: runs.AssetLink{RunID: "run-board-img", WellID: "A1", Timepoint: "6h", FOVIndex: 3},
	})
	a := &app{snap: snap, runAssets: assets, assetRoots: map[string]string{"root-1": dir}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("图板"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The 3% x 6h slot shows A1; with FOV 3 selected it previews the real file.
	if err := tt.Click("FOV 3"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if src := a.recordBoard().imgs["img-real"]; src == nil {
		t.Fatalf("real image was not decoded into the board cache: %q", tt.Texts())
	}
	if a.recordBoard().imgFailed["img-real"] {
		t.Fatal("decodable image marked failed")
	}
	// The fixture images have no roots configured: explicit gap text, no crash.
	if a.recordBoard().imgFailed["img-1"] && a.recordBoard().imgs["img-1"] != nil {
		t.Fatal("missing file produced a preview")
	}
}

func TestNotebookSignLocksAndUnlockRestores(t *testing.T) {
	snap, assets := boardTestSnap("run-seal-1")
	svc := &sealStubSvc{snap: snap}
	a := &app{snap: snap, runAssets: assets, runSvc: svc}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
		// The board is a scrolling surface; the taller window keeps the
		// sign-off controls in reach without changing what is asserted.
	}, 1200, 1050)
	tt.Frame()
	if err := tt.Click("图板"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("签核归档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.noteMode != 2 {
		t.Fatalf("signed record did not move to reading mode: %d", a.noteMode)
	}
	if !tt.HasText("已签核归档") {
		t.Fatalf("sealed state word missing: %q", tt.Texts())
	}
	if _, ok := record.Verify(a.noteSource()); !ok {
		t.Fatal("signed prose does not verify")
	}
	// The sealed record still reaches the board, where 解锁 lives.
	if err := tt.Click("图板"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("解锁") {
		t.Fatalf("unlock missing on sealed board: %q", tt.Texts())
	}
	if err := tt.Click("解锁"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.noteDirty || !tt.HasText("未保存") {
		t.Fatalf("unlock did not return to editable state: dirty=%v %q", a.noteDirty, tt.Texts())
	}
	if _, ok := record.Verify(a.noteSource()); ok {
		t.Fatal("trailer survived unlock")
	}
}

// TestNotebookSealedRecordNeverExposesEditor pins the sealed-mode transitions:
// a signed Run opens in reading mode, and even a forced live mode shows the
// lock instead of the editor, so the signature trailer is never editable and
// never duplicated by a second sign.
func TestNotebookSealedRecordNeverExposesEditor(t *testing.T) {
	signed, _ := record.Sign("# 研判\n\n贴壁良好", "run-seal-mode-1", 3, "lin",
		time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	snap := runs.RunSnapshot{
		RunID: "run-seal-mode-1", Title: "HUVEC 成管",
		DesignID: "design-1", DesignRevision: 3,
		Events: []runs.RunEvent{
			{ID: "note-1", Stage: "notebook", Content: signed, RecordedAt: "2026-11-10T17:00:00Z"},
		},
	}
	a := &app{snap: snap}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if a.noteMode != 2 {
		t.Fatalf("sealed record did not open in reading mode: %d", a.noteMode)
	}
	if !tt.HasText("已签核归档") {
		t.Fatalf("sealed state word missing: %q", tt.Texts())
	}
	// Even if live mode is forced, the prose guard shows the lock instead of
	// the editor, and the trailer stays out of every rendered view.
	a.noteMode = 0
	tt.Frame()
	if !tt.HasText("已签核归档") || !tt.HasText("解锁") {
		t.Fatalf("sealed live mode did not lock: %q", tt.Texts())
	}
	if tt.HasText("record-sign") {
		t.Fatalf("signature trailer leaked into a rendered view: %q", tt.Texts())
	}
}

// TestNotebookGroupBoardDecodesTiffPreview pins TIFF support on the record
// preview path: a real .tif asset decodes into the board cache through
// recordImageSource instead of landing in the unreadable bucket.
func TestNotebookGroupBoardDecodesTiffPreview(t *testing.T) {
	dir := t.TempDir()
	file, err := os.Create(filepath.Join(dir, "fov3.tif"))
	if err != nil {
		t.Fatal(err)
	}
	if err := tiff.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	snap, assets := boardTestSnap("run-board-tif")
	assets = append(assets, runs.Asset{
		ID: "img-tif", RootID: "root-1", RelativePath: "fov3.tif", MediaType: "image/tiff",
		Link: runs.AssetLink{RunID: "run-board-tif", TaskKey: "task-cure", SampleID: "S1", WellID: "A1", Timepoint: "6h", FOVIndex: 3},
	})
	a := &app{snap: snap, runAssets: assets, assetRoots: map[string]string{"root-1": dir}}
	proj := a.recordProjection()
	var tifImg *record.MatrixImage
	for i := range proj.Matrix {
		if img := proj.Matrix[i].Representative(3); img != nil && img.AssetID == "img-tif" {
			tifImg = img
		}
	}
	if tifImg == nil {
		t.Fatalf("tiff asset not placed in any cell: %+v", proj.Matrix)
	}
	board := a.recordBoard()
	if src := a.recordImageSource(board, *tifImg); src == nil {
		t.Fatal("valid TIFF preview did not decode")
	}
	if board.imgFailed["img-tif"] {
		t.Fatal("decodable TIFF marked unreadable")
	}
	if board.imgs["img-tif"] == nil {
		t.Fatal("decoded TIFF missing from the board cache")
	}
}

// TestNotebookPreviewDropsStaleLayerResults pins the stale guard: a worker
// finishing after the board moved to another Run or FOV layer must neither
// plant its bitmap in the active layer nor mark it failed.
func TestNotebookPreviewDropsStaleLayerResults(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	src, err := decodePreviewImage(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := boardTestSnap("run-board-stale")
	a := &app{snap: snap}
	board := a.recordBoard()
	a.finishPreviewLoad(board, "run-board-stale", board.gen+1, board.fov, "img-stale", src, nil)
	if board.imgs["img-stale"] != nil || board.imgFailed["img-stale"] {
		t.Fatal("stale completion reached the active layer")
	}
	a.finishPreviewLoad(board, "run-board-stale", board.gen, board.fov, "img-stale", src, nil)
	if board.imgs["img-stale"] == nil {
		t.Fatal("current-layer completion did not land in the cache")
	}
}
