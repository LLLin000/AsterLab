package runworkspace

import (
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/modules/runs"
)

// makeSnapshot is the shared test fixture: a two-step CCK-8 run.
func makeSnapshot() runs.RunSnapshot {
	return runs.RunSnapshot{
		RunID:    "run-cck8",
		Title:    "L929 · CCK-8",
		Revision: 6,
		Status:   "running",
		Tasks: []runs.RunTask{
			{
				Key:          "t-seed",
				DesignStepID: "step-seed",
				Title:        "细胞接种",
				Path:         "root/seed",
				Timepoint:    "D0",
				AssayID:      "assay-cck8",
				Status:       "completed",
			},
			{
				Key:            "t-read",
				DesignStepID:   "step-read",
				Title:          "CCK-8 检测",
				Path:           "root/read",
				Instruction:    "读取 OD450 并记录。",
				ParametersJSON: `[{"key":"od450","label":"OD450","value":0.82,"unit":"AU"}]`,
				Timepoint:      "D1",
				AssayID:        "assay-cck8",
				Status:         "not_started",
				Checks: []runs.TaskCheck{
					{CheckID: "c-plate", Label: "板与密度已确认", Blocking: true},
					{CheckID: "c-read", Label: "读数已复核", Blocking: false},
				},
			},
		},
		Measurements: []runs.Measurement{
			{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU", TaskKey: "t-read", ProtocolStepPath: "root/read"},
		},
	}
}

// makeFrozenDoc is the shared frozen AsterDoc: experiment + assay + steps with uses.
func makeFrozenDoc() *asterdoc.FrozenAsterDoc {
	src := "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: CCK-8\n```\n" +
		"```asterlab\nkind: group\nid: g-control\nschema: 1\nname: Control\nrole: control\n```\n" +
		"```asterlab\nkind: assay\nid: assay-cck8\nschema: 1\ntitle: CCK-8\nendpoint: OD450\nunit: AU\ngroup_refs: [g-control]\ntimepoints: [D1]\n```\n" +
		"```asterlab\nkind: instrument_profile\nid: uv-reader\nschema: 1\nclass: plate_reader\nmodel: UV-1\ncapabilities:\n  - name: absorbance\nsource: built_in\n```\n" +
		"```asterlab\nkind: calculator\nid: dil-stock\nschema: 1\nmode: dilution\ntitle: 药物稀释\nstock:\n  value: 20\n  unit: mg/mL\ntarget:\n  value: 2\n  unit: mg/mL\nfinal_volume:\n  value: 5\n  unit: mL\n```\n" +
		"```asterlab\nkind: step\nid: step-seed\nschema: 1\ntitle: 细胞接种\nassay: assay-cck8\npath: root/seed\ntimepoints: [D0]\nuses:\n  - ref: uv-reader\n    role: instrument\n```\n" +
		"```asterlab\nkind: step\nid: step-read\nschema: 1\ntitle: CCK-8 检测\nassay: assay-cck8\npath: root/read\ntimepoints: [D1]\nuses:\n  - ref: uv-reader\n    role: instrument\n  - ref: dil-stock\n    role: calculator\n```\n" +
		"```asterlab\nkind: workflow\nid: wf-1\nschema: 1\nsteps: [step-seed, step-read]\nedges:\n  - from: step-seed\n    to: step-read\n```\n"
	return &asterdoc.FrozenAsterDoc{
		DesignID:        "design-cck8",
		Revision:        6,
		Source:          src,
		SourceHash:      "sha-cck8",
		CompilerVersion: "1.0.0",
	}
}
