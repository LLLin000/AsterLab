package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
)

func TestNotebookPageLoadsLatestMarkdownThreeModes(t *testing.T) {
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
	for _, want := range []string{"编辑", "阅读", "源码", "导出 PDF", "导出 Markdown"} {
		if !tt.HasText(want) {
			t.Fatalf("notebook page missing %q: %q", want, tt.Texts())
		}
	}
	for _, forbidden := range []string{"文档", "图板", "归档", "终态归档"} {
		if tt.HasText(forbidden) {
			t.Fatalf("legacy 6-mode text %q should not appear: %q", forbidden, tt.Texts())
		}
	}
	if a.noteDoc == nil || a.noteDoc.Text().Markdown() != "# 今日观察\n\n- 细胞贴壁良好" {
		t.Fatalf("live document did not load the latest revision: %q", a.noteDoc.Text().Markdown())
	}
	if err := tt.Click("源码"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.noteMode != 1 || a.noteDraft != "# 今日观察\n\n- 细胞贴壁良好" {
		t.Fatalf("source mode did not carry latest markdown: mode=%d draft=%q", a.noteMode, a.noteDraft)
	}
	if err := tt.Click("阅读"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.noteMode != 2 || !tt.HasText("今日观察") {
		t.Fatalf("reading mode did not render document: mode=%d texts=%q", a.noteMode, tt.Texts())
	}
	if a.noteDraft != "# 今日观察\n\n- 细胞贴壁良好" || a.noteDirty {
		t.Fatalf("loaded draft=%q dirty=%v", a.noteDraft, a.noteDirty)
	}
}

func TestNotebookAutoSavesWithoutManualSaveButton(t *testing.T) {
	snap, _ := boardTestSnap("run-autosave-1")
	svc := &sealStubSvc{snap: snap, proj: boardTestProj("")}
	a := &app{snap: snap, runSvc: svc}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("源码"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("＋ 小节标题"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if svc.saveCalls != 1 {
		t.Fatalf("expected 1 automatic SaveNotebook call on edit, got %d (err=%q)", svc.saveCalls, a.noteErr)
	}
	if a.noteDirty || a.noteState != "saved" {
		t.Fatalf("expected saved clean state after autosave, dirty=%v state=%q", a.noteDirty, a.noteState)
	}
	if !strings.Contains(a.noteDraft, "## 阶段小节") {
		t.Fatalf("snippet not persisted into noteDraft: %q", a.noteDraft)
	}
}

func TestNotebookReadingModeAndCompanionModulesRenderAsterBlocks(t *testing.T) {
	source := "# D3 实测笔记\n\n```asterlab\nkind: param_table\nid: pt-note\nschema: 1\ntitle: 实测条件摘要\nrows:\n  - {key: temp, label: 培养温度, value: 37, unit: Cel}\n```\n"
	frozenDocSrc := "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: HUVEC 成管\n```\n" +
		"```asterlab\nkind: step\nid: step-2\nschema: 1\ntitle: 405nm 固化\nblocks:\n  - kind: prose\n    id: p-1\n    schema: 1\n    title: 固化要求\n    body: 光照 60s\n```\n"
	snap, runAssets := boardTestSnap("run-comp-1")
	snap.Events = []runs.RunEvent{{ID: "note-1", Stage: "notebook", Content: source, RecordedAt: "2026-11-18T10:00:00Z"}}
	svc := &sealStubSvc{snap: snap, proj: boardTestProj("")}
	a := &app{
		snap:      snap,
		runAssets: runAssets,
		runSvc:    svc,
		asterDoc:  &asterdoc.FrozenAsterDoc{DesignID: "design-1", Revision: 3, Source: frozenDocSrc},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1360, 880)
	tt.Frame()
	if !tt.HasText("实测模块") || !tt.HasText("固化要求") {
		t.Fatalf("right companion panel should default to hydrated 实测模块: %q", tt.Texts())
	}
	if err := tt.Click("阅读"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("实测条件摘要") || !tt.HasText("培养温度") {
		t.Fatalf("embedded asterlab param_table did not render in 阅读: %q", tt.Texts())
	}
}

func TestNotebookSealedRecordLocksIntoReadingMode(t *testing.T) {
	snap, assets := boardTestSnap("run-seal-mode-1")
	proj := boardTestProj("")
	proj.Archive.Status = "signed"
	proj.Archive.Signature = &record.ArchiveSignature{SignedBy: "lin", SignedAtUTC: "2026-11-16T12:00:00Z"}
	svc := &sealStubSvc{snap: snap, proj: proj}
	a := &app{snap: snap, runAssets: assets, runSvc: svc}
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
	if !tt.HasText("记录已锁定") {
		t.Fatalf("sealed state word missing: %q", tt.Texts())
	}
	a.noteMode = 0
	tt.Frame()
	if !tt.HasText("记录已锁定") || !tt.HasText("该实验记录为只读状态") {
		t.Fatalf("sealed live mode did not lock: %q", tt.Texts())
	}
	a.noteDraft = "# 改写"
	a.saveNotebook()
	if a.noteErr != "记录已锁定，笔记只读" {
		t.Fatalf("sealed save was not refused cleanly: %q", a.noteErr)
	}
}

type sealStubSvc struct {
	runs.RunService
	snap      runs.RunSnapshot
	proj      record.Projection
	saveCalls int
}

func (s *sealStubSvc) SaveNotebook(_ context.Context, in runs.NotebookInput) (runs.RunSnapshot, error) {
	s.saveCalls++
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
		},
		Timers: []record.TimerObserve{
			{TimerID: "t1", TaskKey: "task-cure", Mode: "countdown", Status: "completed", PlannedS: u64(60), ElapsedS: u64(75)},
		},
		Images: record.ImageMatrix{
			FOVPerWell: 1, PlannedSlots: 3, FilledSlots: 2, MissingSlots: 1,
			Plates: []record.ImagePlateRow{{
				PlateID: "p1", Timepoints: []*string{str("6h")},
				PlannedSlots: 3, FilledSlots: 2,
				Wells: []record.ImageWellRow{
					{Coordinate: "A1", GroupLabel: str("3%"), PlannedSlots: 3, FilledSlots: 2,
						Slots: []record.ImageSlot{
							{Timepoint: str("6h"), FOVIndex: 1, AssetID: str("img-1")},
							{Timepoint: str("6h"), FOVIndex: 2, AssetID: str("img-2")},
							{Timepoint: str("6h"), FOVIndex: 3, AssetID: fov3},
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
		Notes:   []record.NoteView{{NoteID: "n1", Markdown: "# 研判\n\n贴壁良好\n"}},
		Archive: record.ArchiveState{Status: "recording"},
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
				},
			},
			{ID: "note-1", Stage: "notebook", Content: "# 研判\n\n贴壁良好", RecordedAt: "2026-11-10T17:00:00Z"},
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
