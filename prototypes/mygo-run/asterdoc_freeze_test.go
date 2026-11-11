package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/modules/runs"
)

func TestCreateRunFromAsterDocFreezesDocumentAndDesign(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	dsvc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer dsvc.Close()
	rsvc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer rsvc.Close()

	designs, err := dsvc.ListDesigns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var designID, protocolID string
	for _, design := range designs {
		snap, err := dsvc.LoadDesign(ctx, design.DesignID)
		if err != nil {
			continue
		}
		if strings.Contains(snap.Data, `"protocol_version_id":"290a0ba8-6582-4726-b077-b2a31e7a8950"`) {
			designID, protocolID = design.DesignID, "290a0ba8-6582-4726-b077-b2a31e7a8950"
			break
		}
	}
	if designID == "" {
		t.Skip("fixture has no CCK-8 design with protocol 290a")
	}
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	source := strings.Replace(normalized, "timepoints: [D1]\n```",
		"timepoints: [D1]\nprotocol_version_id: "+protocolID+"\n```", 1)
	doc, err := dsvc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: source, ExpectedRevision: 0,
		OperationID: "freeze-doc", ActorID: "tester",
	})
	if err != nil {
		t.Fatalf("SaveAsterDoc: %v", err)
	}
	run, err := dsvc.CreateRunFromAsterDoc(ctx, runs.AsterDocRunInput{
		DesignID: designID, DocumentRevision: doc.Revision,
		OperationID: "freeze-run", Title: "AsterDoc run",
	})
	if err != nil {
		t.Fatalf("CreateRunFromAsterDoc: %v", err)
	}
	if run.AsterDoc == nil {
		t.Fatal("run has no frozen AsterDoc reference")
	}
	if run.AsterDoc.Revision != doc.Revision || run.AsterDoc.SourceHash != doc.SourceHash {
		t.Fatalf("run asterdoc = %+v, doc = %+v", run.AsterDoc, doc)
	}
	if len(run.Tasks) == 0 {
		t.Fatal("run has no tasks")
	}
	foundDesignCheck := false
	for _, task := range run.Tasks {
		for _, check := range task.Checks {
			if check.CheckID == "custom-check" && check.Blocking {
				foundDesignCheck = true
			}
		}
	}
	if !foundDesignCheck {
		t.Fatal("design check did not reach the frozen run task")
	}
	var seedTask *runs.RunTask
	for i := range run.Tasks {
		if run.Tasks[i].Title == "root/read" {
			seedTask = &run.Tasks[i]
			break
		}
	}
	if seedTask == nil {
		t.Fatal("read task missing")
	}

	schema, plan, err := assays.BuildSchema([]byte(run.DesignData), seedTask.AssayID, seedTask.Timepoint)
	if err != nil {
		t.Fatalf("build result grid from frozen AsterDoc design: %v", err)
	}
	if unresolved := schema.Metadata["unresolved"]; unresolved != "" {
		t.Fatalf("frozen plate mapping unresolved: %s", unresolved)
	}
	cells := make([]runs.ResultCell, len(plan))
	recorded := 0
	for i, cell := range plan {
		cells[i] = runs.ResultCell{
			Row: cell.Row, Column: cell.Column, SampleID: cell.SampleID,
			Group: cell.Group, Replicate: cell.Replicate, Field: cell.Field, Status: "missing",
		}
		if cell.SampleID == "" {
			continue
		}
		value := 0.82
		cells[i].Value, cells[i].Status = &value, "raw"
		recorded++
	}
	if recorded != 2 {
		t.Fatalf("explicit plate identity produced %d recordable wells, want 2", recorded)
	}
	batch, err := rsvc.SaveReadoutsBatch(ctx, runs.BatchReadoutInput{
		RunID: run.RunID, OperationID: "freeze-plate-batch", TaskKey: seedTask.Key,
		AssayID: seedTask.AssayID, Timepoint: seedTask.Timepoint,
		ProtocolStepPath: seedTask.Title, ProtocolVersionID: seedTask.ProtocolVersionID,
		SchemaID: schema.ID, SchemaVersion: schema.Version, Cells: cells,
		ExpectedRevision: run.Revision,
	})
	if err != nil {
		t.Fatalf("SaveReadoutsBatch from frozen AsterDoc design: %v", err)
	}
	if batch.Accepted != len(cells) || batch.BatchID == "" {
		t.Fatalf("saved result batch = %+v", batch)
	}
	if _, err := rsvc.CompleteTask(ctx, runs.CompleteTaskInput{
		RunID: run.RunID, TaskKey: seedTask.Key, OperationID: "freeze-complete-missing",
		AssayID: seedTask.AssayID, Timepoint: seedTask.Timepoint,
		ProtocolStepPath: seedTask.Title, ProtocolVersionID: seedTask.ProtocolVersionID,
	}); err == nil {
		t.Fatal("completion succeeded with an unconfirmed design check")
	}
	acks := make([]runs.CheckAck, 0, len(seedTask.Checks))
	for _, check := range seedTask.Checks {
		acks = append(acks, runs.CheckAck{CheckID: check.CheckID, Status: "passed"})
	}
	if _, err := rsvc.SaveTaskChecks(ctx, runs.TaskCheckInput{
		RunID: run.RunID, TaskKey: seedTask.Key, OperationID: "freeze-checks",
		AssayID: seedTask.AssayID, Timepoint: seedTask.Timepoint,
		ProtocolStepPath: seedTask.Title, ProtocolVersionID: seedTask.ProtocolVersionID,
		Checks: acks,
	}); err != nil {
		t.Fatalf("SaveTaskChecks design check: %v", err)
	}
	if _, err := rsvc.CompleteTask(ctx, runs.CompleteTaskInput{
		RunID: run.RunID, TaskKey: seedTask.Key, OperationID: "freeze-complete",
		AssayID: seedTask.AssayID, Timepoint: seedTask.Timepoint,
		ProtocolStepPath: seedTask.Title, ProtocolVersionID: seedTask.ProtocolVersionID,
		Checks: acks,
	}); err != nil {
		t.Fatalf("CompleteTask with design check: %v", err)
	}
	loaded, err := dsvc.LoadDesign(ctx, designID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(loaded.Data, `"title":"CCK-8"`) || !strings.Contains(loaded.Data, `"workflow"`) ||
		!strings.Contains(loaded.Data, `"wells"`) {
		t.Fatalf("design projection missing: %s", loaded.Data)
	}
	// A fresh RunService reads the same frozen reference.
	rsvc2, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer rsvc2.Close()
	reopened, err := rsvc2.LoadRun(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.AsterDoc == nil || reopened.AsterDoc.Revision != doc.Revision ||
		reopened.AsterDoc.SourceHash != doc.SourceHash {
		t.Fatalf("reopened asterdoc = %+v", reopened.AsterDoc)
	}

	var reopenedBatch *runs.RunEvent
	for i := range reopened.Events {
		if reopened.Events[i].BatchID == batch.BatchID {
			reopenedBatch = &reopened.Events[i]
			break
		}
	}
	if reopenedBatch == nil || len(reopenedBatch.ResultCells) != len(cells) {
		t.Fatalf("reopened Run batch = %+v", reopenedBatch)
	}
	reopenedReplicates := map[int]bool{}
	for _, cell := range reopenedBatch.ResultCells {
		if cell.SampleID == "control-sample-1" && cell.Value != nil {
			reopenedReplicates[cell.Replicate] = true
		}
	}
	if len(reopenedReplicates) != 2 || !reopenedReplicates[1] || !reopenedReplicates[2] {
		t.Fatalf("reopened sample/technical replicates = %v", reopenedReplicates)
	}

	// A second document revision and Run must not rewrite the first Run.
	v2Source := strings.Replace(source, "title: CCK-8", "title: CCK-8 v2", 1)
	doc2, err := dsvc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: v2Source, ExpectedRevision: doc.Revision,
		OperationID: "freeze-doc-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dsvc.CreateRunFromAsterDoc(ctx, runs.AsterDocRunInput{
		DesignID: designID, DocumentRevision: doc2.Revision,
		OperationID: "freeze-run-2", Title: "AsterDoc run 2",
	}); err != nil {
		t.Fatal(err)
	}
	historical, err := rsvc2.LoadRun(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if historical.AsterDoc == nil || historical.AsterDoc.Revision != doc.Revision ||
		historical.AsterDoc.SourceHash != doc.SourceHash {
		t.Fatalf("historical run changed: %+v", historical.AsterDoc)
	}
	if historical.DesignData != reopened.DesignData {
		t.Fatal("historical run design data changed after a later document revision")
	}
	if !strings.Contains(historical.DesignData, `"title":"CCK-8"`) ||
		strings.Contains(historical.DesignData, `"title":"CCK-8 v2"`) {
		t.Fatal("historical run design data was not frozen")
	}
	latestDesign, err := dsvc.LoadDesign(ctx, designID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(latestDesign.Data, `"title":"CCK-8 v2"`) {
		t.Fatal("live design did not follow the second document revision")
	}

	// Legacy Design writes are refused once a document backs the design.
	if _, err := dsvc.SaveDesign(ctx, runs.DesignSaveInput{
		DesignID: designID, ExpectedRevision: latestDesign.Revision,
		Changes: `{"title":"legacy edit"}`, OperationID: "legacy-edit",
	}); err == nil || !strings.Contains(err.Error(), "AsterDoc") {
		t.Fatalf("legacy design write was not refused: %v", err)
	}
}
