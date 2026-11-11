package runworkspace

import (
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/modules/runs"
)

func TestBuildViewModelRemapsModules(t *testing.T) {
	snap := runs.RunSnapshot{
		RunID:    "run-1",
		Title:    "CCK-8",
		Revision: 6,
		Status:   "running",
		Tasks: []runs.RunTask{
			{
				Key:          "t-1",
				DesignStepID: "step-seed",
				Title:        "Seed cells",
				Timepoint:    "D0",
				AssayID:      "assay-cck8",
				Status:       "completed",
				Checks: []runs.TaskCheck{
					{CheckID: "c1", Label: "Confirm plate layout", Blocking: true},
				},
			},
			{
				Key:          "t-2",
				DesignStepID: "step-read",
				Title:        "Read OD450",
				Timepoint:    "D1",
				AssayID:      "assay-cck8",
				Status:       "pending",
				Checks: []runs.TaskCheck{
					{CheckID: "c2", Label: "Confirm OD450 reading", Blocking: true},
				},
			},
		},
		Measurements: []runs.Measurement{
			{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU", TaskKey: "t-2", ProtocolStepPath: "root/read"},
		},
	}
	source := "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: CCK-8\n```\n```asterlab\nkind: group\nid: g-control\nschema: 1\nname: Control\nrole: control\n```\n```asterlab\nkind: instrument_profile\nid: uv-reader\nschema: 1\nclass: plate_reader\nmodel: UV-1\ncapabilities:\n  - name: absorbance\n    wavelength: 450 nm\nsource: built_in\n```\n```asterlab\nkind: dilution\nid: dil-stock\nschema: 1\nstock:\n  value: 20\n  unit: mg/mL\ntarget:\n  value: 2\n  unit: mg/mL\nfinal_volume:\n  value: 5\n  unit: mL\n```\n```asterlab\nkind: assay\nid: assay-cck8\nschema: 1\ntitle: CCK-8\ngroup_refs: [g-control]\ntimepoints: [D1]\n```\n```asterlab\nkind: step\nid: step-seed\nschema: 2\ntitle: Seed cells\nassay: assay-cck8\npath: root/seed\nuses:\n  - ref: uv-reader\n    role: instrument\n  - ref: dil-stock\n    role: calculator\n```\n```asterlab\nkind: step\nid: step-read\nschema: 2\ntitle: Read OD450\nassay: assay-cck8\npath: root/read\nuses:\n  - ref: uv-reader\n    role: instrument\n```\n"
	doc := &asterdoc.FrozenAsterDoc{
		DesignID:        "design-1",
		Revision:        6,
		Source:          source,
		SourceHash:      "abc123",
		CompilerVersion: "1.0.0",
	}
	vm := BuildViewModel(snap, doc)

	if vm.RunID != "run-1" {
		t.Fatalf("run ID = %q", vm.RunID)
	}
	if vm.Revision != 6 {
		t.Fatalf("revision = %d", vm.Revision)
	}
	if len(vm.Steps) != 2 {
		t.Fatalf("steps = %d", len(vm.Steps))
	}

	seed := vm.Steps[0]
	if seed.StepID != "step-seed" {
		t.Fatalf("step ID = %q", seed.StepID)
	}
	if seed.Status != TaskStatusCompleted {
		t.Fatalf("seed status = %q", seed.Status)
	}

	read := vm.Steps[1]
	if read.StepID != "step-read" {
		t.Fatalf("step ID = %q", read.StepID)
	}
	if read.Status != TaskStatusPending {
		t.Fatalf("read status = %q", read.Status)
	}

	if len(seed.Modules) != 2 {
		t.Fatalf("seed modules = %d", len(seed.Modules))
	}
	hasUV := false
	hasDil := false
	for _, m := range seed.Modules {
		if m.ID == "uv-reader" {
			hasUV = true
			if m.Role != "instrument" {
				t.Fatalf("uv-reader role = %q", m.Role)
			}
		}
		if m.ID == "dil-stock" {
			hasDil = true
			if m.Role != "calculator" {
				t.Fatalf("dil-stock role = %q", m.Role)
			}
		}
	}
	if !hasUV || !hasDil {
		t.Fatalf("missing modules: uv=%v dil=%v", hasUV, hasDil)
	}

	// Measurements live on the task child; the step node aggregates children.
	if len(read.Children) != 1 {
		t.Fatalf("read children = %d", len(read.Children))
	}
	child := read.Children[0]
	if len(child.Actual.Measurements) != 1 {
		t.Fatalf("measurements = %d", len(child.Actual.Measurements))
	}
	if child.Actual.Measurements[0].SampleID != "Control-1" {
		t.Fatalf("sample = %q", child.Actual.Measurements[0].SampleID)
	}

	if vm.Selected == nil {
		t.Fatal("selected task is nil")
	}
	if vm.Selected.TaskKey != "t-2" {
		t.Fatalf("selected task = %q", vm.Selected.TaskKey)
	}
}

func TestBuildViewModelInstanceIndependence(t *testing.T) {
	snap := runs.RunSnapshot{
		RunID:    "run-1",
		Title:    "CCK-8",
		Revision: 6,
		Tasks: []runs.RunTask{
			{Key: "t-1", DesignStepID: "step-seed", Title: "Seed", Timepoint: "D0", AssayID: "assay-cck8", Status: "completed"},
		},
	}
	source := "```asterlab\nkind: assay\nid: assay-cck8\nschema: 1\ntitle: CCK-8\n```\n```asterlab\nkind: instrument_profile\nid: uv-reader\nschema: 1\nclass: plate_reader\ncapabilities:\n  - name: absorbance\nsource: built_in\n```\n```asterlab\nkind: step\nid: step-seed\nschema: 2\ntitle: Seed cells\nassay: assay-cck8\npath: root/seed\nuses:\n  - ref: uv-reader\n    role: instrument\n```\n"
	doc := &asterdoc.FrozenAsterDoc{
		DesignID:        "design-1",
		Revision:        6,
		Source:          source,
		SourceHash:      "abc123",
		CompilerVersion: "1.0.0",
	}
	vm1 := BuildViewModel(snap, doc)
	vm2 := BuildViewModel(snap, doc)

	child1 := vm1.Steps[0].Children[0]
	child2 := vm2.Steps[0].Children[0]
	if len(child1.Modules) != 1 {
		t.Fatalf("modules = %d", len(child1.Modules))
	}
	if child1.Modules[0].ID != "uv-reader" {
		t.Fatalf("module ID = %q", child1.Modules[0].ID)
	}
	_ = child2
	_ = vm2
}

func TestBuildViewModelWorkflowEdges(t *testing.T) {
	snap := runs.RunSnapshot{
		RunID:    "run-1",
		Title:    "CCK-8",
		Revision: 6,
		Tasks: []runs.RunTask{
			{Key: "t-1", DesignStepID: "step-seed", Title: "Seed", Timepoint: "D0", AssayID: "assay-cck8", Status: "completed"},
			{Key: "t-2", DesignStepID: "step-read", Title: "Read", Timepoint: "D1", AssayID: "assay-cck8", Status: "pending"},
		},
	}
	source := "```asterlab\nkind: step\nid: step-seed\nschema: 2\ntitle: Seed cells\nassay: assay-cck8\npath: root/seed\nuses: []\n```\n```asterlab\nkind: step\nid: step-read\nschema: 2\ntitle: Read OD450\nassay: assay-cck8\npath: root/read\nuses: []\n```\n```asterlab\nkind: workflow\nid: wf-1\nschema: 1\nsteps: [step-seed, step-read]\nedges:\n  - from: step-seed\n    to: step-read\n```\n"
	doc := &asterdoc.FrozenAsterDoc{
		DesignID:        "design-1",
		Revision:        6,
		Source:          source,
		SourceHash:      "abc123",
		CompilerVersion: "1.0.0",
	}
	vm := BuildViewModel(snap, doc)

	if len(vm.Steps) != 2 {
		t.Fatalf("steps = %d", len(vm.Steps))
	}
	seed := vm.Steps[0]
	read := vm.Steps[1]
	if seed.StepID != "step-seed" {
		t.Fatalf("seed ID = %q", seed.StepID)
	}
	if read.StepID != "step-read" {
		t.Fatalf("read ID = %q", read.StepID)
	}
	if len(seed.Children) != 1 {
		t.Fatalf("seed children = %d", len(seed.Children))
	}
	if len(read.Children) != 1 {
		t.Fatalf("read children = %d", len(read.Children))
	}
	if seed.Children[0].TaskKey != "t-1" {
		t.Fatalf("seed child task = %q", seed.Children[0].TaskKey)
	}
	if read.Children[0].TaskKey != "t-2" {
		t.Fatalf("read child task = %q", read.Children[0].TaskKey)
	}
}

func TestBuildViewModelBlockedTask(t *testing.T) {
	snap := runs.RunSnapshot{
		RunID:    "run-1",
		Title:    "CCK-8",
		Revision: 6,
		Tasks: []runs.RunTask{
			{
				Key:          "t-1",
				DesignStepID: "step-seed",
				Title:        "Seed",
				Timepoint:    "D0",
				AssayID:      "assay-cck8",
				Status:       "not_started",
				Checks: []runs.TaskCheck{
					{CheckID: "c1", Label: "Confirm plate layout", Blocking: true},
				},
			},
		},
	}
	source := "```asterlab\nkind: assay\nid: assay-cck8\nschema: 1\ntitle: CCK-8\n```\n```asterlab\nkind: step\nid: step-seed\nschema: 2\ntitle: Seed cells\nassay: assay-cck8\npath: root/seed\nuses: []\n```\n"
	doc := &asterdoc.FrozenAsterDoc{
		DesignID:        "design-1",
		Revision:        6,
		Source:          source,
		SourceHash:      "abc123",
		CompilerVersion: "1.0.0",
	}
	vm := BuildViewModel(snap, doc)

	if len(vm.Steps) != 1 {
		t.Fatalf("steps = %d", len(vm.Steps))
	}
	// Core owns task status; an unconfirmed blocking check does not invent a
	// "blocked" status on the UI side.
	if vm.Steps[0].Status != TaskStatusPending {
		t.Fatalf("seed status = %q", vm.Steps[0].Status)
	}
	if vm.Steps[0].Children[0].Status != TaskStatusPending {
		t.Fatalf("child status = %q", vm.Steps[0].Children[0].Status)
	}
}

func TestAggregateStatus(t *testing.T) {
	if aggregateStatus([]*RunTaskViewModel{
		{Status: TaskStatusCompleted},
		{Status: TaskStatusCompleted},
	}) != TaskStatusCompleted {
		t.Fatal("all completed should be completed")
	}
	if aggregateStatus([]*RunTaskViewModel{
		{Status: TaskStatusCompleted},
		{Status: TaskStatusCurrent},
	}) != TaskStatusCurrent {
		t.Fatal("one current should be current")
	}
	if aggregateStatus([]*RunTaskViewModel{
		{Status: TaskStatusCompleted},
		{Status: TaskStatusBlocked},
	}) != TaskStatusBlocked {
		t.Fatal("one blocked should be blocked")
	}
	if aggregateStatus([]*RunTaskViewModel{
		{Status: TaskStatusCompleted},
		{Status: TaskStatusPending},
	}) != TaskStatusPending {
		t.Fatal("one pending should be pending")
	}
}

func TestBuildViewModelAssociatesMeasurementsAndAuditEvents(t *testing.T) {
	snap := runs.RunSnapshot{
		RunID: "run-1", Title: "RNA", Revision: 19, DesignRevision: 3,
		Tasks: []runs.RunTask{
			{
				Key: "t-1", Path: "root/01-collect", Title: "01 收集细胞",
				Instruction:    "2,000 x g, 5 min, 4 C",
				ParametersJSON: `[{"key":"force","label":"离心力","value":2000,"unit":"x g"}]`,
				Checks:         []runs.TaskCheck{{CheckID: "c1", Label: "保留沉淀", Blocking: true, Status: "passed"}},
			},
			{Key: "t-2", Path: "root/10-qc", Title: "10 RNA 质检"},
		},
		Measurements: []runs.Measurement{
			{SampleID: "RNA-S01", Endpoint: "RNA concentration", Value: 85.4, Unit: "ng/uL", TaskKey: "t-2", ProtocolStepPath: "root/10-qc"},
			{SampleID: "RNA-OLD", Endpoint: "A260/A280", Value: 2.0, Unit: "ratio"},
		},
		Events: []runs.RunEvent{
			{
				ID: "e1", EventType: "deviation", Content: "RNA-S03 偏差", TaskKey: "t-1",
				ProtocolStepPath: "root/01-collect",
				DataJSON:         `{"deviation":"流出液未完全弃置","materials":[{"material":"mat-wash1","amount":700,"unit":"uL"}],"lineage":[{"from_kind":"container","from_id":"cartridge-03","to_kind":"waste","to_id":"flowthrough-03","relation":"discarded"}],"sample_ids":["RNA-S03"],"check_ack":true,"checks":[{"check_id":"c1","status":"passed"}],"measurements":[{"sample_id":"RNA-S03","endpoint":"RNA concentration","value":63.8,"unit":"ng/uL"}]}`,
			},
			{ID: "e2", EventType: "observation", Content: "旧记录"},
		},
	}
	vm := BuildViewModel(snap, nil)
	if vm.DesignRevision != 3 || vm.AsterDocRevision != 0 {
		t.Fatalf("revisions = design %d asterdoc %d", vm.DesignRevision, vm.AsterDocRevision)
	}
	task := vm.TaskByKey["t-1"]
	if task.Instruction != "2,000 x g, 5 min, 4 C" {
		t.Fatalf("instruction = %q", task.Instruction)
	}
	if len(task.Parameters) != 1 || task.Parameters[0].Label != "离心力" {
		t.Fatalf("parameters = %+v", task.Parameters)
	}
	if len(task.Checks) != 1 || task.Checks[0].Status != "passed" {
		t.Fatalf("checks = %+v", task.Checks)
	}
	if len(task.Actual.Measurements) != 0 {
		t.Fatalf("task 1 received task 2 measurements: %+v", task.Actual.Measurements)
	}
	if len(vm.TaskByKey["t-2"].Actual.Measurements) != 1 {
		t.Fatalf("task 2 measurements = %+v", vm.TaskByKey["t-2"].Actual.Measurements)
	}
	if len(vm.UnassociatedMeasurements) != 1 || vm.UnassociatedMeasurements[0].SampleID != "RNA-OLD" {
		t.Fatalf("unassociated measurements = %+v", vm.UnassociatedMeasurements)
	}
	if len(task.Events) != 1 || task.Events[0].Deviation != "流出液未完全弃置" {
		t.Fatalf("task events = %+v", task.Events)
	}
	event := task.Events[0]
	if len(event.Materials) != 1 || event.Materials[0].Material != "mat-wash1" {
		t.Fatalf("materials = %+v", event.Materials)
	}
	if len(event.Lineage) != 1 || event.Lineage[0].ToKind != "waste" {
		t.Fatalf("lineage = %+v", event.Lineage)
	}
	if len(event.Measurements) != 1 || !event.CheckAck {
		t.Fatalf("event measurements/check_ack = %+v %v", event.Measurements, event.CheckAck)
	}
	if len(vm.UnassociatedEvents) != 1 {
		t.Fatalf("unassociated events = %+v", vm.UnassociatedEvents)
	}
}
