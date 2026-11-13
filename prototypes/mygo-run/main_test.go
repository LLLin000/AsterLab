package main

import (
	"context"
	"testing"

	"asterlab-mygo-prototype/internal/designer"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/runui"
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
