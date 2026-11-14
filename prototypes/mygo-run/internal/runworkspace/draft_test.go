package runworkspace

import (
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/resultgrid"
)

func TestGridDraftCrashRecoveryAndClear(t *testing.T) {
	t.Setenv("LABWORKBENCH_DATA_DIR", t.TempDir())
	ClearGridDraft("run-1", "task-a")

	schema := resultgrid.Schema{
		ID:           "cck8.plate96.od450.v1",
		RowLabels:    []string{"A"},
		ColumnLabels: []string{"1"},
		Fields:       []resultgrid.Field{{Key: "od450", Label: "OD450", Unit: "AU"}},
	}
	grid := resultgrid.New(schema, nil)
	if syncGridDraft("run-1", "task-a", grid, time.Unix(100, 0)) {
		t.Fatal("unexpected recovery before any draft exists")
	}

	val := 0.92
	if err := grid.SetCell(resultgrid.Cell{Row: 0, Column: 0, SampleID: "S1", Field: "od450", Value: &val, Status: "raw"}); err != nil {
		t.Fatal(err)
	}
	grid.PendingOpID = "op-retry-1"
	grid.PendingOpHash = grid.CellsDigest()
	grid.State = resultgrid.StateDraft
	syncGridDraft("run-1", "task-a", grid, time.Unix(102, 0))

	// Simulate process SIGKILL + clean restart: empty tracker map and fresh Grid.
	delete(gridDraftTrackers, "run-1\x00task-a")
	restarted := resultgrid.New(schema, nil)
	restarted.State = resultgrid.StateSaved
	if !syncGridDraft("run-1", "task-a", restarted, time.Unix(200, 0)) {
		t.Fatal("expected draft recovery on restarted grid")
	}
	cell, ok := restarted.CellAt(0, 0)
	if !ok || cell.Value == nil || *cell.Value != 0.92 || restarted.PendingOpID != "op-retry-1" || restarted.State != resultgrid.StateDraft {
		t.Fatalf("recovered cell/op mismatch: cell=%+v op=%q state=%q", cell, restarted.PendingOpID, restarted.State)
	}

	// Clearing after Core save prevents stale recovery on subsequent restart.
	ClearGridDraft("run-1", "task-a")
	fresh := resultgrid.New(schema, nil)
	fresh.State = resultgrid.StateSaved
	if syncGridDraft("run-1", "task-a", fresh, time.Unix(300, 0)) {
		t.Fatal("cleared draft should not recover")
	}
}
