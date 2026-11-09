package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestRustModulePersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runs.json")
	t.Setenv("ASTERLAB_MYGO_RUNS", path)
	if err := os.WriteFile(path, []byte(`{"runId":"run-1","title":"L929","status":"in_progress","tasks":[],"measurements":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	snap, err := svc.LoadRun(ctx, "run-1")
	if err != nil || snap.RunID != "run-1" {
		t.Fatalf("load: %+v %v", snap, err)
	}
	_, err = svc.SaveReadout(ctx, runs.ReadoutInput{RunID: "run-1", Measurements: []runs.Measurement{{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"}}})
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.LoadRun(ctx, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Measurements) != 1 || again.Measurements[0].Value != 0.82 {
		t.Fatalf("persisted: %+v", again)
	}
}
