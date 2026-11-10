package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/notebook"
	"asterlab-mygo-prototype/internal/protocol"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/modules/runs"
)

// TestCCK8VerticalSliceLoop runs the stored-data half of the v0.2 acceptance
// list against a real SQLite copy: frozen protocol, a 96-well OD450 batch,
// task completion, Review, Notebook provenance, reopen, and the CLI cross-read.
// GUI interaction is covered separately by an operator; this test never opens
// a window.
func TestCCK8VerticalSliceLoop(t *testing.T) {
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
	var designID, assayID, timepoint string
	for _, design := range designs {
		snap, err := dsvc.LoadDesign(ctx, design.DesignID)
		if err != nil {
			t.Fatal(err)
		}
		if !designHasControls(snap.Data) {
			continue
		}
		ids := assayKeys(snap.Data)
		for _, id := range ids {
			for _, tp := range []string{"", "24h"} {
				schema, _, err := assays.BuildSchema([]byte(snap.Data), id, tp)
				if err != nil {
					continue
				}
				if schema.Layout == "plate96" {
					designID, assayID = design.DesignID, id
					timepoint = firstTimepoint(snap.Data, id)
					if timepoint == "" {
						timepoint = tp
					}
					break
				}
			}
			if designID != "" {
				break
			}
		}
		if designID != "" {
			break
		}
	}
	if designID == "" {
		t.Skip("fixture has no CCK-8 design")
	}
	if err := seedPlatePlan(ctx, dsvc, designID, assayID, timepoint); err != nil {
		t.Fatalf("seed plate plan: %v", err)
	}

	run, err := dsvc.CreateRun(ctx, designID, "cck8-loop-create")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	snap, err := rsvc.LoadRun(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.DesignData == "" {
		t.Fatal("run has no frozen design data")
	}
	audit := snap.CreationAudit
	if audit == nil {
		t.Fatal("run has no creation audit")
	}
	if audit.DesignRevision != snap.DesignRevision || audit.DesignVersionID == "" {
		t.Fatalf("audit revision/version mismatch: %+v", audit)
	}
	if audit.ActorID != "fixture-operator" || audit.RecordedAt == "" {
		t.Fatalf("audit misses actor or time: %+v", audit)
	}
	foundWarning := false
	for _, warning := range audit.Warnings {
		if strings.Contains(warning, "被 2 个步骤共享") {
			foundWarning = true
		}
	}
	if !foundWarning || audit.Decision != "acknowledged_warnings" || len(audit.AcknowledgedWarnings) != 1 {
		t.Fatalf("warning audit = %+v", audit)
	}
	frozen, err := protocol.Build(snap.DesignData, snap.DesignID, snap.DesignRevision)
	if err != nil {
		t.Fatalf("frozen protocol: %v", err)
	}
	if frozen.Revision != snap.DesignRevision {
		t.Fatalf("protocol revision %d != frozen revision %d", frozen.Revision, snap.DesignRevision)
	}

	var task *runs.RunTask
	for i := range snap.Tasks {
		candidate := &snap.Tasks[i]
		if normalizeAssay(candidate.AssayID) != normalizeAssay(assayID) {
			continue
		}
		if candidate.Timepoint != "" {
			timepoint = candidate.Timepoint
		}
		if len(candidate.Checks) > 0 {
			task = candidate
			break
		}
		if task == nil {
			task = candidate
		}
	}
	if task == nil {
		t.Skip("frozen design produced no task for the CCK-8 assay")
	}

	// Editing the design after creation must not change the Run's protocol.
	cur, err := dsvc.LoadDesign(ctx, designID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dsvc.SaveDesign(ctx, runs.DesignSaveInput{
		DesignID: designID, ExpectedRevision: cur.Revision,
		Changes:     `{"title":"处置后改动的标题"}`,
		OperationID: "cck8-loop-live-edit",
	}); err != nil {
		t.Fatalf("live edit: %v", err)
	}
	again, err := rsvc.LoadRun(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if again.DesignData != snap.DesignData {
		t.Fatal("run's frozen design changed after a live edit")
	}

	schema, plan, err := assays.BuildSchema([]byte(snap.DesignData), task.AssayID, timepoint)
	if err != nil {
		t.Fatalf("build schema for task: %v", err)
	}
	if schema.ID != "cck8.plate96.od450.v1" {
		t.Fatalf("unexpected schema %q", schema.ID)
	}
	cells := make([]runs.ResultCell, len(plan))
	raw := 0
	for i, cell := range plan {
		cells[i] = runs.ResultCell{Row: cell.Row, Column: cell.Column, SampleID: cell.SampleID,
			Group: cell.Group, Replicate: cell.Replicate, Field: cell.Field, Status: "missing"}
		if cell.SampleID == "" {
			continue
		}
		value := 0.40 + float64(i%12)*0.01
		cells[i].Value = &value
		cells[i].Status = "raw"
		raw++
	}
	if raw == 0 {
		t.Fatal("no assigned wells to record")
	}
	batch, err := rsvc.SaveReadoutsBatch(ctx, runs.BatchReadoutInput{
		RunID: run.RunID, OperationID: "cck8-loop-batch", TaskKey: task.Key,
		AssayID: task.AssayID, Timepoint: timepoint, ProtocolStepPath: task.Title,
		ProtocolVersionID: task.ProtocolVersionID,
		SchemaID:          schema.ID, SchemaVersion: schema.Version, Cells: cells,
		ExpectedRevision: snap.Revision,
	})
	if err != nil {
		t.Fatalf("SaveReadoutsBatch: %v", err)
	}
	if batch.Accepted != len(cells) || batch.BatchID == "" {
		t.Fatalf("batch result = %+v", batch)
	}
	if len(task.Checks) == 0 {
		t.Fatalf("frozen task has no checks; this acceptance must run with a real protocol")
	}
	// Completion without acknowledgements must be refused by Core.
	if _, err := rsvc.CompleteTask(ctx, runs.CompleteTaskInput{
		RunID: run.RunID, TaskKey: task.Key, OperationID: "cck8-loop-complete-missing",
		AssayID: task.AssayID, Timepoint: timepoint, ProtocolStepPath: task.Title,
		ProtocolVersionID: task.ProtocolVersionID,
	}); err == nil {
		t.Fatal("CompleteTask accepted a task with unconfirmed blocking checks")
	}
	acks := make([]runs.CheckAck, 0, len(task.Checks))
	for _, check := range task.Checks {
		acks = append(acks, runs.CheckAck{CheckID: check.CheckID, Status: "passed"})
	}
	checkedSnap, err := rsvc.SaveTaskChecks(ctx, runs.TaskCheckInput{
		RunID: run.RunID, TaskKey: task.Key, OperationID: "cck8-loop-checks",
		AssayID: task.AssayID, Timepoint: timepoint, ProtocolStepPath: task.Title,
		ProtocolVersionID: task.ProtocolVersionID, Checks: acks,
	})
	if err != nil {
		t.Fatalf("SaveTaskChecks: %v", err)
	}
	persisted := false
	for i := range checkedSnap.Tasks {
		if checkedSnap.Tasks[i].Key == task.Key {
			persisted = len(checkedSnap.Tasks[i].Checks) == len(task.Checks)
		}
	}
	if !persisted {
		t.Fatal("saved checks were not returned with the task")
	}
	noteMarkdown := "# D1 观察\n\n- 培养基澄清\n- 边缘无气泡"
	if _, err := rsvc.SaveNotebook(ctx, runs.NotebookInput{
		RunID: run.RunID, Markdown: noteMarkdown, OperationID: "cck8-loop-notebook",
	}); err != nil {
		t.Fatalf("SaveNotebook: %v", err)
	}
	if _, err := rsvc.CompleteTask(ctx, runs.CompleteTaskInput{
		RunID: run.RunID, TaskKey: task.Key, OperationID: "cck8-loop-complete",
		AssayID: task.AssayID, Timepoint: timepoint, ProtocolStepPath: task.Title,
		ProtocolVersionID: task.ProtocolVersionID, Checks: acks,
	}); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	after, err := rsvc.LoadRun(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var batchEvent *runs.RunEvent
	completed := false
	for i := range after.Events {
		event := &after.Events[i]
		if event.BatchID == batch.BatchID {
			batchEvent = event
		}
		if event.TaskKey == task.Key && event.StepStatus == "completed" {
			completed = true
		}
	}
	if batchEvent == nil || len(batchEvent.ResultCells) != len(cells) {
		t.Fatalf("stored batch event = %+v", batchEvent)
	}
	noteStored := false
	for _, event := range after.Events {
		if event.Stage == "notebook" && event.Content == noteMarkdown {
			noteStored = true
		}
	}
	if !noteStored {
		t.Fatal("notebook markdown was not stored as a notebook-stage event")
	}
	if batchEvent.RecordedAt == "" || batchEvent.OccurredAt == "" {
		t.Fatal("stored batch is missing a recorded or occurred time")
	}
	if !completed {
		t.Fatal("task completion was not stored")
	}

	grid := resultgrid.New(schema, toGridCells(batchEvent.ResultCells))
	review := resultgrid.Summarize(grid)
	bySample := map[string]resultgrid.ReviewRow{}
	for _, row := range review.Rows {
		bySample[row.SampleID] = row
	}
	checked := 0
	for _, cell := range cells {
		if cell.Status != "raw" {
			continue
		}
		row, ok := bySample[cell.SampleID]
		if !ok {
			t.Fatalf("review has no row for %s", cell.SampleID)
		}
		if row.Recorded < 1 {
			t.Fatalf("review dropped a recorded value for %s", cell.SampleID)
		}
		if math.Abs(row.Mean-*cell.Value) > 1e-12 && row.Recorded == 1 {
			t.Fatalf("review mean for %s = %v, raw = %v", cell.SampleID, row.Mean, *cell.Value)
		}
		checked++
	}
	if checked != raw {
		t.Fatalf("checked %d raw cells, recorded %d", checked, raw)
	}

	book := notebook.Build(after)
	var batchEntry *notebook.Entry
	for i := range book.Entries {
		if book.Entries[i].References[0].Kind == "readout_batch" && book.Entries[i].References[0].ID == batch.BatchID {
			batchEntry = &book.Entries[i]
		}
	}
	if batchEntry == nil {
		t.Fatal("notebook has no entry for the stored batch")
	}
	if batchEntry.Trace != notebook.TraceRecorded {
		t.Fatalf("batch trace = %q, want recorded", batchEntry.Trace)
	}
	if batchEntry.DesignStepID != task.DesignStepID {
		t.Fatalf("batch step = %q, task step = %q", batchEntry.DesignStepID, task.DesignStepID)
	}
	foundCells := false
	for _, detail := range batchEntry.Details {
		if detail.Label == "单元格" && detail.Value == fmt.Sprintf("%d 个", len(cells)) {
			foundCells = true
		}
	}
	if !foundCells {
		t.Fatalf("batch entry does not show %d cells: %+v", len(cells), batchEntry.Details)
	}

	// Reopen the database through a fresh service: the same revision and cells
	// must come back, then the CLI must read the same run.
	rsvc2, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer rsvc2.Close()
	reopened, err := rsvc2.LoadRun(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Revision != after.Revision {
		t.Fatalf("reopen revision %d != %d", reopened.Revision, after.Revision)
	}
	reopenedCells := 0
	for _, event := range reopened.Events {
		if event.BatchID == batch.BatchID {
			reopenedCells = len(event.ResultCells)
		}
	}
	if reopenedCells != len(cells) {
		t.Fatalf("reopen batch cells = %d, want %d", reopenedCells, len(cells))
	}

	cli := "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/release/labworkbench.exe"
	if _, err := os.Stat(cli); err == nil {
		dir := os.Getenv("LABWORKBENCH_DATA_DIR")
		input := filepath.Join(dir, "cck8-loop-input.json")
		if err := os.WriteFile(input, []byte(`{"id":"`+run.RunID+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(cli, "run.workspace", "--input", input)
		cmd.Env = append(os.Environ(), "LABWORKBENCH_DATA_DIR="+dir)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("CLI cross-read: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), batch.BatchID) || !strings.Contains(string(out), "OD450") {
			t.Fatalf("CLI cross-read missing the batch: %s", string(out))
		}
		if !strings.Contains(string(out), "acknowledged_warnings") {
			t.Fatalf("CLI cross-read missing the creation audit: %s", string(out))
		}
		if !strings.Contains(string(out), "D1 观察") {
			t.Fatalf("CLI cross-read missing the notebook: %s", string(out))
		}
	}
}

// seedPlatePlan writes the 96-well allocation this test executes. It is a
// test fixture, not experimental data: six planned wells for two groups.
func seedPlatePlan(ctx context.Context, svc runs.DesignService, designID, assayID, timepoint string) error {
	cur, err := svc.LoadDesign(ctx, designID)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(cur.Data), &doc); err != nil {
		return err
	}
	groups := map[string]string{}
	names := []string{}
	if specs, ok := doc["group_specs"].([]any); ok {
		for _, raw := range specs {
			spec, _ := raw.(map[string]any)
			name, _ := spec["name"].(string)
			id, _ := spec["id"].(string)
			if name == "" || id == "" {
				continue
			}
			if _, ok := groups[name]; !ok {
				names = append(names, name)
			}
			groups[name] = id
		}
	}
	if len(names) < 2 {
		return fmt.Errorf("design has fewer than two group specs")
	}
	control, treatment := names[0], names[1]
	if specs, ok := doc["group_specs"].([]any); ok {
		for _, raw := range specs {
			spec, _ := raw.(map[string]any)
			if role, _ := spec["role"].(string); role == "control" {
				if name, _ := spec["name"].(string); name != "" {
					control = name
				}
			}
		}
	}
	if treatment == control {
		treatment = names[1]
	}
	wells := make([]any, 0, 6)
	samples := []struct {
		well, sample, group string
		rep                 int
	}{
		{"A1", control + "-1", control, 1},
		{"A2", control + "-2", control, 2},
		{"A3", control + "-3", control, 3},
		{"A4", treatment + "-1", treatment, 1},
		{"A5", treatment + "-2", treatment, 2},
		{"A6", treatment + "-3", treatment, 3},
	}
	for _, sample := range samples {
		groupID := groups[sample.group]
		if groupID == "" {
			return fmt.Errorf("design has no %s group", sample.group)
		}
		wells = append(wells, map[string]any{
			"wellId": sample.well, "sampleId": sample.sample, "groupId": groupID,
			"replicate": sample.rep, "timepoint": timepoint,
		})
	}
	mygo, _ := doc["mygo"].(map[string]any)
	if mygo == nil {
		mygo = map[string]any{}
	}
	params, _ := mygo["assays"].(map[string]any)
	if params == nil {
		params = map[string]any{}
	}
	entry, _ := params[assayID].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
	}
	entry["wells"] = wells
	entry["timepoints"] = []any{timepoint}
	entry["technicalReplicates"] = 3
	entry["controlGroup"] = control
	params[assayID] = entry
	mygo["assays"] = params
	// Two steps claim the same failure-prone pair: Core must record this as a
	// warning in the creation audit instead of silently dropping it.
	mygo["stepTasks"] = []any{
		map[string]any{"stepId": "seed-step-a", "assayId": assayID, "timepoint": timepoint},
		map[string]any{"stepId": "seed-step-b", "assayId": assayID, "timepoint": timepoint},
	}
	ackWarning := fmt.Sprintf("检测 %s 的 %s 被 2 个步骤共享；任务按 path / Step ID 区分，不按时间点合并。", assayID, timepoint)
	mygo["preflightAck"] = map[string]any{
		"warnings":       []any{ackWarning},
		"actorId":        "fixture-operator",
		"acknowledgedAt": "2026-11-10T09:00:00Z",
	}
	doc["mygo"] = mygo
	raw, err := json.Marshal(mygo)
	if err != nil {
		return err
	}
	title, _ := json.Marshal(cur.Title)
	_, err = svc.SaveDesign(ctx, runs.DesignSaveInput{
		DesignID: designID, ExpectedRevision: cur.Revision,
		Changes:     fmt.Sprintf(`{"title":%s,"mygo":%s}`, title, raw),
		OperationID: "cck8-loop-seed-plate",
	})
	return err
}

func firstTimepoint(data, assayID string) string {
	var doc struct {
		Assays []struct {
			ID         string   `json:"id"`
			Timepoints []string `json:"timepoints"`
		} `json:"assays"`
	}
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		return ""
	}
	for _, assay := range doc.Assays {
		if normalizeAssay(assay.ID) == normalizeAssay(assayID) && len(assay.Timepoints) > 0 {
			return assay.Timepoints[0]
		}
	}
	return ""
}

func designHasControls(data string) bool {
	var doc struct {
		Assays []struct {
			Controls []string `json:"controls"`
		} `json:"assays"`
	}
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		return true
	}
	for _, assay := range doc.Assays {
		if len(assay.Controls) > 0 {
			return true
		}
	}
	return false
}

func assayKeys(data string) []string {
	var doc struct {
		MyGo struct {
			Assays map[string]json.RawMessage `json:"assays"`
		} `json:"mygo"`
		Assays []struct {
			ID string `json:"id"`
		} `json:"assays"`
	}
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		return nil
	}
	seen := map[string]bool{}
	keys := make([]string, 0, len(doc.MyGo.Assays)+len(doc.Assays))
	for key := range doc.MyGo.Assays {
		seen[key] = true
		keys = append(keys, key)
	}
	for _, assay := range doc.Assays {
		if assay.ID != "" && !seen[assay.ID] {
			seen[assay.ID] = true
			keys = append(keys, assay.ID)
		}
	}
	return keys
}

func normalizeAssay(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func toGridCells(cells []runs.ResultCell) []resultgrid.Cell {
	out := make([]resultgrid.Cell, len(cells))
	for i, cell := range cells {
		out[i] = resultgrid.Cell{Row: cell.Row, Column: cell.Column, SampleID: cell.SampleID,
			Group: cell.Group, Replicate: cell.Replicate, Field: cell.Field,
			Value: cell.Value, Status: cell.Status}
	}
	return out
}
