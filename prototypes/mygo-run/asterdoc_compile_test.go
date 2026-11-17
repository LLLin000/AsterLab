package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestCompileAsterDocProjectsGroupsAssaysAndSteps(t *testing.T) {
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
	saved, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: string(raw), ExpectedRevision: 0,
		OperationID: "compile-doc", ActorID: "tester",
	})
	if err != nil {
		t.Fatal(err)
	}
	projection, err := svc.CompileAsterDoc(ctx, designID, saved.Revision)
	if err != nil {
		t.Fatalf("CompileAsterDoc: %v", err)
	}
	var design struct {
		Title      string `json:"title"`
		GroupSpecs []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"group_specs"`
		Assays []struct {
			ID          string   `json:"id"`
			GroupIDs    []string `json:"group_ids"`
			Timepoints  []string `json:"timepoints"`
			ProtocolRef string   `json:"protocol_version_id"`
		} `json:"assays"`
		MyGo struct {
			Workflow struct {
				Nodes         []map[string]any `json:"Nodes"`
				BusinessOrder []string         `json:"BusinessOrder"`
			} `json:"workflow"`
			StepTasks []map[string]any `json:"stepTasks"`
			Assays    map[string]struct {
				Wells []struct {
					WellID    string `json:"wellId"`
					SampleID  string `json:"sampleId"`
					GroupID   string `json:"groupId"`
					Replicate int    `json:"replicate"`
					Timepoint string `json:"timepoint"`
				} `json:"wells"`
			} `json:"assays"`
		} `json:"mygo"`
	}
	if err := json.Unmarshal([]byte(projection.Design), &design); err != nil {
		t.Fatal(err)
	}
	if design.Title != "CCK-8" || len(design.GroupSpecs) != 2 {
		t.Fatalf("design = %+v", design)
	}
	if len(design.Assays) != 1 || design.Assays[0].ID != "assay-cck8" ||
		len(design.Assays[0].GroupIDs) != 2 {
		t.Fatalf("assays = %+v", design.Assays)
	}
	params := design.MyGo.Assays["assay-cck8"]
	if len(params.Wells) != 2 || params.Wells[0].GroupID != "g-control" ||
		params.Wells[0].SampleID == "" || params.Wells[0].Timepoint == "" {
		t.Fatalf("projected wells = %+v", params.Wells)
	}
	if len(design.MyGo.StepTasks) != 2 || design.MyGo.StepTasks[0]["path"] != "root/seed" ||
		design.MyGo.StepTasks[1]["path"] != "root/read" {
		t.Fatalf("step tasks = %+v", design.MyGo.StepTasks)
	}
	if len(design.MyGo.Workflow.Nodes) != 2 || len(design.MyGo.Workflow.BusinessOrder) != 2 {
		t.Fatalf("workflow = %+v", design.MyGo.Workflow)
	}
}

func TestCompileAsterDocPreservesSDKModules(t *testing.T) {
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
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw) + `
` + "```asterlab\nkind: calculator\nid: dilution-01\nschema: 1\nmode: dilution\ntitle: Drug dilution\nstock:\n  value: 20\n  unit: mg/mL\ntarget:\n  value: 2\n  unit: mg/mL\nfinal_volume:\n  value: 5\n  unit: mL\n```\n" + "```asterlab\nkind: instrument_profile\nid: uv-1\nschema: 1\nclass: uv_curing\nmodel: UV-1\ncapabilities:\n  - name: 365 nm\nsource: built_in\n```\n"
	saved, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designs[0].DesignID, Source: source, ExpectedRevision: 0,
		OperationID: "compile-sdk-modules",
	})
	if err != nil {
		t.Fatal(err)
	}
	projection, err := svc.CompileAsterDoc(ctx, designs[0].DesignID, saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	var design struct {
		AsterLab struct {
			FoundationVersion int              `json:"foundationVersion"`
			SDKVersion        string           `json:"sdkVersion"`
			Modules           []map[string]any `json:"modules"`
		} `json:"asterlab"`
	}
	if err := json.Unmarshal([]byte(projection.Design), &design); err != nil {
		t.Fatal(err)
	}
	if design.AsterLab.FoundationVersion != 1 || design.AsterLab.SDKVersion == "" || len(design.AsterLab.Modules) != 2 {
		t.Fatalf("frozen sdk modules = %+v", design.AsterLab)
	}
	if design.AsterLab.Modules[0]["kind"] != "calculator" || design.AsterLab.Modules[0]["id"] != "dilution-01" {
		t.Fatalf("calculator not preserved: %+v", design.AsterLab.Modules[0])
	}
}

func TestCompileAsterDocProjectsStepUses(t *testing.T) {
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
	source := "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: SDK binding\n```\n```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: Assay\n```\n```asterlab\nkind: calculator\nid: dilution-01\nschema: 1\nmode: dilution\nstock: {value: 20, unit: mg/mL}\ntarget: {value: 2, unit: mg/mL}\nfinal_volume: {value: 5, unit: mL}\n```\n```asterlab\nkind: step\nid: step-uv\nschema: 1\ntitle: UV cure\nassay: assay-1\npath: root/uv\nuses:\n  - ref: dilution-01\n    role: calculator\n```\n"
	saved, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designs[0].DesignID, Source: source, ExpectedRevision: 0,
		OperationID: "compile-step-uses",
	})
	if err != nil {
		t.Fatal(err)
	}
	projection, err := svc.CompileAsterDoc(ctx, designs[0].DesignID, saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	var design struct {
		MyGo struct {
			StepUses map[string][]map[string]any `json:"stepUses"`
		} `json:"mygo"`
	}
	if err := json.Unmarshal([]byte(projection.Design), &design); err != nil {
		t.Fatal(err)
	}
	uses := design.MyGo.StepUses["step-uv"]
	if len(uses) != 1 || uses[0]["ref"] != "dilution-01" || uses[0]["role"] != "calculator" {
		t.Fatalf("step uses = %+v", design.MyGo.StepUses)
	}
}
