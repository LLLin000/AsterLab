package main

import (
	"context"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestProtocolBlockPreviewFixture(t *testing.T) {
	dir := rnaFixtureDataDir(t)
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	preview, err := svc.PreviewProtocolBlock(context.Background(), "c23df8e1-3f2a-4449-8ac9-a4bb7256a113")
	if err != nil {
		t.Fatal(err)
	}
	if preview.BlockVersionID != "c23df8e1-3f2a-4449-8ac9-a4bb7256a113" {
		t.Fatalf("block version = %q", preview.BlockVersionID)
	}
	if len(preview.Steps) != 10 {
		t.Fatalf("steps = %d, want 10", len(preview.Steps))
	}
	first := preview.Steps[0]
	if first.Path != "root/01-collect" || first.Title == "" || first.Instruction == "" || first.SourceBlockVersionID != preview.BlockVersionID {
		t.Fatalf("first step = %+v", first)
	}
}
