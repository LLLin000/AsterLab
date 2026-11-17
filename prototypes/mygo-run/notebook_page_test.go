package main

import (
	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/egoist/mygo/ui"
	"golang.org/x/image/tiff"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// sealStubSvc serves Core's record lifecycle in memory: the projection is a
// fixed record.Projection (CLOSE-04 split the packages, so pages never build
// one from the snapshot), and SignAndArchiveRun flips its persisted archive
// state exactly once, like Core.
type sealStubSvc struct {
	runs.RunService
	snap runs.RunSnapshot
	proj record.Projection
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

func (s *sealStubSvc) RecordProjection(_ context.Context, _ runs.RecordProjectionInput) (string, error) {
	p := s.proj
	if p.SchemaVersion == 0 {
		p.SchemaVersion = 1
	}
	p.RunID = s.snap.RunID
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (s *sealStubSvc) SignAndArchiveRun(_ context.Context, in runs.SignAndArchiveInput) (runs.ArchiveState, error) {
	switch s.proj.Archive.Status {
	case "signed", "archived":
		return runs.ArchiveState{}, errors.New("record already sealed")
	}
	s.proj.Archive.Status = "signed"
	s.proj.Archive.Signature = &record.ArchiveSignature{SignedBy: in.SignedBy, SignedAtUTC: in.SignedAtUTC}
	return runs.ArchiveState{RunID: s.snap.RunID, Status: "signed", SignedBy: in.SignedBy, SignedAtUTC: in.SignedAtUTC}, nil
}

// boardTestProj is the canned Core projection the board tests project from:
// two assay groups plus an ungrouped well, planned-vs-actual parameters, one
// valued reading, and a Well x Timepoint x FOV matrix. Slot placement is
// Core's answer; tests only choose which asset ID fills the FOV-3 slot.
func boardTestProj(fov3Asset string) record.Projection {
	str := func(s string) *string { return &s }
	u64 := func(v uint64) *uint64 { return &v }
	f64 := func(v float64) *float64 { return &v }
	eventType := func(s string) *string { return &s }
	fov3 := str(fov3Asset)
	if strings.TrimSpace(fov3Asset) == "" {
		fov3 = nil
	}
	return record.Projection{
		SchemaVersion: 1,
		Design: record.DesignSummary{
			DesignID: "design-1", DesignRevision: 3, Title: "HUVEC 成管",
			Groups: []record.Group{{Name: "3%"}, {Name: "5%"}},
			Assays: []record.Assay{{AssayID: "assay-huvec", Title: "成管", Endpoint: "OD450", Timepoints: []string{"0h", "6h"}}},
		},
		Parameters: []record.ParamCompare{
			{ScopeID: "task-cure", StepPath: str("task-cure"), Slot: "cure_s", Label: str("固化秒数"), Unit: str("s"),
				Planned: json.RawMessage(`60`), Actual: []record.ActualParam{{Value: json.RawMessage(`75`), Unit: str("s"), EventID: "evt-cure"}}, Status: "differs"},
			{ScopeID: "task-cure", Slot: "stray", Planned: json.RawMessage(`1`), Status: "unplanned"},
		},
		Timers: []record.TimerObserve{
			{TimerID: "t1", TaskKey: "task-cure", Mode: "countdown", Status: "completed", PlannedS: u64(60), ElapsedS: u64(75)},
		},
		Images: record.ImageMatrix{
			FOVPerWell: 1, PlannedSlots: 9, FilledSlots: 2, MissingSlots: 7,
			Plates: []record.ImagePlateRow{{
				PlateID: "p1", Timepoints: []*string{str("6h")},
				PlannedSlots: 9, FilledSlots: 2,
				Wells: []record.ImageWellRow{
					{Coordinate: "A1", GroupLabel: str("3%"), PlannedSlots: 3, FilledSlots: 2,
						Slots: []record.ImageSlot{
							{Timepoint: str("6h"), FOVIndex: 1, AssetID: str("img-1")},
							{Timepoint: str("6h"), FOVIndex: 2, AssetID: str("img-2")},
							{Timepoint: str("6h"), FOVIndex: 3, AssetID: fov3},
						}},
					{Coordinate: "A2", GroupLabel: str("5%"), PlannedSlots: 3,
						Slots: []record.ImageSlot{
							{Timepoint: str("6h"), FOVIndex: 1},
							{Timepoint: str("6h"), FOVIndex: 2},
							{Timepoint: str("6h"), FOVIndex: 3},
						}},
					{Coordinate: "B1", PlannedSlots: 3,
						Slots: []record.ImageSlot{
							{Timepoint: str("6h"), FOVIndex: 1},
							{Timepoint: str("6h"), FOVIndex: 2},
							{Timepoint: str("6h"), FOVIndex: 3},
						}},
				},
			}},
		},
		Assets: []record.RecordAsset{
			{AssetID: "img-1", TaskKey: "task-cure", WellID: str("A1"), Timepoint: str("6h")},
			{AssetID: "img-2", TaskKey: "task-cure", WellID: str("A1"), Timepoint: str("6h")},
		},
		Readings: []record.Reading{
			{RunID: "run-board", StepPath: "task-cure", Endpoint: "OD450", Value: f64(0.82), Unit: str("AU"), SourceEventID: "evt-read"},
		},
		Timeline: record.TimelineSummary{
			Events: []record.TimelineEvent{
				{EventID: "evt-cure", EventType: eventType("operation"), TaskKey: str("task-cure"), Content: str("固化"), Deviation: str("流出液未完全弃置")},
			},
		},
		Notes:   []record.NoteView{{NoteID: "n1", Markdown: "# 研判\n\n贴壁良好 @matrix(6h)\n"}},
		Archive: record.ArchiveState{Status: "recording"},
		Issues:  []record.Issue{{Code: "timer_watermark_missing", Message: "缺水印"}},
	}
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
	svc := &sealStubSvc{snap: snap, proj: boardTestProj("")}
	a := &app{snap: snap, runAssets: assets, runSvc: svc}
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
		"记录图板", "计划 vs 实际", "任务完成 1/1", "代表视野矩阵", "代表视野（分组",
		"3%", "5%", "未分组", "6h",
		"固化秒数", "FOV 1", "FOV 6", "缺", "派生设计", "签核归档", "@matrix(6h)",
	} {
		if !tt.HasText(want) {
			t.Fatalf("board missing %q: %q", want, tt.Texts())
		}
	}
	// FOV 1 shows the coordinated image; switching layers follows the slot.
	if !tt.HasText("图·img-1") {
		t.Fatalf("FOV 1 representative missing: %q", tt.Texts())
	}
	if err := tt.Click("FOV 2"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("图·img-2") {
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
	svc := &sealStubSvc{snap: snap, proj: boardTestProj("img-real")}
	a := &app{snap: snap, runAssets: assets, assetRoots: map[string]string{"root-1": dir}, runSvc: svc}
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

// TestNotebookSignLocksIntoReadingMode pins the sign flow: sign/archive is
// one Core command behind an explicit confirmation naming the 缺/计划外
// counts, and the sealed record is final — reading mode, no unlock.
func TestNotebookSignLocksIntoReadingMode(t *testing.T) {
	snap, assets := boardTestSnap("run-seal-1")
	svc := &sealStubSvc{snap: snap, proj: boardTestProj("")}
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
	if !tt.HasText("确认签核归档") {
		t.Fatalf("sign confirmation missing: %q", tt.Texts())
	}
	if !tt.HasText("实测 1 个") || !tt.HasText("排除 1 个") {
		t.Fatalf("confirmation does not name the counts: %q", tt.Texts())
	}
	if err := tt.Click("确认签核归档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.noteMode != 2 {
		t.Fatalf("signed record did not move to reading mode: %d", a.noteMode)
	}
	if !tt.HasText("已签核归档") {
		t.Fatalf("sealed state word missing: %q", tt.Texts())
	}
	if tt.HasText("解锁") {
		t.Fatalf("sealed record offers an unlock: %q", tt.Texts())
	}
	proj, _, _ := a.recordProjection()
	if proj.Archive.Status != "signed" {
		t.Fatalf("Core archive state is not signed: %q", proj.Archive.Status)
	}
	if proj.Archive.Signature == nil || strings.TrimSpace(proj.Archive.Signature.SignedBy) == "" {
		t.Fatalf("signed record carries no signature: %+v", proj.Archive)
	}
	if !a.recordSealed() {
		t.Fatal("sealed record does not report sealed")
	}
	// A sealed record refuses new saves instead of dropping prose.
	a.noteMode = 0
	a.noteDraft = "# 改写\n\n签核后改写"
	a.saveNotebook()
	if a.noteErr != "记录已签核归档，笔记已锁定" {
		t.Fatalf("sealed save was not refused: %q", a.noteErr)
	}
}

// TestNotebookSealedRecordNeverExposesEditor pins the sealed-mode transitions:
// a signed Run opens in reading mode, and even a forced live mode shows the
// lock instead of the editor, so the prose is never editable and the sign
// button never offers a second sign.
func TestNotebookSealedRecordNeverExposesEditor(t *testing.T) {
	snap, assets := boardTestSnap("run-seal-mode-1")
	proj := boardTestProj("")
	proj.Archive.Status = "signed"
	proj.Archive.Signature = &record.ArchiveSignature{SignedBy: "lin", SignedAtUTC: "2026-11-16T12:00:00Z"}
	svc := &sealStubSvc{snap: snap, proj: proj}
	a := &app{snap: snap, runAssets: assets, runSvc: svc}
	// Warm the shared memo so the sealed Core state is known before the
	// first frame, the way visiting the board first would.
	if _, _, fetchErr := a.recordProjection(); strings.TrimSpace(fetchErr) != "" {
		t.Fatalf("record fetch failed: %s", fetchErr)
	}
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
	if !tt.HasText("已签核归档") || !tt.HasText("记录已锁定为只读") {
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
	svc := &sealStubSvc{snap: snap, proj: boardTestProj("img-tif")}
	a := &app{snap: snap, runAssets: assets, assetRoots: map[string]string{"root-1": dir}, runSvc: svc}
	proj, _, fetchErr := a.recordProjection()
	if strings.TrimSpace(fetchErr) != "" {
		t.Fatalf("record fetch failed: %s", fetchErr)
	}
	tifImg := record.SlotImage(proj, "A1", "6h", 3)
	if tifImg == nil || tifImg.AssetID != "img-tif" {
		t.Fatalf("tiff asset not placed in the A1/6h/FOV-3 slot: %+v", tifImg)
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
