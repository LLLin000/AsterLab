package main

import (
	"context"
	"testing"

	"asterlab-mygo-prototype/internal/designer"
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
