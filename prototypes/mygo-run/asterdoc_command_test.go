package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/modules/runs"
)

func TestAsterDocSemanticCommandsPatchOnlyTargetModule(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	designs, err := svc.ListDesigns(ctx)
	if err != nil || len(designs) == 0 {
		t.Fatalf("designs: %v %v", designs, err)
	}
	designID := designs[0].DesignID
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	original := string(raw)
	if _, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: original, ExpectedRevision: 0,
		OperationID: "commands-doc", ActorID: "tester",
	}); err != nil {
		t.Fatal(err)
	}

	moved, err := svc.ApplyAsterDocCommand(ctx, runs.AsterDocCommandInput{
		DesignID: designID, ExpectedRevision: 1, OperationID: "assign-a1",
		Command: `{"type":"assign_wells","moduleId":"plate-d1","group":"g-material","wells":["A1"]}`,
	})
	if err != nil {
		t.Fatalf("AssignWells: %v", err)
	}
	if moved.Revision != 2 {
		t.Fatalf("revision = %d", moved.Revision)
	}
	doc, err := asterdoc.Parse(moved.Source)
	if err != nil {
		t.Fatal(err)
	}
	plate, _ := doc.PlateByID("plate-d1")
	if plate.WellGroups()["A1"] != "g-material" {
		t.Fatalf("A1 was not moved: %+v", plate.WellGroups())
	}
	if !strings.Contains(moved.Source, "# CCK-8") || !strings.Contains(moved.Source, "kind: group") {
		t.Fatalf("untargeted prose/modules changed:\n%s", moved.Source)
	}

	// Same operation + command is idempotent.
	again, err := svc.ApplyAsterDocCommand(ctx, runs.AsterDocCommandInput{
		DesignID: designID, ExpectedRevision: 1, OperationID: "assign-a1",
		Command: `{"type":"assign_wells","moduleId":"plate-d1","group":"g-material","wells":["A1"]}`,
	})
	if err != nil || again.Revision != 2 {
		t.Fatalf("idempotent command = %d, %v", again.Revision, err)
	}
	// Same operation with another command conflicts.
	if _, err := svc.ApplyAsterDocCommand(ctx, runs.AsterDocCommandInput{
		DesignID: designID, ExpectedRevision: 1, OperationID: "assign-a1",
		Command: `{"type":"assign_wells","moduleId":"plate-d1","group":"g-control","wells":["A2"]}`,
	}); err == nil {
		t.Fatal("same operation with another command was accepted")
	}
	// Stale revision is rejected.
	if _, err := svc.ApplyAsterDocCommand(ctx, runs.AsterDocCommandInput{
		DesignID: designID, ExpectedRevision: 1, OperationID: "stale-command",
		Command: `{"type":"clear_wells","moduleId":"plate-d1","wells":["B1"]}`,
	}); err == nil {
		t.Fatal("stale command was accepted")
	}

	// Update a different module and keep the already-patched plate.
	updated, err := svc.ApplyAsterDocCommand(ctx, runs.AsterDocCommandInput{
		DesignID: designID, ExpectedRevision: 2, OperationID: "update-exp",
		Command: `{"type":"update_module","moduleId":"exp-1","properties":{"objective":"updated objective"}}`,
	})
	if err != nil {
		t.Fatalf("UpdateModule: %v", err)
	}
	doc2, err := asterdoc.Parse(updated.Source)
	if err != nil {
		t.Fatal(err)
	}
	plate2, _ := doc2.PlateByID("plate-d1")
	if plate2.WellGroups()["A1"] != "g-material" {
		t.Fatal("an unrelated update reverted the plate patch")
	}
	if !strings.Contains(updated.Source, "objective: updated objective") {
		t.Fatal("update_module did not patch the experiment")
	}
}

func TestAsterDocStepCommandsUpdateWorkflowAtomically(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	designs, err := svc.ListDesigns(ctx)
	if err != nil || len(designs) == 0 {
		t.Fatalf("designs: %v %v", designs, err)
	}
	designID := designs[0].DesignID
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: string(raw), ExpectedRevision: 0,
		OperationID: "step-doc",
	}); err != nil {
		t.Fatal(err)
	}
	step := `{"kind":"step","id":"step-detect","schema":1,"title":"read","assay":"assay-cck8","timepoints":["D1"],"path":"root/read"}`
	added, err := svc.ApplyAsterDocCommand(ctx, runs.AsterDocCommandInput{
		DesignID: designID, ExpectedRevision: 1, OperationID: "add-step-detect",
		Command: `{"type":"add_step","workflowId":"wf-1","module":` + step + `}`,
	})
	if err != nil {
		t.Fatalf("add_step: %v", err)
	}
	doc, err := asterdoc.Parse(added.Source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(added.Source, "id: step-detect") {
		t.Fatal("step module was not appended")
	}
	workflowModule := false
	for _, module := range doc.Modules {
		if module.Kind == "workflow" {
			workflowModule = true
			steps := module.Data["steps"].([]any)
			if len(steps) != 3 {
				t.Fatalf("workflow steps = %+v", steps)
			}
		}
	}
	if !workflowModule {
		t.Fatal("workflow module missing")
	}
	connected, err := svc.ApplyAsterDocCommand(ctx, runs.AsterDocCommandInput{
		DesignID: designID, ExpectedRevision: 2, OperationID: "connect-read",
		Command: `{"type":"connect_steps","moduleId":"wf-1","from":"step-seed","to":"step-detect"}`,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !strings.Contains(connected.Source, "from: step-seed") {
		t.Fatal("edge was not added")
	}
	removed, err := svc.ApplyAsterDocCommand(ctx, runs.AsterDocCommandInput{
		DesignID: designID, ExpectedRevision: 3, OperationID: "remove-seed",
		Command: `{"type":"remove_step","moduleId":"step-seed"}`,
	})
	if err != nil {
		t.Fatalf("remove_step: %v", err)
	}
	doc2, err := asterdoc.Parse(removed.Source)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(removed.Source, "id: step-seed") ||
		strings.Contains(removed.Source, "from: step-seed") {
		t.Fatalf("step or edge survived removal:\n%s", removed.Source)
	}
	for _, module := range doc2.Modules {
		if module.Kind == "workflow" {
			steps := module.Data["steps"].([]any)
			if len(steps) != 2 || steps[0] != "step-read" || steps[1] != "step-detect" {
				t.Fatalf("workflow steps after removal = %+v", steps)
			}
		}
	}
}

func TestInspectorTaskAndCheckPropertiesReachProjection(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	designs, err := svc.ListDesigns(ctx)
	if err != nil || len(designs) == 0 {
		t.Fatalf("designs: %v %v", designs, err)
	}
	designID := designs[0].DesignID
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: string(raw), ExpectedRevision: 0, OperationID: "inspector-doc",
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.ApplyAsterDocCommand(ctx, runs.AsterDocCommandInput{
		DesignID: designID, ExpectedRevision: 1, OperationID: "inspector-update",
		Command: `{"type":"update_module","moduleId":"step-seed","properties":{"title":"接种","tasks":[{"id":"task-seed","title":"接种细胞","kind":"procedure","readout":""}],"checks":[{"checkId":"inspect-check","title":"确认密度","blocking":true}]}}`,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	proj, err := svc.CompileAsterDoc(ctx, designID, updated.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj.Design, `"inspect-check"`) {
		t.Fatalf("inspector checks missing from projection: %s", proj.Design)
	}
	if !strings.Contains(proj.Design, `"接种细胞"`) {
		t.Fatalf("inspector tasks missing from projection: %s", proj.Design)
	}
}
