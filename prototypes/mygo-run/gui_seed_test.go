package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/modules/runs"
)

// TestSeedGUIRun prepares target/gui-data for a real-window walkthrough. It is
// skipped unless ASTERLAB_SEED_GUI=1; it writes only the requested data dir.
func TestSeedGUIRun(t *testing.T) {
	if os.Getenv("ASTERLAB_SEED_GUI") != "1" {
		t.Skip("set ASTERLAB_SEED_GUI=1 to seed the GUI data directory")
	}
	dir, err := filepath.Abs("target/gui-data")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
	src := "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/asterlab-ui-runtime3/labworkbench.sqlite"
	db, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "labworkbench.sqlite"), db, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
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
			continue
		}
		var doc struct {
			Assays []struct {
				ID         string   `json:"id"`
				Timepoints []string `json:"timepoints"`
				Controls   []string `json:"controls"`
			} `json:"assays"`
		}
		if json.Unmarshal([]byte(snap.Data), &doc) != nil {
			continue
		}
		for _, assay := range doc.Assays {
			if normalizeAssay(assay.ID) == "cck8" && len(assay.Controls) > 0 && len(assay.Timepoints) > 0 {
				designID, assayID, timepoint = design.DesignID, assay.ID, assay.Timepoints[0]
				break
			}
		}
		if designID != "" {
			break
		}
	}
	if designID == "" {
		t.Fatal("no CCK-8 design with controls")
	}
	if err := seedPlatePlan(ctx, dsvc, designID, assayID, timepoint); err != nil {
		t.Fatal(err)
	}
	run, err := dsvc.CreateRun(ctx, designID, "gui-seed-run")
	if err != nil {
		t.Fatal(err)
	}
	snap, err := rsvc.LoadRun(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var task *runs.RunTask
	for i := range snap.Tasks {
		candidate := &snap.Tasks[i]
		if len(candidate.Checks) > 0 && normalizeAssay(candidate.AssayID) == normalizeAssay(assayID) {
			task = candidate
			break
		}
	}
	if task == nil {
		t.Fatal("no task with checks")
	}
	schema, plan, err := assays.BuildSchema([]byte(snap.DesignData), task.AssayID, task.Timepoint)
	if err != nil {
		t.Fatal(err)
	}
	cells := make([]runs.ResultCell, len(plan))
	for i, cell := range plan {
		cells[i] = runs.ResultCell{Row: cell.Row, Column: cell.Column, SampleID: cell.SampleID,
			Group: cell.Group, Replicate: cell.Replicate, Field: cell.Field, Status: "missing"}
		if cell.SampleID == "" {
			continue
		}
		value := 0.40 + float64(i%12)*0.01
		cells[i].Value = &value
		cells[i].Status = "raw"
	}
	if _, err := rsvc.SaveReadoutsBatch(ctx, runs.BatchReadoutInput{
		RunID: run.RunID, OperationID: "gui-seed-batch", TaskKey: task.Key,
		AssayID: task.AssayID, Timepoint: task.Timepoint, ProtocolStepPath: task.Title,
		ProtocolVersionID: task.ProtocolVersionID,
		SchemaID:          schema.ID, SchemaVersion: schema.Version, Cells: cells,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := rsvc.SaveNotebook(ctx, runs.NotebookInput{
		RunID: run.RunID, OperationID: "gui-seed-notebook",
		Markdown: "# D1 观察\n\n- 细胞贴壁良好，边缘无气泡\n- 空白孔已加入\n\n```asterlab\nkind: plate96\nid: plate-cck8-d1\nschema: 1\nassay: cck8\ngroups:\n  control: [A1, A2, A3]\n  material: [B1, B2, B3]\n```\n\n## 备注\n\n下一次读数前检查培养基颜色。",
	}); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("GUI_RUN_ID=%s TASK=%s CHECKS=%d\n", run.RunID, task.Title, len(task.Checks))
	for _, check := range task.Checks {
		if strings.TrimSpace(check.Label) == "" {
			t.Fatalf("empty check label: %+v", check)
		}
	}
}
