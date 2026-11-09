package main

import (
	"context"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestStaleDesignRevisionLeavesNoPartialCommit(t *testing.T) {
	dir := copyConcurrencyFixture(t)
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
	before, err := svc.LoadDesign(ctx, designs[0].DesignID)
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.SaveDesign(ctx, runs.DesignSaveInput{
		DesignID: before.DesignID, ExpectedRevision: before.Revision,
		Changes: `{"title":"concurrency-current"}`, OperationID: "op-stale-base",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveDesign(ctx, runs.DesignSaveInput{
		DesignID: before.DesignID, ExpectedRevision: before.Revision,
		Changes: `{"title":"must-not-commit"}`, OperationID: "op-stale-rejected",
	}); err == nil {
		t.Fatal("stale revision unexpectedly succeeded")
	}
	after, err := svc.LoadDesign(ctx, before.DesignID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != first.Revision || after.Title != first.Title || after.Data != first.Data {
		t.Fatalf("stale save partially committed: before rejected=%+v after=%+v", first, after)
	}
}
