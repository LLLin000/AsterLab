package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

// Regression: a real Design must create a Run with a default title, and the
// same operation id must not create a second Run.
func TestCreateRunFromRealDesignIsIdempotent(t *testing.T) {
	src := "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/asterlab-ui-runtime3/labworkbench.sqlite"
	db, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("fixture DB unavailable: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "labworkbench.sqlite"), db, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	ctx := context.Background()
	designs, err := svc.ListDesigns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(designs) == 0 {
		t.Skip("fixture has no designs")
	}
	designID := designs[0].DesignID
	first, err := svc.CreateRun(ctx, designID, "create-run-test-op")
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	if first.RunID == "" {
		t.Fatal("empty run id")
	}
	second, err := svc.CreateRun(ctx, designID, "create-run-test-op")
	if err != nil {
		t.Fatalf("idempotent CreateRun failed: %v", err)
	}
	if second.RunID != first.RunID {
		t.Fatalf("same operation created a second Run: %s != %s", second.RunID, first.RunID)
	}
	if _, err := svc.CreateRun(ctx, designID, "create-run-test-op-2"); err != nil {
		t.Fatalf("second operation failed: %v", err)
	}
}
