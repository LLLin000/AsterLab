package main

import (
	"context"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/designer"
	"asterlab-mygo-prototype/internal/runui"
	"asterlab-mygo-prototype/internal/runworkspace"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/modules/runs"
)

type runServiceSpy struct {
	runs.RunService
	loadCalls int
}

func (s *runServiceSpy) LoadRun(context.Context, string) (runs.RunSnapshot, error) {
	s.loadCalls++
	return runs.RunSnapshot{}, nil
}

func TestLoadRunWithoutConfiguredIDDoesNotInventRun(t *testing.T) {
	t.Setenv("ASTERLAB_RUN_ID", "")
	service := &runServiceSpy{}
	workspace := &runui.Workspace{
		View:     runui.RunView{Title: "stale"},
		State:    runui.StateLoading,
		Selected: "stale-task",
	}
	app := &app{
		runSvc: service,
		snap:   runs.RunSnapshot{RunID: "stale", Title: "stale"},
		ws:     workspace,
	}

	snap, err := app.fetchRun()
	app.applyRun(snap, err)
	if service.loadCalls != 0 {
		t.Fatalf("LoadRun called %d times without an explicit run ID", service.loadCalls)
	}
	if app.snap.RunID != "" || app.snap.Title != "" || workspace.View.Title != "" {
		t.Fatalf("missing run ID retained run data: snap=%+v view=%+v", app.snap, workspace.View)
	}
	if workspace.State != runui.StateEmpty || workspace.Selected != "" || workspace.Grid != nil {
		t.Fatalf("missing run ID did not produce an empty workspace: %+v", workspace)
	}
}

func TestSelectDesignKeepsDirtyDraftUntilUserConfirms(t *testing.T) {
	current := designer.New("design-current", "Current", 1, nil)
	current.Touch()
	workspace := &shell.Shell{Selected: "design-next"}
	app := &app{designer: current, shell: workspace}

	app.selectDesign("design-next")
	if app.pendingDesignID != "design-next" || workspace.Selected != "design-current" || app.designer != current {
		t.Fatalf("dirty draft was replaced before confirmation: pending=%q selected=%q", app.pendingDesignID, workspace.Selected)
	}
	app.selectDesign("design-current")
	if app.designer != current || !current.Dirty || app.pendingDesignID != "design-next" {
		t.Fatalf("selecting the active design disturbed pending draft state")
	}
}

func TestRenderRunRecordsIncludesGroupedAndUnassociatedData(t *testing.T) {
	vm := &runworkspace.RunWorkspaceViewModel{
		Steps: []*runworkspace.RunTaskViewModel{
			{
				StepTitle: "RNA 分组",
				Children: []*runworkspace.RunTaskViewModel{
					{
						StepTitle: "RNA 反应",
						Timepoint: "24 h",
						Parameters: []runworkspace.PlannedParameter{
							{Key: "speed", Label: "离心力", Value: "3000", Unit: "rpm"},
						},
						Actual: &runworkspace.ActualRecord{
							Parameters: map[string]string{"speed": "3200 rpm"},
							Measurements: []runworkspace.MeasurementView{
								{SampleID: "S1", Field: "OD450", Value: "0.82", Unit: "a.u."},
							},
						},
						Events: []runworkspace.EventViewModel{
							{
								Type: "measurement", Content: "读取完成", RecordedAt: "2026-11-13T12:00:00Z",
								BatchID: "B-1", CheckAck: true, Deviation: "读数偏高",
								Checks: []runworkspace.CheckAckView{{CheckID: "CHECK-1", Status: "passed"}},
								Materials: []runworkspace.MaterialUseView{{Material: "试剂A", Amount: 1.25, Unit: "mL"}},
								Parameters: []runworkspace.PlannedParameter{{Key: "speed", Value: "3200", Unit: "rpm"}},
								Lineage: []runworkspace.LineageView{{
									FromKind: "sample", FromID: "S1", ToKind: "result", ToID: "R1", Relation: "measured",
								}},
								Measurements: []runworkspace.MeasurementView{
									{SampleID: "S1", Field: "OD450", Value: "0.82", Unit: "a.u."},
								},
							},
						},
					},
					{StepTitle: "RNA 反应", Timepoint: "48 h"},
				},
			},
		},
		UnassociatedMeasurements: []runworkspace.MeasurementView{
			{SampleID: "S2", Field: "OD450", Value: "0.74", Unit: "a.u."},
		},
		UnassociatedEvents: []runworkspace.EventViewModel{
			{Type: "observation", Content: "未关联事件"},
		},
	}
	got := renderRunRecords(vm, 4, "2026-11-13T12:00:00Z")
	if got != renderRunRecords(vm, 4, "2026-11-13T12:00:00Z") {
		t.Fatal("identical Run snapshot produced different record text")
	}
	for _, marker := range []string{
		"Run Revision: 4",
		"冻结时间：2026-11-13T12:00:00Z",
		"### RNA 反应 · 24 h",
		"离心力: 3000 rpm；实际 3200 rpm",
		"S1 · OD450 0.82 a.u.",
		"#### 测量 · 2026-11-13T12:00:00Z",
		"Batch B-1",
		"检查确认",
		"偏差：读数偏高",
		"CHECK-1 · passed",
		"试剂A 1.25 mL",
		"speed 3200 rpm",
		"sample:S1 → result:R1 (measured)",
		"未关联记录",
		"S2 · OD450 0.74 a.u.",
		"观察 · 未关联事件",
	} {
		if !strings.Contains(got, marker) {
			t.Fatalf("Run record text is missing %q:\n%s", marker, got)
		}
	}
	first := strings.Index(got, "### RNA 反应 · 24 h")
	second := strings.Index(got, "### RNA 反应 · 48 h")
	if first < 0 || second <= first {
		t.Fatal("grouped task records were missing or out of UI order")
	}
	if strings.Contains(got, "### RNA 分组") {
		t.Fatal("group container was rendered instead of its Run task children")
	}
}
