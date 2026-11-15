package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/designer"
	"asterlab-mygo-prototype/internal/resultgrid"
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

func TestSelectTaskPreservesUnsavedDraft(t *testing.T) {
	value := 1.23
	grid := resultgrid.New(resultgrid.Schema{
		RowLabels: []string{"A"}, ColumnLabels: []string{"OD"},
		Fields: []resultgrid.Field{{Key: "od"}},
	}, nil)
	cell, _ := grid.CellAt(0, 0)
	cell.SampleID, cell.Value, cell.Status = "sample-a", &value, "raw"
	if err := grid.SetCell(cell); err != nil {
		t.Fatal(err)
	}
	editBuffer, _ := grid.InputAt(0, 0)
	*editBuffer = "1.234"
	ws := &runui.Workspace{
		Selected: "task-a", Grid: grid,
		Draft: runui.Draft{Rows: []runui.DraftRow{{SampleID: "sample-a", Value: "1.23"}}},
	}
	a := &app{snap: runs.RunSnapshot{RunID: "run-a"}, ws: ws, selectedTask: "task-a"}

	a.selectRunTask("task-b")
	if ws.Selected != "task-b" || ws.Grid != nil {
		t.Fatalf("switch to task B = selected %q grid %p", ws.Selected, ws.Grid)
	}
	ws.Draft.Rows[0].SampleID = "sample-b"
	a.selectRunTask("task-a")
	if ws.Grid != grid {
		t.Fatal("task A did not restore the same grid and its active edit state")
	}
	restoredInput, _ := ws.Grid.InputAt(0, 0)
	if restoredInput == nil || *restoredInput != "1.234" {
		t.Fatalf("active cell edit buffer was lost: %v", restoredInput)
	}
	got, ok := ws.Grid.CellAt(0, 0)
	if !ok || got.Value == nil || *got.Value != value || got.SampleID != "sample-a" {
		t.Fatalf("task A grid draft was lost: %+v", got)
	}
	if len(ws.Draft.Rows) != 1 || ws.Draft.Rows[0].SampleID != "sample-a" {
		t.Fatalf("task A row draft was lost: %+v", ws.Draft)
	}

	a.clearTaskRecordingFor(a.snap.RunID, "task-a")
	if _, ok := a.taskGrids[runTaskCacheKey(a.snap.RunID, "task-a")]; ok {
		t.Fatal("successful save left a cached task grid")
	}
	if _, ok := a.taskDrafts[runTaskCacheKey(a.snap.RunID, "task-a")]; ok {
		t.Fatal("successful save left a cached task draft")
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

type batchRetrySpy struct {
	runs.RunService
	failRemaining int
	opIDs         []string
	snap          runs.RunSnapshot
}

func (s *batchRetrySpy) SaveReadoutsBatch(_ context.Context, input runs.BatchReadoutInput) (runs.BatchReadoutResult, error) {
	s.opIDs = append(s.opIDs, input.OperationID)
	if s.failRemaining > 0 {
		s.failRemaining--
		return runs.BatchReadoutResult{}, fmt.Errorf("transient write failure")
	}
	s.snap.Revision++
	return runs.BatchReadoutResult{RunID: input.RunID, Revision: s.snap.Revision, Accepted: len(input.Cells)}, nil
}

func (s *batchRetrySpy) LoadRun(context.Context, string) (runs.RunSnapshot, error) {
	return s.snap, nil
}

func (s *batchRetrySpy) WorkflowTree(context.Context, string) (string, error) {
	return "", nil
}

func (s *batchRetrySpy) ListAssetLinks(context.Context, string) ([]runs.Asset, error) {
	return nil, nil
}

func TestSaveBatchReusesOperationIDAcrossRetries(t *testing.T) {
	t.Setenv("LABWORKBENCH_DATA_DIR", t.TempDir())
	schema := resultgrid.Schema{
		ID: "cck8.plate96.od450.v1", Version: 1,
		RowLabels: []string{"A"}, ColumnLabels: []string{"1"},
		Fields: []resultgrid.Field{{Key: "od450", Label: "OD450", Unit: "AU"}},
	}
	grid := resultgrid.New(schema, nil)
	v := 0.85
	_ = grid.SetCell(resultgrid.Cell{Row: 0, Column: 0, SampleID: "S1", Field: "od450", Value: &v, Status: "raw"})

	spy := &batchRetrySpy{failRemaining: 1, snap: runs.RunSnapshot{RunID: "run-retry", Revision: 1, Tasks: []runs.RunTask{{Key: "t-1"}}}}
	a := &app{
		runSvc: spy,
		snap:   spy.snap,
		ws:     &runui.Workspace{Selected: "t-1", Grid: grid},
	}

	// First attempt fails -> error state preserved with PendingOpID.
	a.saveBatch(schema, grid.Cells)
	if grid.State != resultgrid.StateError || grid.PendingOpID == "" || len(spy.opIDs) != 1 {
		t.Fatalf("after failure: state=%q op=%q ops=%v", grid.State, grid.PendingOpID, spy.opIDs)
	}

	// Retry unchanged cells -> must reuse exact same OperationID.
	a.saveBatch(schema, grid.Cells)
	if grid.State != resultgrid.StateSaved || grid.PendingOpID != "" || len(spy.opIDs) != 2 {
		t.Fatalf("after retry: state=%q op=%q ops=%v", grid.State, grid.PendingOpID, spy.opIDs)
	}
	if spy.opIDs[0] != spy.opIDs[1] {
		t.Fatalf("retry generated new operationID %q != %q", spy.opIDs[1], spy.opIDs[0])
	}

	// Subsequent edit after commit allocates a fresh OperationID.
	v2 := 0.91
	_ = grid.SetCell(resultgrid.Cell{Row: 0, Column: 0, SampleID: "S1", Field: "od450", Value: &v2, Status: "raw"})
	a.saveBatch(schema, grid.Cells)
	if len(spy.opIDs) != 3 || spy.opIDs[2] == spy.opIDs[1] {
		t.Fatalf("new edit reused old operationID: %v", spy.opIDs)
	}
}
func TestRunAssetsForTaskPreservesCoordinatesAndUnlabelledAssets(t *testing.T) {
	a := &app{runAssets: []runs.Asset{
		{
			ID: "full", RelativePath: "images/A1_24h_F03.tif",
			Link: runs.AssetLink{TaskKey: "task", WellID: "B2", Timepoint: "t6h", FOVIndex: 3},
		},
		{
			ID: "empty", RelativePath: "images/A1_24h_F03.tif",
			Link: runs.AssetLink{TaskKey: "task"},
		},
	}}
	projected := a.runAssetsForTask("task")
	if len(projected) != 2 {
		t.Fatalf("projected %d assets, want 2", len(projected))
	}
	if got := projected[0]; got.WellID != "B2" || got.Timepoint != "t6h" || got.FOVIndex != 3 {
		t.Fatalf("full coordinates lost in projection: %+v", got)
	}
	if got := projected[1]; got.WellID != "" || got.Timepoint != "" || got.FOVIndex != 0 {
		t.Fatalf("missing coordinates were inferred: %+v", got)
	}
}
