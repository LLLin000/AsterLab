package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/modules/runs"
)

// TestCreateRunBindsExplicitStepTasks pins the frozen-design contract: a Run
// task names the design step the design explicitly assigned to its
// (assay, timepoint) pair — one assay may run under several steps, as D1/D3/D7
// CCK-8 does. Missing or duplicated entries stay unresolved; only designs
// without the explicit table fall back to the old unique-assay rule.
func TestCreateRunBindsExplicitStepTasks(t *testing.T) {
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

	type target struct {
		designID string
		assayID  string
		points   []string // distinct non-empty timepoints of the assay
		snapshot runs.RunSnapshot
	}
	var picked *target
	for _, design := range designs {
		run, err := dsvc.CreateRun(ctx, design.DesignID, "op-probe-"+design.DesignID)
		if err != nil {
			continue
		}
		snap, err := rsvc.LoadRun(ctx, run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		byAssay := map[string]map[string]bool{}
		for _, task := range snap.Tasks {
			if task.AssayID == "" || task.Timepoint == "" {
				continue
			}
			if byAssay[task.AssayID] == nil {
				byAssay[task.AssayID] = map[string]bool{}
			}
			byAssay[task.AssayID][task.Timepoint] = true
		}
		for assayID, set := range byAssay {
			if len(set) < 2 {
				continue
			}
			points := make([]string, 0, len(set))
			for p := range set {
				points = append(points, p)
			}
			sort.Strings(points)
			picked = &target{designID: design.DesignID, assayID: assayID, points: points, snapshot: snap}
			break
		}
		if picked != nil {
			break
		}
	}
	if picked == nil {
		t.Skip("fixture has no multi-timepoint assay task")
	}

	opSeq := 0
	saveMygo := func(mygo map[string]any) {
		cur, err := dsvc.LoadDesign(ctx, picked.designID)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(mygo)
		if err != nil {
			t.Fatal(err)
		}
		title, _ := json.Marshal(cur.Title)
		opSeq++
		if _, err := dsvc.SaveDesign(ctx, runs.DesignSaveInput{
			DesignID: picked.designID, ExpectedRevision: cur.Revision,
			Changes:     fmt.Sprintf(`{"title":%s,"mygo":%s}`, title, raw),
			OperationID: fmt.Sprintf("op-link-%d", opSeq),
		}); err != nil {
			t.Fatal(err)
		}
	}
	newRun := func() runs.RunSnapshot {
		opSeq++
		run, err := dsvc.CreateRun(ctx, picked.designID, fmt.Sprintf("op-run-%d", opSeq))
		if err != nil {
			t.Fatal(err)
		}
		snap, err := rsvc.LoadRun(ctx, run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		return snap
	}

	// Explicit table: every observed (assay, timepoint, path) gets its own
	// step, so one assay can run under different steps at D1, D3 and D7.
	type pair struct{ timepoint, path string }
	stepFor := map[pair]string{}
	links := make([]map[string]string, 0, len(picked.points))
	for _, task := range picked.snapshot.Tasks {
		if task.AssayID != picked.assayID || task.Timepoint == "" {
			continue
		}
		key := pair{task.Timepoint, task.Title}
		if _, ok := stepFor[key]; ok {
			continue
		}
		step := fmt.Sprintf("step-%d", len(stepFor)+1)
		stepFor[key] = step
		links = append(links, map[string]string{
			"stepId": step, "assayId": picked.assayID,
			"timepoint": task.Timepoint, "path": task.Title,
		})
	}
	if len(links) < 2 {
		t.Fatalf("fixture produced fewer than two task paths: %+v", picked.snapshot.Tasks)
	}
	saveMygo(map[string]any{"stepTasks": links})
	snap := newRun()
	seen := map[pair]bool{}
	for _, task := range snap.Tasks {
		if task.AssayID != picked.assayID {
			if task.DesignStepID != "" {
				t.Fatalf("task %s on assay %s unexpectedly linked to %q", task.Key, task.AssayID, task.DesignStepID)
			}
			continue
		}
		if task.Timepoint == "" {
			if task.DesignStepID != "" {
				t.Fatalf("task %s without a timepoint got step %q", task.Key, task.DesignStepID)
			}
			continue
		}
		key := pair{task.Timepoint, task.Title}
		want, ok := stepFor[key]
		if !ok {
			if task.DesignStepID != "" {
				t.Fatalf("task %s at %s/%s linked to %q without an entry", task.Key, task.Timepoint, task.Title, task.DesignStepID)
			}
			continue
		}
		seen[key] = true
		if task.DesignStepID != want {
			t.Fatalf("task %s at %s/%s linked to %q, want %q", task.Key, task.Timepoint, task.Title, task.DesignStepID, want)
		}
	}
	if len(seen) != len(stepFor) {
		t.Fatalf("bound %d of %d explicit task templates: %+v", len(seen), len(stepFor), snap.Tasks)
	}

	// Duplicating one exact entry is ambiguous and must not pick either step.
	dup := append([]map[string]string{{"stepId": "step-dup", "assayId": picked.assayID, "timepoint": links[0]["timepoint"], "path": links[0]["path"]}}, links...)
	saveMygo(map[string]any{"stepTasks": dup})
	snap = newRun()
	ambiguous := pair{links[0]["timepoint"], links[0]["path"]}
	for _, task := range snap.Tasks {
		if task.AssayID == picked.assayID && task.Timepoint == ambiguous.timepoint && task.Title == ambiguous.path && task.DesignStepID != "" {
			t.Fatalf("duplicated entry guessed step %q for task %s", task.DesignStepID, task.Key)
		}
		if task.AssayID == picked.assayID && task.DesignStepID == "step-dup" {
			t.Fatalf("duplicate step leaked onto task %s", task.Key)
		}
	}

	// Execution records keep the binding and never rewrite the freeze.
	var linked runs.RunTask
	for _, task := range snap.Tasks {
		if task.DesignStepID != "" && task.DesignStepID != "step-dup" {
			linked = task
			break
		}
	}
	if linked.Key == "" {
		t.Fatalf("no bound task to record against: %+v", snap.Tasks)
	}
	schema, gridCells, err := assays.BuildSchema([]byte(snap.DesignData), linked.AssayID, linked.Timepoint)
	if err != nil || len(gridCells) == 0 {
		t.Logf("fixture design has no readout schema: %v", err)
	} else {
		readout := 0.82
		cell := gridCells[0]
		if _, err := rsvc.SaveReadoutsBatch(ctx, runs.BatchReadoutInput{
			RunID: snap.RunID, OperationID: "op-linked-readout", TaskKey: linked.Key,
			AssayID: linked.AssayID, Timepoint: linked.Timepoint,
			ProtocolStepPath: linked.Title, ProtocolVersionID: linked.ProtocolVersionID,
			SchemaID: schema.ID, SchemaVersion: schema.Version,
			Cells: []runs.ResultCell{{
				Row: cell.Row, Column: cell.Column, SampleID: cell.SampleID, Group: cell.Group,
				Replicate: cell.Replicate, Field: cell.Field, Value: &readout, Status: "raw",
			}},
			ExpectedRevision: snap.Revision,
		}); err != nil {
			t.Fatal(err)
		}
		after, err := rsvc.LoadRun(ctx, snap.RunID)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, event := range after.Events {
			if event.RecordedAt == "" {
				t.Fatalf("stored event has no recorded_at: %+v", event)
			}
			for _, cell := range event.ResultCells {
				if cell.Value != nil && *cell.Value == readout {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("saved readout missing after reload: %+v", after.Events)
		}
		for _, task := range after.Tasks {
			if task.Key == linked.Key && task.DesignStepID != linked.DesignStepID {
				t.Fatalf("execution record changed the design link: %+v", task)
			}
		}
	}

	// Legacy designs without stepTasks keep the unique-assay fallback.
	saveMygo(map[string]any{"stepAssays": map[string]string{"legacy-step": picked.assayID}})
	snap = newRun()
	for _, task := range snap.Tasks {
		if task.AssayID == picked.assayID && task.DesignStepID != "legacy-step" {
			t.Fatalf("legacy fallback lost the unique assay link: %+v", task)
		}
	}
}
