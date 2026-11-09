package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestReadoutWithTaskProtocolContext(t *testing.T) {
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
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	ctx := context.Background()
	snap, err := svc.LoadRun(ctx, "1c0c8989-0513-483a-851b-bdc7d6c67483")
	if err != nil {
		t.Fatal(err)
	}
	var task runs.RunTask
	for _, candidate := range snap.Tasks {
		if candidate.AssayID != "" && candidate.Timepoint != "" {
			task = candidate
			break
		}
	}
	if task.Key == "" {
		t.Skip("no assay task in fixture")
	}
	if _, err := svc.SaveReadout(ctx, runs.ReadoutInput{
		RunID:             snap.RunID,
		TaskKey:           task.Key,
		Timepoint:         task.Timepoint,
		AssayID:           task.AssayID,
		ProtocolStepPath:  task.Title,
		ProtocolVersionID: task.ProtocolVersionID,
		Measurements:      []runs.Measurement{{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"}},
		OperationID:       "readout-protocol-test",
	}); err != nil {
		t.Fatalf("SaveReadout with task protocol context: %v", err)
	}
}
