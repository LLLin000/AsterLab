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
