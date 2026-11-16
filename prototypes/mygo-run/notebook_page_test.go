package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
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
					{Row: 0, Column: 0, SampleID: "S1", Field: "OD450", Value: new(0.82)},
					{Row: 0, Column: 1, SampleID: "S2", Field: "OD450", Value: new(0.91)},
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
		"记录图板", "计划 vs 实际", "任务完成 1/1", "代表视野矩阵",
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

func TestNotebookSignLocksAndUnlockRestores(t *testing.T) {
	snap, assets := boardTestSnap("run-seal-1")
	svc := &sealStubSvc{snap: snap}
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
