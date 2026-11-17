package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/designer"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/runui"
	"asterlab-mygo-prototype/internal/runworkspace"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
)

type runServiceSpy struct {
	runs.RunService
	loadCalls int
}

func (s *runServiceSpy) LoadRun(context.Context, string) (runs.RunSnapshot, error) {
	s.loadCalls++
	return runs.RunSnapshot{}, nil
}

// ListTimers keeps the run bench alive on tests whose fake predates the
// timer bridge: no persisted timers, never a nil-embedded panic.
func (s *runServiceSpy) ListTimers(context.Context, string) ([]runs.TimerRecord, error) {
	return nil, nil
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

// ListTimers keeps the run bench alive on tests whose fake predates the
// timer bridge: no persisted timers, never a nil-embedded panic.
func (s *batchRetrySpy) ListTimers(context.Context, string) ([]runs.TimerRecord, error) {
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

func TestValidateAssetCoordinatesRejectsUnknownValuesAndFOVZero(t *testing.T) {
	wells, times := []string{"A1", "B1"}, []string{"6 h", "24 h"}
	for _, tc := range []struct {
		name, well, timepoint, fov string
	}{
		{"unknown well", "Z9", "6 h", "1"},
		{"unknown timepoint", "A1", "48 h", "1"},
		{"zero FOV", "A1", "6 h", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := validateAssetCoordinates(tc.well, tc.timepoint, tc.fov, wells, times); err == nil {
				t.Fatal("invalid coordinate was accepted")
			}
		})
	}
}

func TestBatchLinkInheritsCoordinateForEveryPickedPath(t *testing.T) {
	want := runs.AssetLink{RunID: "run", TaskKey: "task", WellID: "B1", Timepoint: "24 h", FOVIndex: 2}
	var paths []string
	var links []runs.AssetLink
	err := linkPickedAssets([]string{"one.tif", "two.tif"}, want, func(path string, link runs.AssetLink) error {
		paths = append(paths, path)
		links = append(links, link)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || len(links) != 2 || links[0] != want || links[1] != want {
		t.Fatalf("batch paths=%v links=%+v, want two identical coordinates %+v", paths, links, want)
	}
}

func TestDesignPlateWellsUsesConfiguredPlateGeometry(t *testing.T) {
	wells := designPlateWells(`{"mygo":{"instances":{"step":[{"id":"plate","type":"plate_12","props":{"rows":3,"cols":4}}]},"assays":{"assay":{"plateInstanceId":"plate"}}}}`, "assay")
	if len(wells) != 12 || wells[0] != "A1" || wells[len(wells)-1] != "C4" {
		t.Fatalf("plate geometry wells = %v", wells)
	}
}

func TestOpenAssetLinkFormRetainsGapCoordinatesAndUsesRunTimepoints(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{
		Tasks: []runs.RunTask{
			{Key: "task", AssayID: "assay", Timepoint: "24 h"},
			{Key: "task-2", AssayID: "assay", Timepoint: "6 h"},
		},
		DesignData: `{"mygo":{"instances":{"step":[{"id":"plate","type":"plate_12","props":{"rows":3,"cols":4}}]},"assays":{"assay":{"plateInstanceId":"plate"}}}}`,
	}}
	a.openAssetLinkForm("task", runworkspace.LinkedAsset{
		WellID: "B2", Timepoint: "24 h", FOVIndex: 2,
	})
	if !a.linkDialogOpen || a.linkDialogWell != "B2" || a.linkDialogTimepoint != "24 h" || a.linkDialogFOV != "2" {
		t.Fatalf("form did not preserve gap coordinate: %+v", a)
	}
	if len(a.linkDialogWells) != 12 || a.linkDialogWells[0] != "A1" ||
		len(a.linkDialogTimepoints) != 2 || a.linkDialogTimepoints[0] != "24 h" || a.linkDialogTimepoints[1] != "6 h" {
		t.Fatalf("form options = wells %v, timepoints %v", a.linkDialogWells, a.linkDialogTimepoints)
	}
}

func TestAssetCoordinateOptionsUseFrozenAssayTimepointsWhenRunTaskOmitsOne(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{
		Tasks:      []runs.RunTask{{Key: "task", AssayID: "assay"}},
		DesignData: `{"mygo":{"assays":{"assay":{"timepoints":["6 h","24 h"]}}}}`,
	}}
	_, got := a.assetCoordinateOptions("task")
	if len(got) != 2 || got[0] != "6 h" || got[1] != "24 h" {
		t.Fatalf("task timepoint options = %v", got)
	}
}

func TestAssetCoordinateOptionsRejectsOtherAssayTimepoint(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{
		Tasks: []runs.RunTask{
			{Key: "task-a", AssayID: "assay-a", Timepoint: "24 h"},
			{Key: "task-b", AssayID: "assay-b", Timepoint: "48 h"},
		},
		DesignData: `{"mygo":{"instances":{"step":[{"id":"plate","type":"plate_12","props":{"rows":3,"cols":4}}]},"assays":{"assay-a":{"plateInstanceId":"plate","timepoints":["6 h","24 h"]},"assay-b":{"plateInstanceId":"plate","timepoints":["48 h"]}}}}`,
	}}
	wells, times := a.assetCoordinateOptions("task-a")
	if _, err := validateAssetCoordinates("A1", "48 h", "1", wells, times); err == nil {
		t.Fatalf("another assay's timepoint 48 h was accepted: options=%v", times)
	}
	if len(times) != 2 || times[0] != "24 h" || times[1] != "6 h" {
		t.Fatalf("assay-a timepoint options = %v, want [24 h 6 h]", times)
	}
}

func TestAssetCoordinateOptionsIncludesAllFrozenTimepointsForGapPrefill(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{
		Tasks:      []runs.RunTask{{Key: "task", AssayID: "assay", Timepoint: "24 h"}},
		DesignData: `{"mygo":{"instances":{"step":[{"id":"plate","type":"plate_12","props":{"rows":3,"cols":4}}]},"assays":{"assay":{"plateInstanceId":"plate","timepoints":["6 h","24 h"]}}}}`,
	}}
	a.openAssetLinkForm("task", runworkspace.LinkedAsset{
		WellID: "A1", Timepoint: "6 h", FOVIndex: 1,
	})
	if len(a.linkDialogTimepoints) != 2 || a.linkDialogTimepoints[0] != "24 h" || a.linkDialogTimepoints[1] != "6 h" {
		t.Fatalf("timepoint options = %v, want both [24 h 6 h]", a.linkDialogTimepoints)
	}
	got, err := validateAssetCoordinates(
		a.linkDialogWell, a.linkDialogTimepoint, a.linkDialogFOV,
		a.linkDialogWells, a.linkDialogTimepoints,
	)
	if err != nil || got.Timepoint != "6 h" {
		t.Fatalf("gap prefill validation failed: got=%+v err=%v", got, err)
	}
}

func TestOpenAssetLinkFormTrimsDefaultTaskTimepoint(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{
		Tasks:      []runs.RunTask{{Key: "task", AssayID: "assay", Timepoint: "  24 h \t"}},
		DesignData: `{"mygo":{"instances":{"step":[{"id":"plate","type":"plate_12","props":{"rows":3,"cols":4}}]},"assays":{"assay":{"plateInstanceId":"plate"}}}}`,
	}}
	a.openAssetLinkForm("task", runworkspace.LinkedAsset{})
	if a.linkDialogTimepoint != "24 h" {
		t.Fatalf("default timepoint = %q, want %q", a.linkDialogTimepoint, "24 h")
	}
	if _, err := validateAssetCoordinates(
		a.linkDialogWell, a.linkDialogTimepoint, a.linkDialogFOV,
		a.linkDialogWells, a.linkDialogTimepoints,
	); err != nil {
		t.Fatalf("default trimmed timepoint failed validation: %v", err)
	}
}

type assetSaveSpy struct {
	runs.RunService
	saveCalls int
	saved     []runs.Asset
}

func (s *assetSaveSpy) SaveAssetLink(_ context.Context, input runs.AssetLinkInput) (runs.Asset, error) {
	s.saveCalls++
	s.saved = append(s.saved, input.Asset)
	return input.Asset, nil
}

// ListTimers keeps the run bench alive on tests whose fake predates the
// timer bridge: no persisted timers, never a nil-embedded panic.
func (s *assetSaveSpy) ListTimers(context.Context, string) ([]runs.TimerRecord, error) {
	return nil, nil
}

func TestLinkAssetPathsRejectsOutOfBoundsBatchWithoutSavingFirstFile(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	first := filepath.Join(rootDir, "one.tif")
	validSecond := filepath.Join(rootDir, "two.tif")
	outsideSecond := filepath.Join(outsideDir, "outside.tif")
	for _, p := range []string{first, validSecond, outsideSecond} {
		if err := os.WriteFile(p, []byte("tif-data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	spy := &assetSaveSpy{}
	a := &app{
		runSvc:              spy,
		snap:                runs.RunSnapshot{RunID: "run-1", DesignID: "design-1"},
		linkDialogOpen:      true,
		linkDialogTask:      "task-1",
		linkDialogWells:     []string{"A1"},
		linkDialogTimepoints: []string{"24 h"},
		linkDialogWell:      "A1",
		linkDialogTimepoint: "24 h",
		linkDialogFOV:       "1",
	}

	a.submitAssetLinkForm(func(taskKey string, coordinate runs.AssetLink) error {
		coordinate.RunID, coordinate.TaskKey = a.snap.RunID, taskKey
		return a.linkAssetPaths("design-data-design-1", rootDir, []string{first, outsideSecond}, coordinate)
	})
	if spy.saveCalls != 0 {
		t.Fatalf("SaveAssetLink called %d times when 2nd path was out of bounds, want 0", spy.saveCalls)
	}
	if len(a.runAssets) != 0 {
		t.Fatalf("runAssets mutated after failed batch: %+v", a.runAssets)
	}
	if a.linkDialogErr == "" || !a.linkDialogOpen {
		t.Fatalf("form did not keep error open on batch failure: open=%v err=%q", a.linkDialogOpen, a.linkDialogErr)
	}

	a.submitAssetLinkForm(func(taskKey string, coordinate runs.AssetLink) error {
		coordinate.RunID, coordinate.TaskKey = a.snap.RunID, taskKey
		return a.linkAssetPaths("design-data-design-1", rootDir, []string{first, validSecond}, coordinate)
	})
	if spy.saveCalls != 2 || len(a.runAssets) != 2 {
		t.Fatalf("valid batch saved %d calls / %d assets, want 2", spy.saveCalls, len(a.runAssets))
	}
	if a.linkDialogErr != "" || a.linkDialogOpen {
		t.Fatalf("form did not clear error/close after valid batch: open=%v err=%q", a.linkDialogOpen, a.linkDialogErr)
	}
}

type checkFailRunService struct {
	runs.RunService
	failChecks int
	checkCalls int
	failLoad   int
	loadCalls  int
	snap       runs.RunSnapshot
}

func (s *checkFailRunService) SaveTaskChecks(_ context.Context, input runs.TaskCheckInput) (runs.RunSnapshot, error) {
	s.checkCalls++
	if s.failChecks > 0 {
		s.failChecks--
		return runs.RunSnapshot{}, fmt.Errorf("CONFLICT: checklist write rejected")
	}
	for ti := range s.snap.Tasks {
		if s.snap.Tasks[ti].Key == input.TaskKey {
			for ci := range s.snap.Tasks[ti].Checks {
				for _, ack := range input.Checks {
					if s.snap.Tasks[ti].Checks[ci].CheckID == ack.CheckID {
						s.snap.Tasks[ti].Checks[ci].Status = ack.Status
					}
				}
			}
		}
	}
	s.snap.Revision++
	return s.snap, nil
}

func (s *checkFailRunService) LoadRun(_ context.Context, runID string) (runs.RunSnapshot, error) {
	s.loadCalls++
	if s.failLoad > 0 {
		s.failLoad--
		return runs.RunSnapshot{}, fmt.Errorf("NOT_FOUND: run %s not found", runID)
	}
	return s.snap, nil
}

func (s *checkFailRunService) WorkflowTree(context.Context, string) (string, error) {
	return "", nil
}

func (s *checkFailRunService) ListAssetLinks(context.Context, string) ([]runs.Asset, error) {
	return nil, nil
}

// ListTimers keeps the run bench alive on tests whose fake predates the
// timer bridge: no persisted timers, never a nil-embedded panic.
func (s *checkFailRunService) ListTimers(context.Context, string) ([]runs.TimerRecord, error) {
	return nil, nil
}

func TestSaveRunChecksFailureRevertsAndRetriesThroughApp(t *testing.T) {
	svc := &checkFailRunService{
		failChecks: 1,
		snap: runs.RunSnapshot{
			RunID:    "run-1",
			Title:    "Run 1",
			Revision: 1,
			Status:   "running",
			Tasks: []runs.RunTask{
				{
					Key:    "task-1",
					Title:  "步骤一",
					Status: "not_started",
					Checks: []runs.TaskCheck{
						{CheckID: "chk-1", Label: "台面已消毒", Blocking: true, Status: "pending"},
					},
				},
			},
		},
	}
	a := &app{
		runSvc: svc,
		snap:   svc.snap,
		ws:     runui.NewWorkspace(runui.RunView{}, runui.Callbacks{}),
	}
	a.runWSVM = a.buildRunWorkspaceVM()
	a.runWS = runworkspace.NewState(a.runWSVM, runworkspace.Callbacks{
		SaveChecks: a.saveRunChecks,
	})

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		runworkspace.View(c, a.runWS)
	}, 1360, 880)
	tt.Frame()

	if err := tt.Click("台面已消毒"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if svc.checkCalls != 1 {
		t.Fatalf("SaveTaskChecks calls = %d, want 1", svc.checkCalls)
	}
	if !tt.HasText("CONFLICT: checklist write rejected") || !tt.HasText("重试") {
		t.Fatalf("check error/retry not visible: %v", tt.Texts())
	}
	if a.runWS.VM.TaskByKey["task-1"].Checks[0].Status != "pending" {
		t.Fatalf("persisted status mutated on failure: %+v", a.runWS.VM.TaskByKey["task-1"].Checks)
	}

	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if svc.checkCalls != 2 {
		t.Fatalf("SaveTaskChecks calls after retry = %d, want 2", svc.checkCalls)
	}
	if tt.HasText("CONFLICT: checklist write rejected") {
		t.Fatalf("check error did not clear on retry: %v", tt.Texts())
	}
	if a.runWS.VM.TaskByKey["task-1"].Checks[0].Status != "passed" {
		t.Fatalf("persisted status not updated after retry: %+v", a.runWS.VM.TaskByKey["task-1"].Checks)
	}
}

func TestApplyRunLoadErrorUpdatesRunWorkspaceAndRetries(t *testing.T) {
	svc := &checkFailRunService{
		failLoad: 1,
		snap: runs.RunSnapshot{
			RunID:    "run-missing",
			Title:    "恢复后的运行",
			Revision: 2,
			Status:   "running",
			Tasks: []runs.RunTask{
				{Key: "task-1", Title: "接种步骤", Status: "not_started"},
			},
		},
	}
	a := &app{
		runSvc: svc,
		snap:   runs.RunSnapshot{Title: "加载中", Status: "loading"},
		shell:  &shell.Shell{Page: shell.Run},
		ws:     &runui.Workspace{State: runui.StateLoading},
	}
	a.runWSVM = a.buildRunWorkspaceVM()
	a.runWS = runworkspace.NewState(a.runWSVM, runworkspace.Callbacks{
		RetryLoad: a.retryLoadRun,
	})

	a.selectRun("run-missing")
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		runPage(c, a)
	}, 1360, 880)
	tt.Frame()

	if tt.HasText("正在加载实验运行数据…") {
		t.Fatalf("Run panel stayed stuck on loading message: %v", tt.Texts())
	}
	if !tt.HasText("NOT_FOUND: run run-missing not found") || !tt.HasText("重试") {
		t.Fatalf("Run load failure missing error or retry: %v", tt.Texts())
	}

	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if svc.loadCalls != 2 {
		t.Fatalf("LoadRun calls after retry = %d, want 2", svc.loadCalls)
	}
	if tt.HasText("NOT_FOUND: run run-missing not found") || !tt.HasText("恢复后的运行") {
		t.Fatalf("retry did not load Run workspace: %v", tt.Texts())
	}
}

type designLoadFakeService struct {
	runs.DesignService
	designs      map[string]runs.DesignSnapshot
	docs         map[string]runs.AsterDocSnapshot
	loadDesign   func(id string) (runs.DesignSnapshot, error)
	loadAsterDoc func(id string) (runs.AsterDocSnapshot, error)
	designCalls  []string
	docCalls     []string
}

func (s *designLoadFakeService) LoadDesign(_ context.Context, designID string) (runs.DesignSnapshot, error) {
	s.designCalls = append(s.designCalls, designID)
	if s.loadDesign != nil {
		return s.loadDesign(designID)
	}
	snap, ok := s.designs[designID]
	if !ok {
		return runs.DesignSnapshot{}, fmt.Errorf("NOT_FOUND: design %s not found", designID)
	}
	return snap, nil
}

func (s *designLoadFakeService) LoadAsterDoc(_ context.Context, designID string) (runs.AsterDocSnapshot, error) {
	s.docCalls = append(s.docCalls, designID)
	if s.loadAsterDoc != nil {
		return s.loadAsterDoc(designID)
	}
	doc, ok := s.docs[designID]
	if !ok {
		return runs.AsterDocSnapshot{}, fmt.Errorf("NOT_FOUND: asterdoc %s not found", designID)
	}
	return doc, nil
}

func (s *designLoadFakeService) AssessAsterDoc(_ context.Context, designID string, revision uint64) (runs.AsterDocAssessment, error) {
	return runs.AsterDocAssessment{DesignID: designID, Revision: revision, State: "ready"}, nil
}

func TestLoadDesignFailuresVisibleKeepLoadedAndRetry(t *testing.T) {
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)

	t.Run("LoadDesign failure keeps existing design visible and retries", func(t *testing.T) {
		failB := true
		svc := &designLoadFakeService{
			designs: map[string]runs.DesignSnapshot{
				"design-a": {DesignID: "design-a", Title: "Design A", Revision: 1},
				"design-b": {DesignID: "design-b", Title: "Design B", Revision: 2},
			},
			docs: map[string]runs.AsterDocSnapshot{
				"design-a": {DesignID: "design-a", Revision: 1, Source: source},
				"design-b": {DesignID: "design-b", Revision: 2, Source: strings.Replace(source, "title: ", "title: B-", 1)},
			},
		}
		svc.loadDesign = func(id string) (runs.DesignSnapshot, error) {
			if id == "design-b" && failB {
				return runs.DesignSnapshot{}, fmt.Errorf("NOT_FOUND: design-b unavailable")
			}
			return svc.designs[id], nil
		}

		a := &app{designSvc: svc, shell: &shell.Shell{Page: shell.Design}}
		a.loadDesign("design-a")
		if a.designDoc == nil || a.designDoc.DesignID != "design-a" {
			t.Fatalf("failed to load initial design-a: %+v", a.designDoc)
		}

		a.loadDesign("design-b")
		if a.designDoc == nil || a.designDoc.DesignID != "design-a" {
			t.Fatalf("failed LoadDesign overwrote loaded design-a: %+v", a.designDoc)
		}

		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			designPage(c, a)
		}, 1360, 880)
		tt.Frame()

		if !tt.HasText("AsterDoc 设计") || !tt.HasText("rev 1") {
			t.Fatalf("already loaded design-a was hidden on LoadDesign error: %v", tt.Texts())
		}
		if !tt.HasText("NOT_FOUND: design-b unavailable") || !tt.HasText("重试") {
			t.Fatalf("LoadDesign error or retry missing: %v", tt.Texts())
		}

		failB = false
		if err := tt.Click("重试"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.designDoc == nil || a.designDoc.DesignID != "design-b" {
			t.Fatalf("retry did not load design-b: %+v", a.designDoc)
		}
		if tt.HasText("NOT_FOUND: design-b unavailable") || !tt.HasText("rev 2") {
			t.Fatalf("error stayed or new design not rendered after retry: %v", tt.Texts())
		}
	})

	t.Run("LoadDesign failure on empty workspace offers retry", func(t *testing.T) {
		failOnce := true
		svc := &designLoadFakeService{
			designs: map[string]runs.DesignSnapshot{
				"design-1": {DesignID: "design-1", Title: "Design 1", Revision: 1},
			},
			docs: map[string]runs.AsterDocSnapshot{
				"design-1": {DesignID: "design-1", Revision: 1, Source: source},
			},
		}
		svc.loadDesign = func(id string) (runs.DesignSnapshot, error) {
			if failOnce {
				failOnce = false
				return runs.DesignSnapshot{}, fmt.Errorf("NOT_FOUND: design-1 missing")
			}
			return svc.designs[id], nil
		}

		a := &app{designSvc: svc, shell: &shell.Shell{Page: shell.Design}}
		a.loadDesign("design-1")

		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			designPage(c, a)
		}, 1360, 880)
		tt.Frame()

		if !tt.HasText("NOT_FOUND: design-1 missing") || !tt.HasText("重试") {
			t.Fatalf("empty workspace LoadDesign failure missing error or retry: %v", tt.Texts())
		}

		if err := tt.Click("重试"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.designDoc == nil || a.designDoc.DesignID != "design-1" {
			t.Fatalf("retry did not load design-1: %+v", a.designDoc)
		}
		if tt.HasText("NOT_FOUND: design-1 missing") || !tt.HasText("AsterDoc 设计") {
			t.Fatalf("AsterDoc surface not shown after retry: %v", tt.Texts())
		}
	})

	t.Run("LoadAsterDoc failure keeps designer visible and retries", func(t *testing.T) {
		failDoc := true
		svc := &designLoadFakeService{
			designs: map[string]runs.DesignSnapshot{
				"design-1": {DesignID: "design-1", Title: "Legacy Fallback Design", Revision: 3},
			},
			docs: map[string]runs.AsterDocSnapshot{
				"design-1": {DesignID: "design-1", Revision: 4, Source: source},
			},
		}
		svc.loadAsterDoc = func(id string) (runs.AsterDocSnapshot, error) {
			if failDoc {
				return runs.AsterDocSnapshot{}, fmt.Errorf("INTERNAL: asterdoc read failed")
			}
			return svc.docs[id], nil
		}

		a := &app{designSvc: svc, shell: &shell.Shell{Page: shell.Design}}
		a.loadDesign("design-1")

		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			designPage(c, a)
		}, 1360, 880)
		tt.Frame()

		if !tt.HasText("Legacy Fallback Design") {
			t.Fatalf("already loaded designer was hidden when LoadAsterDoc failed: %v", tt.Texts())
		}
		if !tt.HasText("INTERNAL: asterdoc read failed") || !tt.HasText("重试") {
			t.Fatalf("LoadAsterDoc error or retry missing: %v", tt.Texts())
		}

		failDoc = false
		if err := tt.Click("重试"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.designDoc == nil || a.designDoc.Revision != 4 {
			t.Fatalf("retry did not load AsterDoc: %+v", a.designDoc)
		}
		if tt.HasText("INTERNAL: asterdoc read failed") || !tt.HasText("AsterDoc 设计") {
			t.Fatalf("AsterDoc surface not restored after retry: %v", tt.Texts())
		}
	})
}

type assetRelabelSpy struct {
	runs.RunService
	calls []struct {
		runID     string
		assetID   string
		wellID    string
		timepoint string
		fovIndex  uint32
	}
	err   error
	asset runs.Asset
}

func (s *assetRelabelSpy) UpdateAssetLinkCoordinates(
	_ context.Context,
	runID, assetID, wellID, timepoint string,
	fovIndex uint32,
) (runs.Asset, error) {
	s.calls = append(s.calls, struct {
		runID     string
		assetID   string
		wellID    string
		timepoint string
		fovIndex  uint32
	}{runID, assetID, wellID, timepoint, fovIndex})
	if s.err != nil {
		return runs.Asset{}, s.err
	}
	updated := s.asset
	updated.ID = assetID
	updated.Link.RunID = runID
	updated.Link.WellID = wellID
	updated.Link.Timepoint = timepoint
	updated.Link.FOVIndex = fovIndex
	return updated, nil
}

// ListTimers keeps the run bench alive on tests whose fake predates the
// timer bridge: no persisted timers, never a nil-embedded panic.
func (s *assetRelabelSpy) ListTimers(context.Context, string) ([]runs.TimerRecord, error) {
	return nil, nil
}

func newRelabelTestApp(spy runs.RunService) *app {
	unlabelled := runs.Asset{
		ID:           "asset-1",
		RootID:       "design-data-design-1",
		RelativePath: "images/sample.tif",
		MediaType:    "image/tiff",
		Link:         runs.AssetLink{RunID: "run-1", TaskKey: "task-1"},
	}
	return &app{
		runSvc: spy,
		snap: runs.RunSnapshot{
			RunID:    "run-1",
			DesignID: "design-1",
			Tasks: []runs.RunTask{
				{Key: "task-1", Title: "显微成像", AssayID: "assay", Timepoint: "24 h"},
			},
			DesignData: `{"mygo":{"instances":{"step":[{"id":"plate","type":"plate_12","props":{"rows":3,"cols":4}}]},"assays":{"assay":{"plateInstanceId":"plate","timepoints":["6 h","24 h"]}}}}`,
		},
		runAssets: []runs.Asset{unlabelled},
	}
}

func TestRelabelUnlabelledAssetUpdatesRunAssetsAndMatrix(t *testing.T) {
	spy := &assetRelabelSpy{}
	a := newRelabelTestApp(spy)
	spy.asset = a.runAssets[0]

	projected := a.runAssetsForTask("task-1")
	a.openAssetLinkForm("task-1", projected[0])
	if !a.linkDialogOpen || a.linkDialogAssetID != "asset-1" {
		t.Fatalf("openAssetLinkForm did not enter relabel mode: open=%v asset=%q", a.linkDialogOpen, a.linkDialogAssetID)
	}
	if a.linkDialogWell != "A1" || a.linkDialogTimepoint != "24 h" || a.linkDialogFOV != "1" {
		t.Fatalf("relabel prefill = (%q, %q, %q), want (A1, 24 h, 1)", a.linkDialogWell, a.linkDialogTimepoint, a.linkDialogFOV)
	}

	a.linkDialogWell = "B2"
	a.linkDialogTimepoint = "24 h"
	a.linkDialogFOV = "2"
	a.submitAssetLinkForm(a.relabelRunAsset)

	if len(spy.calls) != 1 {
		t.Fatalf("UpdateAssetLinkCoordinates calls = %d, want 1", len(spy.calls))
	}
	call := spy.calls[0]
	if call.runID != "run-1" || call.assetID != "asset-1" || call.wellID != "B2" || call.timepoint != "24 h" || call.fovIndex != 2 {
		t.Fatalf("UpdateAssetLinkCoordinates call = %+v", call)
	}
	if a.linkDialogOpen || a.linkDialogErr != "" || a.linkDialogAssetID != "" {
		t.Fatalf("form did not close/clear on success: open=%v err=%q asset=%q", a.linkDialogOpen, a.linkDialogErr, a.linkDialogAssetID)
	}
	if len(a.runAssets) != 1 || a.runAssets[0].Link.WellID != "B2" || a.runAssets[0].Link.Timepoint != "24 h" || a.runAssets[0].Link.FOVIndex != 2 {
		t.Fatalf("runAssets not refreshed: %+v", a.runAssets)
	}

	vm := runworkspace.BuildViewModel(a.snap, nil, runworkspace.Projection{})
	rv := &runworkspace.RecordingView{
		Kind:   runworkspace.ViewGallery,
		Task:   vm.TaskByKey["task-1"],
		Assets: a.runAssetsForTask("task-1"),
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("B2") {
		t.Fatalf("matrix did not show relabelled well B2: %q", tt.Texts())
	}
	if tt.HasText("未标注") {
		t.Fatalf("matrix still showed unlabelled after relabel: %q", tt.Texts())
	}
}

func TestRelabelUnlabelledAssetRejectsInvalidCoordinateWithoutMutatingRunAssets(t *testing.T) {
	spy := &assetRelabelSpy{}
	a := newRelabelTestApp(spy)
	spy.asset = a.runAssets[0]

	a.openAssetLinkForm("task-1", a.runAssetsForTask("task-1")[0])
	a.linkDialogFOV = "0"
	a.submitAssetLinkForm(a.relabelRunAsset)

	if len(spy.calls) != 0 {
		t.Fatalf("local invalid FOV still called Core %d times", len(spy.calls))
	}
	if got := a.runAssets[0].Link; got.WellID != "" || got.Timepoint != "" || got.FOVIndex != 0 {
		t.Fatalf("runAssets mutated after invalid local FOV: %+v", a.runAssets)
	}
	if a.linkDialogErr == "" || !a.linkDialogOpen {
		t.Fatalf("form did not stay open with error on invalid local FOV: open=%v err=%q", a.linkDialogOpen, a.linkDialogErr)
	}

	spy.err = fmt.Errorf("INVALID: well B2 is not in frozen plate")
	a.linkDialogWell = "B2"
	a.linkDialogFOV = "1"
	a.submitAssetLinkForm(a.relabelRunAsset)

	if len(spy.calls) != 1 {
		t.Fatalf("Core INVALID calls = %d, want 1", len(spy.calls))
	}
	if got := a.runAssets[0].Link; got.WellID != "" || got.Timepoint != "" || got.FOVIndex != 0 {
		t.Fatalf("runAssets mutated after Core INVALID: %+v", a.runAssets)
	}
	if !strings.Contains(a.linkDialogErr, "INVALID:") || !a.linkDialogOpen {
		t.Fatalf("form did not retain Core INVALID error open: open=%v err=%q", a.linkDialogOpen, a.linkDialogErr)
	}
}

func TestRelabelUnlabelledAssetSurfacesNotFoundWithoutMutatingRunAssets(t *testing.T) {
	spy := &assetRelabelSpy{
		err: fmt.Errorf("NOT_FOUND: asset link asset-1 not found"),
	}
	a := newRelabelTestApp(spy)
	spy.asset = a.runAssets[0]

	a.openAssetLinkForm("task-1", a.runAssetsForTask("task-1")[0])
	a.linkDialogWell = "A1"
	a.linkDialogTimepoint = "24 h"
	a.linkDialogFOV = "1"
	a.submitAssetLinkForm(a.relabelRunAsset)

	if len(spy.calls) != 1 {
		t.Fatalf("Core NOT_FOUND calls = %d, want 1", len(spy.calls))
	}
	if got := a.runAssets[0].Link; got.WellID != "" || got.Timepoint != "" || got.FOVIndex != 0 {
		t.Fatalf("runAssets mutated after Core NOT_FOUND: %+v", a.runAssets)
	}
	if !strings.Contains(a.linkDialogErr, "NOT_FOUND:") || !a.linkDialogOpen {
		t.Fatalf("form did not retain Core NOT_FOUND error open: open=%v err=%q", a.linkDialogOpen, a.linkDialogErr)
	}
}

type notebookSaveFailSpy struct {
	runs.RunService
	failRemaining int
	saveCalls     int
	opIDs         []string
	snap          runs.RunSnapshot
}

func (s *notebookSaveFailSpy) SaveNotebook(_ context.Context, input runs.NotebookInput) (runs.RunSnapshot, error) {
	s.saveCalls++
	s.opIDs = append(s.opIDs, input.OperationID)
	if s.failRemaining > 0 {
		s.failRemaining--
		return runs.RunSnapshot{}, fmt.Errorf("CONFLICT: notebook write rejected")
	}
	s.snap.Revision++
	s.snap.Events = append(s.snap.Events, runs.RunEvent{
		ID:         fmt.Sprintf("evt-note-%d", s.saveCalls),
		Stage:      "notebook",
		EventType:  "observation",
		Content:    input.Markdown,
		RecordedAt: fmt.Sprintf("2026-11-16T00:00:0%dZ", s.saveCalls),
	})
	return s.snap, nil
}

// RecordProjection is unstubbed by design: the record board is out of scope
// for this save-path test, so the fetch surfaces its refusal (UX-P5) instead
// of calling into the nil embedded backend.
func (s *notebookSaveFailSpy) RecordProjection(context.Context, runs.RecordProjectionInput) (string, error) {
	return "", fmt.Errorf("record not stubbed")
}

func (s *notebookSaveFailSpy) WorkflowTree(context.Context, string) (string, error) {
	return "", nil
}

func (s *notebookSaveFailSpy) ListAssetLinks(context.Context, string) ([]runs.Asset, error) {
	return nil, nil
}

// ListTimers keeps the run bench alive on tests whose fake predates the
// timer bridge: no persisted timers, never a nil-embedded panic.
func (s *notebookSaveFailSpy) ListTimers(context.Context, string) ([]runs.TimerRecord, error) {
	return nil, nil
}

func TestNotebookSaveFailureVisibleAndRetrySaves(t *testing.T) {
	t.Setenv("LABWORKBENCH_DATA_DIR", t.TempDir())
	spy := &notebookSaveFailSpy{
		failRemaining: 1,
		snap: runs.RunSnapshot{
			RunID:          "run-note-fail",
			Title:          "Notebook Failure Test",
			Status:         "running",
			DesignRevision: 3,
		},
	}
	a := &app{
		runSvc: spy,
		snap:   spy.snap,
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1360, 880)
	tt.Frame()

	if !tt.HasText("草稿") {
		t.Fatalf("initial unsaved notebook missing 草稿 state word: %q", tt.Texts())
	}

	if err := tt.Click("编辑"); err != nil {
		t.Fatalf("switch to 编辑: %v", err)
	}
	tt.Frame()
	a.noteDraft = "# 失败重试测试\n\n- 细胞状态稳定"
	a.markNotebookDirty()
	tt.Frame()
	if !tt.HasText("未保存") {
		t.Fatalf("dirty notebook missing 未保存 state word: %q", tt.Texts())
	}

	if err := tt.Click("保存笔记"); err != nil {
		t.Fatalf("click 保存笔记: %v", err)
	}
	tt.Frame()
	if spy.saveCalls != 1 {
		t.Fatalf("SaveNotebook calls = %d, want 1", spy.saveCalls)
	}
	if !tt.HasText("未保存（CONFLICT: notebook write rejected）") || !tt.HasText("重试") {
		t.Fatalf("save failure reason or retry button missing: %q", tt.Texts())
	}
	if a.noteDraft != "# 失败重试测试\n\n- 细胞状态稳定" || !a.noteDirty {
		t.Fatalf("failed save mutated draft=%q dirty=%v", a.noteDraft, a.noteDirty)
	}

	a.noteState = "saving"
	tt.Frame()
	if !tt.HasText("保存中") {
		t.Fatalf("saving state missing 保存中 label: %q", tt.Texts())
	}
	a.noteState = "error"
	tt.Frame()

	if err := tt.Click("重试"); err != nil {
		t.Fatalf("click 重试: %v", err)
	}
	tt.Frame()
	if spy.saveCalls != 2 {
		t.Fatalf("SaveNotebook calls after retry = %d, want 2", spy.saveCalls)
	}
	if tt.HasText("CONFLICT: notebook write rejected") {
		t.Fatalf("error banner remained after retry succeeded: %q", tt.Texts())
	}
	if !tt.HasText("已保存") || a.noteDirty {
		t.Fatalf("notebook did not reach 已保存 after retry: dirty=%v texts=%q", a.noteDirty, tt.Texts())
	}
}

func TestNotebookDraftSurvivesRunSwitchAndRestart(t *testing.T) {
	t.Setenv("LABWORKBENCH_DATA_DIR", t.TempDir())
	snapA := runs.RunSnapshot{
		RunID:          "run-a-draft",
		Title:          "Run A",
		Status:         "running",
		Revision:       1,
		DesignRevision: 1,
	}
	snapB := runs.RunSnapshot{
		RunID:          "run-b-saved",
		Title:          "Run B",
		Status:         "running",
		Revision:       3,
		DesignRevision: 2,
		Events: []runs.RunEvent{
			{ID: "evt-b", Stage: "notebook", Content: "Run B 已保存笔记", RecordedAt: "2026-11-16T00:00:00Z"},
		},
	}

	a := &app{snap: snapA}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1360, 880)
	tt.Frame()

	if err := tt.Click("编辑"); err != nil {
		t.Fatalf("switch to 编辑: %v", err)
	}
	tt.Frame()
	a.noteDraft = "Run A 未保存草稿"
	a.markNotebookDirty()
	tt.Frame()
	if !tt.HasText("未保存") {
		t.Fatalf("Run A dirty state not rendered: %q", tt.Texts())
	}

	// Switch to Run B: Run B loads its own persisted note, while Run A's draft is kept.
	a.applySnap(snapB)
	tt.Frame()
	if a.noteDraft != "Run B 已保存笔记" || a.noteDirty || !tt.HasText("已保存") {
		t.Fatalf("Run B state wrong: draft=%q dirty=%v texts=%q", a.noteDraft, a.noteDirty, tt.Texts())
	}

	// Switch back to Run A: Run A restores its dirty draft instead of clearing it.
	a.applySnap(snapA)
	tt.Frame()
	if a.noteDraft != "Run A 未保存草稿" || !a.noteDirty || !tt.HasText("未保存") {
		t.Fatalf("Run A draft was lost on Run switch: draft=%q dirty=%v texts=%q", a.noteDraft, a.noteDirty, tt.Texts())
	}

	// Simulate full process restart with empty in-memory state and same LABWORKBENCH_DATA_DIR.
	spy := &notebookSaveFailSpy{snap: snapA}
	restarted := &app{runSvc: spy, snap: snapA}
	ttRestart := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, restarted)
	}, 1360, 880)
	ttRestart.Frame()
	if restarted.noteDraft != "Run A 未保存草稿" || !restarted.noteDirty {
		t.Fatalf("Run A draft was lost across restart: draft=%q dirty=%v", restarted.noteDraft, restarted.noteDirty)
	}
	if !ttRestart.HasText("已从本地草稿恢复未保存修改。") || !ttRestart.HasText("未保存") {
		t.Fatalf("restarted recovery banner/state missing: %q", ttRestart.Texts())
	}

	// Saving clears the local disk draft so a subsequent restart starts clean at 已保存.
	if err := ttRestart.Click("保存笔记"); err != nil {
		t.Fatalf("save recovered draft: %v", err)
	}
	ttRestart.Frame()
	if restarted.noteDirty || !ttRestart.HasText("已保存") {
		t.Fatalf("recovered draft failed to save: dirty=%v texts=%q", restarted.noteDirty, ttRestart.Texts())
	}
	afterSave := &app{snap: spy.snap}
	ttClean := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, afterSave)
	}, 1360, 880)
	ttClean.Frame()
	if afterSave.noteDirty || ttClean.HasText("已从本地草稿恢复未保存修改。") || !ttClean.HasText("已保存") {
		t.Fatalf("cleared disk draft still recovered: dirty=%v texts=%q", afterSave.noteDirty, ttClean.Texts())
	}
}

func TestNotebookStateWordsChineseLabelsAndProgressiveDisclosure(t *testing.T) {
	t.Setenv("LABWORKBENCH_DATA_DIR", t.TempDir())
	val := 0.85
	a := &app{
		snap: runs.RunSnapshot{
			RunID:          "run-uuid-999",
			Title:          "CCK-8 实验",
			Status:         "running",
			DesignRevision: 7,
			Tasks: []runs.RunTask{
				{Key: "task-od-raw", Title: "酶标仪读数", Timepoint: "24 h"},
			},
			Events: []runs.RunEvent{
				{
					ID:         "evt-uuid-123",
					EventType:  "measurement",
					TaskKey:    "task-od-raw",
					BatchID:    "batch-uuid-456",
					RecordedAt: "2026-11-16T01:02:03Z",
					ResultCells: []runs.ResultCell{
						{Row: 0, Column: 0, SampleID: "S1", Field: "od450", Value: &val, Status: "raw"},
					},
				},
			},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1360, 880)
	tt.Frame()

	for _, want := range []string{"草稿", "设计版本 7", "设计版本 7 · 2 条", "测量记录 · 酶标仪读数 · 24 h · 1 个单元格"} {
		if !tt.HasText(want) {
			t.Fatalf("missing expected Chinese text %q in %q", want, tt.Texts())
		}
	}
	for _, forbid := range []string{"Document Revision", "Design rev", "execution_event evt-uuid-123", "run run-uuid-999", "batch-uuid-456"} {
		if tt.HasText(forbid) {
			t.Fatalf("unexpected mixed-language or raw ID %q in default view: %q", forbid, tt.Texts())
		}
	}

	// Raw identifier is reachable in 1 click via the timeline detail disclosure.
	if err := tt.Click("来源详情"); err != nil {
		t.Fatalf("click 来源详情: %v", err)
	}
	tt.Frame()
	if !tt.HasText("原始来源：run run-uuid-999") {
		t.Fatalf("clicking 来源详情 did not reveal raw run identifier: %q", tt.Texts())
	}
	view := a.noteView()
	view.expanded["event:evt-uuid-123"] = true
	tt.Frame()
	if !tt.HasText("原始来源：execution_event evt-uuid-123") {
		t.Fatalf("expanded detail did not reveal raw event identifier: %q", tt.Texts())
	}

	// Read-only empty state uses §3 state word and matches 实时/编辑 tabs.
	if err := tt.Click("阅读"); err != nil {
		t.Fatalf("switch to 阅读: %v", err)
	}
	tt.Frame()
	if !tt.HasText("未开始") || !tt.HasText("尚未记录笔记内容") {
		t.Fatalf("read-only empty state missing §3 word or Chinese guidance: %q", tt.Texts())
	}
	if tt.HasText("还没有内容") {
		t.Fatalf("legacy empty state text still present: %q", tt.Texts())
	}
}

type readinessTestService struct {
	runs.DesignService
	designs         map[string]runs.DesignSnapshot
	docs            map[string]runs.AsterDocSnapshot
	failAssess      bool
	assessCalls     int
	failCreateRun   bool
	createRunCalls  int
	failApply       bool
	applyCalls      int
	lastApply       runs.AsterDocCommandInput
	assessState     string
}

func (s *readinessTestService) LoadDesign(_ context.Context, id string) (runs.DesignSnapshot, error) {
	return s.designs[id], nil
}

func (s *readinessTestService) LoadAsterDoc(_ context.Context, id string) (runs.AsterDocSnapshot, error) {
	return s.docs[id], nil
}

func (s *readinessTestService) AssessAsterDoc(_ context.Context, id string, rev uint64) (runs.AsterDocAssessment, error) {
	s.assessCalls++
	if s.failAssess {
		return runs.AsterDocAssessment{}, fmt.Errorf("INTERNAL: assessment service timeout for %s rev %d", id, rev)
	}
	st := s.assessState
	if st == "" {
		st = "ready"
	}
	return runs.AsterDocAssessment{DesignID: id, Revision: rev, State: st}, nil
}

func (s *readinessTestService) ApplyAsterDocCommand(_ context.Context, in runs.AsterDocCommandInput) (runs.AsterDocSnapshot, error) {
	s.applyCalls++
	s.lastApply = in
	if s.failApply {
		return runs.AsterDocSnapshot{}, fmt.Errorf("CONFLICT: expected revision %d stale END_OF_APPLY_ERR", in.ExpectedRevision)
	}
	doc := s.docs[in.DesignID]
	doc.Revision++
	s.docs[in.DesignID] = doc
	return doc, nil
}

func (s *readinessTestService) CreateRunFromAsterDoc(_ context.Context, in runs.AsterDocRunInput) (runs.RunSnapshot, error) {
	s.createRunCalls++
	if s.failCreateRun {
		return runs.RunSnapshot{}, fmt.Errorf("CREATE_RUN_CONFLICT: protocol binding mismatch across multiple assay stages and timepoints END_OF_CREATE_RUN_ERROR")
	}
	return runs.RunSnapshot{RunID: "run-created", DesignID: in.DesignID, Title: "Created Run", Revision: 1, Status: "running"}, nil
}

func TestDesignReadinessStatesAndRetryOnFailure(t *testing.T) {
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	svc := &readinessTestService{
		designs:    map[string]runs.DesignSnapshot{"d1": {DesignID: "d1", Title: "D1", Revision: 2}},
		docs:       map[string]runs.AsterDocSnapshot{"d1": {DesignID: "d1", Revision: 2, Source: string(raw)}},
		failAssess: true,
	}
	a := &app{designSvc: svc, shell: &shell.Shell{Page: shell.Design}}
	a.loadDesign("d1")

	if a.readiness != nil || !strings.Contains(a.readinessErr, "INTERNAL: assessment service timeout") {
		t.Fatalf("unexpected readiness after failure: readiness=%+v err=%q", a.readiness, a.readinessErr)
	}

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		designPage(c, a)
	}, 1360, 880)
	tt.Frame()

	if !tt.HasText("未解析") || !tt.HasText("INTERNAL: assessment service timeout for d1 rev 2") || !tt.HasText("重试") {
		t.Fatalf("readiness failure missing state/reason/retry: %v", tt.Texts())
	}

	// Preflight dialog also shows 未解析, reason, and 重试.
	a.preflightOpen = true
	tt.Frame()
	if !tt.HasText("未解析 · 无法获取 Core 校验结果") || !tt.HasText("INTERNAL: assessment service timeout for d1 rev 2") {
		t.Fatalf("preflight dialog missing reason on assessment failure: %v", tt.Texts())
	}

	svc.failAssess = false
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if svc.assessCalls < 2 || a.readiness == nil || a.readinessErr != "" {
		t.Fatalf("retry did not recover readiness: calls=%d readiness=%+v err=%q", svc.assessCalls, a.readiness, a.readinessErr)
	}
	if !tt.HasText("就绪") {
		t.Fatalf("top bar missing 就绪 after retry: %v", tt.Texts())
	}

	// 校验中 state disables 创建 Run.
	a.preflightOpen = false
	a.readinessLoading = true
	tt.Frame()
	if !tt.HasText("校验中") {
		t.Fatalf("missing 校验中 state word: %v", tt.Texts())
	}
	_ = tt.Click("创建 Run")
	tt.Frame()
	if a.preflightOpen {
		t.Fatal("创建 Run opened preflight while 校验中")
	}
}

func TestDesignCreateRunCommandErrorsAndSilentNoOp(t *testing.T) {
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	// Empty service/document is never a silent no-op.
	empty := &app{}
	if empty.applyDesignRaw(map[string]any{"type": "update_module"}) {
		t.Fatal("applyDesignRaw succeeded with nil service")
	}
	if empty.designDocErr == "" {
		t.Fatal("applyDesignRaw failed silently with nil service")
	}
	empty.designSvc = &readinessTestService{}
	empty.designDocErr = ""
	if empty.applyDesignRaw(map[string]any{"type": "update_module"}) {
		t.Fatal("applyDesignRaw succeeded with nil designDoc")
	}
	if empty.designDocErr == "" {
		t.Fatal("applyDesignRaw failed silently with nil designDoc")
	}

	svc := &readinessTestService{
		designs:       map[string]runs.DesignSnapshot{"d1": {DesignID: "d1", Title: "D1", Revision: 2}},
		docs:          map[string]runs.AsterDocSnapshot{"d1": {DesignID: "d1", Revision: 2, Source: string(raw)}},
		failCreateRun: true,
		failApply:     true,
	}
	a := &app{designSvc: svc, shell: &shell.Shell{Page: shell.Design}}
	a.loadDesign("d1")

	a.createRunFromDesignDoc()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		designPage(c, a)
	}, 1360, 880)
	tt.Frame()
	if !tt.HasText("END_OF_CREATE_RUN_ERROR") || !tt.HasText("重试") {
		t.Fatalf("createRunErr truncated or missing retry: %v", tt.Texts())
	}

	svc.failCreateRun = false
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if svc.createRunCalls != 2 || a.createRunErr != "" || a.shell.Page != shell.Run {
		t.Fatalf("createRun retry failed: calls=%d err=%q page=%v", svc.createRunCalls, a.createRunErr, a.shell.Page)
	}

	// Command failure banner also shows full message + 重试.
	a.shell.Page = shell.Design
	if a.applyDesignRaw(map[string]any{"type": "update_module", "moduleId": "step-seed", "properties": map[string]any{"title": "X"}}) {
		t.Fatal("expected applyDesignRaw failure")
	}
	tt.Frame()
	if !tt.HasText("END_OF_APPLY_ERR") || !tt.HasText("重试") {
		t.Fatalf("designDocErr missing full text or retry: %v", tt.Texts())
	}
	svc.failApply = false
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if svc.applyCalls != 2 || a.designDocErr != "" {
		t.Fatalf("command retry failed: calls=%d err=%q", svc.applyCalls, a.designDocErr)
	}
}

func TestDesignStepSwitchKeepsAndWarnsAboutPropertyDraft(t *testing.T) {
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	svc := &readinessTestService{
		designs: map[string]runs.DesignSnapshot{"d1": {DesignID: "d1", Title: "D1", Revision: 2}},
		docs:    map[string]runs.AsterDocSnapshot{"d1": {DesignID: "d1", Revision: 2, Source: string(raw)}},
	}
	a := &app{designSvc: svc, shell: &shell.Shell{Page: shell.Design}}
	a.loadDesign("d1")

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		asterDesignPage(c, a)
	}, 1360, 880)
	tt.Frame()

	if err := tt.Click("seed"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("未保存") {
		t.Fatalf("unexpected 未保存 before edit: %v", tt.Texts())
	}

	a.canvasTitle = "seed-edited-draft"
	tt.Frame()
	if !tt.HasText("未保存") {
		t.Fatalf("missing 未保存 hint after editing drawer: %v", tt.Texts())
	}

	// Switching to another step keeps the draft and warns the operator.
	if err := tt.Click("read"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.designSelected != "step-read" {
		t.Fatalf("selected = %q, want step-read", a.designSelected)
	}
	if !tt.HasText("未保存") || !tt.HasText("已保留") || !tt.HasText("切回草稿") {
		t.Fatalf("missing draft switch warning: %v", tt.Texts())
	}

	// Switching back restores the exact unsaved draft.
	if err := tt.Click("seed"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.designSelected != "step-seed" || a.canvasTitle != "seed-edited-draft" {
		t.Fatalf("draft dropped on step switch: selected=%q title=%q", a.designSelected, a.canvasTitle)
	}

	if err := tt.Click("保存属性"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if svc.applyCalls != 1 || !strings.Contains(svc.lastApply.Command, "seed-edited-draft") {
		t.Fatalf("save did not submit edited draft: calls=%d cmd=%q", svc.applyCalls, svc.lastApply.Command)
	}
	if len(a.stepDrafts) != 0 {
		t.Fatalf("stepDrafts not cleared after save: %+v", a.stepDrafts)
	}
}

func TestDesignerListViewFullErrorRetryAndLoadingState(t *testing.T) {
	d := designer.New("design-1", "Fallback List", 2, nil)
	saveCalls := 0
	d.Callbacks.SaveDesign = func(_ context.Context, in runs.DesignSaveInput) (runs.DesignSnapshot, error) {
		saveCalls++
		return runs.DesignSnapshot{DesignID: in.DesignID, Revision: in.ExpectedRevision + 1}, nil
	}
	d.Touch()
	d.SaveState = designer.SaveError
	d.Error = "CONFLICT: design revision 2 differs from Core revision 4 because another session committed changes TAIL_END_UNCLIPPED"

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		designer.ListView(c, d)
	}, 1360, 880)
	tt.Frame()

	if !tt.HasText("未保存") || !tt.HasText("TAIL_END_UNCLIPPED") || !tt.HasText("重试") {
		t.Fatalf("ListView missing full error, 未保存, or 重试: %v", tt.Texts())
	}

	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if saveCalls != 1 || d.Error != "" || d.SaveState != designer.SaveSaved {
		t.Fatalf("ListView retry failed: calls=%d err=%q state=%s", saveCalls, d.Error, d.SaveState)
	}

	d.Loading = true
	tt.Frame()
	if !tt.HasText("校验中") {
		t.Fatalf("ListView missing 校验中 while loading: %v", tt.Texts())
	}
}

func TestGridCommitFailureSetsStateErrorAndShowsBannerWithRetry(t *testing.T) {
	t.Setenv("LABWORKBENCH_DATA_DIR", t.TempDir())
	schema := resultgrid.Schema{
		ID: "cck8.plate96.od450.v1", Version: 1,
		RowLabels: []string{"A"}, ColumnLabels: []string{"1"},
		Fields: []resultgrid.Field{{Key: "od450", Label: "OD450", Unit: "AU"}},
	}
	grid := resultgrid.New(schema, nil)
	task := &runworkspace.RunTaskViewModel{TaskKey: "t-1", StepTitle: "酶标仪读数"}
	saveCalls := 0
	rv := &runworkspace.RecordingView{
		RunID: "run-commit-err",
		Kind:  runworkspace.ViewPlate,
		Grid:  grid,
		Task:  task,
		OnSaveBatch: func([]resultgrid.Cell) {
			saveCalls++
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 1360, 880)
	tt.Frame()

	// Pending invalid draft blocks view switch and sets StateError + banner + 重试保存.
	draft, ok := grid.BeginEdit(0, 0)
	if !ok {
		t.Fatal("BeginEdit failed")
	}
	*draft = "not-a-number"
	grid.MarkEditDirty()
	if err := tt.Click("表格"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if rv.Kind != runworkspace.ViewPlate {
		t.Fatalf("view switched despite invalid pending edit: kind=%q", rv.Kind)
	}
	if grid.State != resultgrid.StateError || grid.Error != "请输入数字" {
		t.Fatalf("view switch commit failure did not set StateError: state=%q err=%q", grid.State, grid.Error)
	}
	if !tt.HasText("保存失败") || !tt.HasText("未保存 · 请输入数字") || !tt.HasText("重试保存") {
		t.Fatalf("banner/retry missing after view switch commit failure: %v", tt.Texts())
	}
	if tt.HasText("检查目标目录权限") {
		t.Fatalf("unexpected generic directory permission hint on commit failure: %v", tt.Texts())
	}

	// Clicking 保存读数 / 重试保存 with invalid pending edit also fails with StateError and never invokes OnSaveBatch.
	grid.State = resultgrid.StateDraft
	grid.Error = ""
	tt.Frame()
	if err := tt.Click("保存读数"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if saveCalls != 0 {
		t.Fatalf("OnSaveBatch called despite commit error: calls=%d", saveCalls)
	}
	if grid.State != resultgrid.StateError || grid.Error != "请输入数字" {
		t.Fatalf("saveNow commit failure did not set StateError: state=%q err=%q", grid.State, grid.Error)
	}
	if !tt.HasText("未保存 · 请输入数字") || !tt.HasText("重试保存") {
		t.Fatalf("banner/retry missing after save commit failure: %v", tt.Texts())
	}
}

type saveBatchCauseSpy struct {
	runs.RunService
	observedGridState resultgrid.SaveState
	saveErr           error
	accepted          int
	loadErr           error
	snap              runs.RunSnapshot
}

func (s *saveBatchCauseSpy) SaveReadoutsBatch(_ context.Context, input runs.BatchReadoutInput) (runs.BatchReadoutResult, error) {
	if s.saveErr != nil {
		return runs.BatchReadoutResult{}, s.saveErr
	}
	accepted := len(input.Cells)
	if s.accepted >= 0 {
		accepted = s.accepted
	}
	s.snap.Revision++
	return runs.BatchReadoutResult{RunID: input.RunID, Revision: s.snap.Revision, Accepted: accepted}, nil
}

func (s *saveBatchCauseSpy) LoadRun(context.Context, string) (runs.RunSnapshot, error) {
	if s.loadErr != nil {
		return runs.RunSnapshot{}, s.loadErr
	}
	return s.snap, nil
}

func (s *saveBatchCauseSpy) WorkflowTree(context.Context, string) (string, error) {
	return "", nil
}

func (s *saveBatchCauseSpy) ListAssetLinks(context.Context, string) ([]runs.Asset, error) {
	return nil, nil
}

// ListTimers keeps the run bench alive on tests whose fake predates the
// timer bridge: no persisted timers, never a nil-embedded panic.
func (s *saveBatchCauseSpy) ListTimers(context.Context, string) ([]runs.TimerRecord, error) {
	return nil, nil
}

type saveBatchObserver struct {
	*saveBatchCauseSpy
	grid *resultgrid.Grid
}

func (o *saveBatchObserver) SaveReadoutsBatch(ctx context.Context, input runs.BatchReadoutInput) (runs.BatchReadoutResult, error) {
	o.observedGridState = o.grid.State
	return o.saveBatchCauseSpy.SaveReadoutsBatch(ctx, input)
}

func TestSaveBatchPassesThroughStateSavingAndDistinguishesCauses(t *testing.T) {
	t.Setenv("LABWORKBENCH_DATA_DIR", t.TempDir())
	schema := resultgrid.Schema{
		ID: "cck8.plate96.od450.v1", Version: 1,
		RowLabels: []string{"A"}, ColumnLabels: []string{"1", "2"},
		Fields: []resultgrid.Field{{Key: "od450", Label: "OD450", Unit: "AU"}},
	}
	grid := resultgrid.New(schema, nil)
	v := 0.72
	_ = grid.SetCell(resultgrid.Cell{Row: 0, Column: 0, SampleID: "S1", Field: "od450", Value: &v, Status: "raw"})
	task := &runworkspace.RunTaskViewModel{TaskKey: "t-1", StepTitle: "酶标仪读数"}

	spy := &saveBatchCauseSpy{
		accepted: -1,
		snap: runs.RunSnapshot{
			RunID: "run-causes", Revision: 1, Status: "running",
			Tasks: []runs.RunTask{{Key: "t-1", Title: "酶标仪读数"}},
		},
	}
	obs := &saveBatchObserver{saveBatchCauseSpy: spy, grid: grid}
	a := &app{
		runSvc: obs,
		snap:   spy.snap,
		ws:     &runui.Workspace{Selected: "t-1", Grid: grid},
	}

	// Verify OnSaveBatch passes through StateSaving.
	a.startSaveBatch(grid, "t-1", grid.Cells)
	if obs.observedGridState != resultgrid.StateSaving {
		t.Fatalf("SaveReadoutsBatch did not observe StateSaving: got %q", obs.observedGridState)
	}

	// Verify RecordingView renders 保存中… and disables duplicate save clicks when StateSaving.
	savingCalls := 0
	savingGrid := resultgrid.New(schema, nil)
	savingGrid.State = resultgrid.StateSaving
	rv := &runworkspace.RecordingView{
		RunID: "run-causes", Kind: runworkspace.ViewPlate, Grid: savingGrid, Task: task,
		OnSaveBatch: func([]resultgrid.Cell) { savingCalls++ },
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 1360, 880)
	tt.Frame()
	if !tt.HasText("保存中…") {
		t.Fatalf("StateSaving did not render 保存中…: %v", tt.Texts())
	}
	_ = tt.Click("保存读数")
	tt.Frame()
	if savingCalls != 0 {
		t.Fatalf("disabled button fired OnSaveBatch during StateSaving")
	}

	// Cause 1: revision conflict.
	spy.saveErr = fmt.Errorf("CONFLICT: expected revision 1, got 2")
	a.saveBatch(schema, grid.Cells)
	rv.Grid = grid
	tt.Frame()
	if grid.State != resultgrid.StateError || !tt.HasText("CONFLICT: expected revision 1, got 2") || !tt.HasText("修订冲突") || !tt.HasText("重试保存") {
		t.Fatalf("revision conflict banner incorrect: state=%q texts=%v", grid.State, tt.Texts())
	}
	if tt.HasText("检查目标目录权限") {
		t.Fatalf("revision conflict misattributed to directory permissions: %v", tt.Texts())
	}

	// Cause 2: rejected cells.
	spy.saveErr = nil
	spy.accepted = 1
	a.saveBatch(schema, grid.Cells)
	tt.Frame()
	if grid.State != resultgrid.StateError || !tt.HasText("1/2") || !tt.HasText("单元格") || !tt.HasText("重试保存") {
		t.Fatalf("rejected cells banner incorrect: state=%q texts=%v", grid.State, tt.Texts())
	}
	if tt.HasText("检查目标目录权限") {
		t.Fatalf("rejected cells misattributed to directory permissions: %v", tt.Texts())
	}

	// Cause 3: Core write succeeded, LoadRun refresh failed.
	spy.accepted = -1
	spy.loadErr = fmt.Errorf("INTERNAL: sqlite busy during refresh")
	a.saveBatch(schema, grid.Cells)
	tt.Frame()
	if grid.State != resultgrid.StateError || !tt.HasText("已写入 Core") || !tt.HasText("INTERNAL: sqlite busy during refresh") || !tt.HasText("重试保存") {
		t.Fatalf("refresh failure banner incorrect: state=%q texts=%v", grid.State, tt.Texts())
	}
	if tt.HasText("未保存 · ") || tt.HasText("检查目标目录权限") {
		t.Fatalf("refresh failure lied as unsaved or directory permissions: %v", tt.Texts())
	}
}

type assetFailSpy struct {
	runs.RunService
	listErr error
	assets  []runs.Asset
}

func (s *assetFailSpy) ListAssetLinks(context.Context, string) ([]runs.Asset, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.assets, nil
}

func (s *assetFailSpy) WorkflowTree(context.Context, string) (string, error) {
	return "", nil
}

// ListTimers keeps the run bench alive on tests whose fake predates the
// timer bridge: no persisted timers, never a nil-embedded panic.
func (s *assetFailSpy) ListTimers(context.Context, string) ([]runs.TimerRecord, error) {
	return nil, nil
}

func TestAssetOpenFailureShowsPageBannerWithRetryAndLinkErrReachable(t *testing.T) {
	root := t.TempDir()
	spy := &assetFailSpy{
		assets: []runs.Asset{{
			ID:           "asset-1",
			RootID:       "design-data-d1",
			RelativePath: "images/missing.tif",
			MediaType:    "image/tiff",
			Size:         3,
			SHA256:       strings.Repeat("a", 64),
			Availability: "available",
			Link:         runs.AssetLink{RunID: "run-asset", TaskKey: "t-img", WellID: "A1", Timepoint: "24 h", FOVIndex: 1},
		}},
	}
	imgPath := filepath.Join(root, "images", "missing.tif")
	if err := os.MkdirAll(filepath.Dir(imgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Size matches (fast verify keeps Available=true so 打开 is clickable), but SHA256 mismatches when 打开 runs Verify(digest=true).
	if err := os.WriteFile(imgPath, []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := &app{
		runSvc:     spy,
		assetRoots: map[string]string{"design-data-d1": root},
		snap: runs.RunSnapshot{
			RunID:    "run-asset",
			DesignID: "d1",
			Title:    "资产测试运行",
			Status:   "running",
			Tasks:    []runs.RunTask{{Key: "t-img", Title: "显微成像", Status: "not_started"}},
		},
		runAssets: spy.assets,
		shell:     &shell.Shell{Page: shell.Run},
	}
	a.runWSVM = a.buildRunWorkspaceVM()
	a.runWS = runworkspace.NewState(a.runWSVM, runworkspace.Callbacks{
		Recording: func(c *ui.Context, task *runworkspace.RunTaskViewModel) {
			linked := a.runAssetsForTask(task.TaskKey)
			rv := runworkspace.RecordingView{
				RunID: a.snap.RunID, Kind: runworkspace.ViewGallery, Assets: linked, Task: task,
				LinkErr: a.linkErr, AssetErr: a.assetErr, OnRetryAssetErr: a.assetErrRetry,
				OnOpenAsset: func(asset runworkspace.LinkedAsset) {
					_ = a.tryOpenRunAsset(task.TaskKey, asset)
				},
			}
			rv.Render(c)
		},
	})

	// Height 1080 keeps the 1360 tier but gives headroom for the taller Bridge v2 bench.
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		contentPage(c, a)
	}, 1360, 1080)
	tt.Frame()

	if err := tt.Click("打开"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.assetErr == "" {
		t.Fatal("openRunAsset failure did not record page-level assetErr")
	}
	if !tt.HasText("无法打开文件") || !tt.HasText("重试") {
		t.Fatalf("page-level asset open failure banner or retry missing: %v", tt.Texts())
	}

	// Re-verify retry invokes open attempt again (remove root mapping to verify new reason appears on retry).
	delete(a.assetRoots, "design-data-d1")
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !strings.Contains(a.assetErr, "根目录") && !strings.Contains(a.assetErr, "root") {
		t.Fatalf("retry did not re-run openRunAsset with updated state: %q", a.assetErr)
	}

	// Also verify submitAssetLinkForm failure populates a.linkErr so the gallery link banner is reachable.
	relSpy := &assetRelabelSpy{err: fmt.Errorf("CONFLICT: coordinate conflict")}
	relApp := newRelabelTestApp(relSpy)
	relApp.openAssetLinkForm("task-1", relApp.runAssetsForTask("task-1")[0])
	relApp.linkDialogWell = "A1"
	relApp.linkDialogTimepoint = "24 h"
	relApp.linkDialogFOV = "1"
	relApp.submitAssetLinkForm(relApp.relabelRunAsset)
	if !strings.Contains(relApp.linkErr, "CONFLICT: coordinate conflict") {
		t.Fatalf("submitAssetLinkForm did not populate linkErr: %q", relApp.linkErr)
	}
}

func TestSnapshotRejectionAndConflictRenderedOnContentPage(t *testing.T) {
	a := &app{
		snap: runs.RunSnapshot{
			RunID: "run-1", Revision: 3, Status: "running",
			Tasks: []runs.RunTask{{Key: "t-1", Title: "步骤一"}},
		},
		shell: &shell.Shell{Page: shell.Run},
	}
	a.guard.Accept(3)

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		contentPage(c, a)
	}, 1360, 880)

	// Stale snapshot rejected by StaleGuard must be visibly rendered.
	a.applySnap(runs.RunSnapshot{RunID: "run-1", Revision: 2, Status: "running"})
	tt.Frame()
	if !tt.HasText("忽略过期快照") {
		t.Fatalf("stale snapshot rejection not rendered: %v", tt.Texts())
	}

	// Same-revision conflicting snapshot must be visibly rendered.
	a.applySnap(runs.RunSnapshot{RunID: "run-1", Revision: 3, Status: "completed"})
	tt.Frame()
	if !tt.HasText("检测到同修订快照冲突") {
		t.Fatalf("snapshot conflict not rendered: %v", tt.Texts())
	}
}
