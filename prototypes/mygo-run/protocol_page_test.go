package main

import (
	"testing"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
)

func TestProtocolNeverFallsBackToLiveDesignForSelectedRun(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{RunID: "run-1", DesignID: "design-1", DesignRevision: 3}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		protocolPage(c, a)
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("已选择 Run，但它的冻结设计缺失。") {
		t.Fatal("missing frozen design did not render its error state")
	}
	if !tt.HasText("不会改用实时设计") {
		t.Fatal("error state does not state that the live design is refused")
	}
}
func archiveTestSnap(runID, prose string) runs.RunSnapshot {
	return runs.RunSnapshot{
		RunID: runID, Title: "HUVEC 成管",
		DesignID: "design-1", DesignRevision: 3, Status: "completed",
		AsterDoc: &runs.AsterDocRef{Revision: 5},
		Tasks: []runs.RunTask{
			{
				Key: "task-cure", Title: "405nm 固化", Timepoint: "0h",
				Status: "completed", DesignStepID: "step-2",
				ParametersJSON: `[{"key":"cure_s","label":"固化秒数","value":60,"unit":"s"}]`,
			},
		},
		Events: []runs.RunEvent{
			{
				ID: "evt-read", EventType: "measurement", TaskKey: "task-cure", Timepoint: "6h",
				BatchID: "batch-1", RecordedAt: "2026-11-10T16:01:00+08:00",
				ResultCells: []runs.ResultCell{
					{Row: 0, Column: 0, SampleID: "S1", Field: "OD450", Value: new(0.82)},
				},
			},
			{ID: "note-1", Stage: "notebook", Content: prose, RecordedAt: "2026-11-10T17:00:00Z"},
		},
	}
}

func TestProtocolArchiveToggleRendersEndState(t *testing.T) {
	a := &app{
		snap:     archiveTestSnap("run-arch-1", "# 研判\n\n结论见对比 @matrix(6h)"),
		asterErr: "冻结文档损坏（测试夹具）",
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		protocolPage(c, a)
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("终态归档") {
		t.Fatalf("archive toggle missing: %q", tt.Texts())
	}
	if err := tt.Click("终态归档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"终态归档", "实验记录归档", "计划 vs 实际", "导出归档包", "导出归档 PDF", "未签核"} {
		if !tt.HasText(want) {
			t.Fatalf("archive view missing %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("record-sign") {
		t.Fatal("signature trailer leaked into the archive view")
	}
}

func TestProtocolArchiveShowsSealedState(t *testing.T) {
	snap := archiveTestSnap("run-arch-2", "# 研判\n\n贴壁良好")
	proj := boardTestProj("")
	proj.Archive.Status = "signed"
	proj.Archive.Signature = &record.ArchiveSignature{SignedBy: "lin", SignedAtUTC: "2026-11-16T12:00:00Z"}
	a := &app{
		snap:     snap,
		runSvc:   &sealStubSvc{snap: snap, proj: proj},
		asterErr: "冻结文档损坏（测试夹具）",
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		protocolPage(c, a)
	}, 900, 600)
	tt.Frame()
	if err := tt.Click("终态归档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("已签核归档") {
		t.Fatalf("sealed state missing: %q", tt.Texts())
	}
	if !tt.HasText("lin") {
		t.Fatalf("signer missing from sealed archive: %q", tt.Texts())
	}
}
