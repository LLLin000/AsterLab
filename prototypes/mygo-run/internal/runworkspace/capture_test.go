package runworkspace

import (
	"strings"
	"testing"
)

func captureAssets() []LinkedAsset {
	return []LinkedAsset{
		{AssetID: "a1", WellID: "A1", Timepoint: "6h", FOVIndex: 1},
		{AssetID: "a2", WellID: "A1", Timepoint: "6h", FOVIndex: 2},
		{AssetID: "a3", WellID: "A1", Timepoint: "6h", FOVIndex: 2},
		{AssetID: "a4", WellID: "A2", Timepoint: "6h", FOVIndex: 1},
		{AssetID: "a5", WellID: "A2", Timepoint: "6h", FOVIndex: 3},
		{AssetID: "a6", WellID: "", Timepoint: "6h", FOVIndex: 1},
		{AssetID: "a7", WellID: "A3", Timepoint: "6h", FOVIndex: 0},
		{AssetID: "a8", WellID: "B1", Timepoint: "12h", FOVIndex: 1},
	}
}

func TestCoverageByWellMirrorsCoreGrouping(t *testing.T) {
	cells := CoverageByWell(captureAssets(), []string{"A1", "A2", "A3"}, "6h", 6)
	byWell := map[string]CoverageCell{}
	for _, cell := range cells {
		if cell.Timepoint == "6h" {
			byWell[cell.WellID] = cell
		}
	}
	a1 := byWell["A1"]
	// FOV 2 linked twice counts once; trailing slots 3..6 are gaps.
	if len(a1.FOVs) != 2 || a1.FOVs[0] != 1 || a1.FOVs[1] != 2 {
		t.Fatalf("A1 FOVs = %v", a1.FOVs)
	}
	if len(a1.Gaps) != 4 || a1.Gaps[0] != 3 || a1.Gaps[3] != 6 {
		t.Fatalf("A1 gaps = %v", a1.Gaps)
	}
	a2 := byWell["A2"]
	if len(a2.Gaps) != 4 { // 2, 4, 5, 6 (3 recorded, 1 recorded)
		t.Fatalf("A2 gaps = %v, want 4", a2.Gaps)
	}
	for _, gap := range a2.Gaps {
		if gap == 1 || gap == 3 {
			t.Fatalf("A2 gap %d collides with a recorded slot", gap)
		}
	}
	// Links without a well or with FOV 0 never form groups.
	for _, cell := range cells {
		if cell.WellID == "" {
			t.Fatalf("empty well formed a group: %+v", cell)
		}
	}
	if _, ok := byWell["B1"]; ok {
		t.Fatal("another timepoint must not leak into the 6h rows")
	}
	// The plan leads: A3 has no links yet still reads 0/6 with 6 gaps.
	a3 := byWell["A3"]
	if a3.Filled != 0 || a3.Slots != 6 || len(a3.Gaps) != 6 {
		t.Fatalf("A3 = %+v, want empty 0/6", a3)
	}
}

func TestPlanBatchDropsRollsToNextWell(t *testing.T) {
	full := []LinkedAsset{}
	for i := uint32(1); i <= 6; i++ {
		full = append(full, LinkedAsset{AssetID: "f", WellID: "A1", Timepoint: "6h", FOVIndex: i})
	}
	full = append(full,
		LinkedAsset{AssetID: "g1", WellID: "A2", Timepoint: "6h", FOVIndex: 1},
		LinkedAsset{AssetID: "g2", WellID: "A2", Timepoint: "6h", FOVIndex: 3},
	)
	wells := []string{"A1", "A2", "A3"}
	cov := CoverageByWell(full, wells, "6h", 6)
	drops := PlanBatchDrops(wells, "6h", 6, cov, 4)
	// A1 is full (6/6): the plan skips it. A2 misses 2,4,5,6: first two drops
	// fill FOV 2 and 4; nothing is double-booked.
	if len(drops) != 4 {
		t.Fatalf("drops = %+v", drops)
	}
	if drops[0] != (BatchDrop{WellID: "A2", Timepoint: "6h", FOVIndex: 2}) {
		t.Fatalf("first drop = %+v", drops[0])
	}
	if drops[1] != (BatchDrop{WellID: "A2", Timepoint: "6h", FOVIndex: 4}) {
		t.Fatalf("second drop = %+v", drops[1])
	}
	if drops[2].WellID != "A2" || drops[2].FOVIndex != 5 {
		t.Fatalf("third drop = %+v", drops[2])
	}
	if drops[3].WellID != "A2" || drops[3].FOVIndex != 6 {
		t.Fatalf("fourth drop = %+v", drops[3])
	}
	// More files than empty slots: the plan returns what fits, never invents.
	many := PlanBatchDrops(wells, "6h", 6, cov, 20)
	// A2 has 4 gaps + A3 has 6 = 10 slots.
	if len(many) != 10 {
		t.Fatalf("overflow drops = %d, want 10", len(many))
	}
	last := many[len(many)-1]
	if last.WellID != "A3" || last.FOVIndex != 6 {
		t.Fatalf("last drop = %+v", last)
	}
	if drops := PlanBatchDrops(wells, "6h", 0, cov, 3); drops != nil {
		t.Fatalf("zero FOV plan must assign nothing, got %+v", drops)
	}
}

func TestFOVPerWellForStepReadsFrozenPlan(t *testing.T) {
	source := "```asterlab\nkind: group\nid: g-tube\nschema: 1\nname: 成管\nrole: sample\n```\n" +
		"```asterlab\nkind: assay\nid: assay-tube\nschema: 1\ntitle: 成管\nendpoint: 图像\nunit: images\ngroup_refs: [g-tube]\ntimepoints: [6h]\n```\n" +
		"```asterlab\nkind: step\nid: step-img\nschema: 2\ntitle: 采图\nassay: assay-tube\npath: root/img\nuses:\n  - ref: mp-img\n    role: other\n```\n" +
		"```asterlab\nkind: measurement_profile\nid: mp-img\nschema: 1\nendpoint: 明场显微图像\nunit: images\nreadout_schema:\n  per_well_fov_count: 6\n  total_wells: 9\n```\n"
	if got := FOVPerWellForStep(source, "step-img"); got != 6 {
		t.Fatalf("FOV per well = %d, want 6", got)
	}
	if got := FOVPerWellForStep(source, "step-missing"); got != 0 {
		t.Fatalf("unknown step must declare nothing, got %d", got)
	}
	plain := "```asterlab\nkind: group\nid: g-tube\nschema: 1\nname: 成管\nrole: sample\n```\n" +
		"```asterlab\nkind: assay\nid: assay-tube\nschema: 1\ntitle: 成管\nendpoint: 图像\nunit: images\ngroup_refs: [g-tube]\ntimepoints: [6h]\n```\n" +
		"```asterlab\nkind: step\nid: step-img\nschema: 2\ntitle: 采图\nassay: assay-tube\npath: root/img\n```\n"
	if got := FOVPerWellForStep(plain, "step-img"); got != 0 {
		t.Fatalf("step without a profile must declare nothing, got %d", got)
	}
}

func TestSkipPayloadScope(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	input := SkipEventInput(task, "无气泡")
	if input.EventType != "skipped_by_rule" {
		t.Fatalf("event type = %q", input.EventType)
	}
	if input.StepPath != "root/read" || input.AssayID != "assay-cck8" || input.Timepoint != "D1" {
		t.Fatalf("scope = %+v", input)
	}
	if !strings.HasPrefix(input.Content, "按规则跳过") {
		t.Fatalf("content = %q", input.Content)
	}
	// A task without an assay stays unscoped: Core rejects a step reference
	// outside its assay, so the surface never invents one.
	bare := &RunTaskViewModel{TaskKey: "t-bare", StepPath: "root/bare", Timepoint: "D0"}
	unscoped := SkipEventInput(bare, "")
	if unscoped.StepPath != "" || unscoped.AssayID != "" {
		t.Fatalf("unscoped skip = %+v", unscoped)
	}
	deviation := DeviationEventInput(task, " 实际离心 5 分钟 ")
	if deviation.EventType != "deviation" || deviation.Deviation != "实际离心 5 分钟" {
		t.Fatalf("deviation = %+v", deviation)
	}
	observation := ObservationEventInput(task, " 3% 组固化 42 秒 ", "item-7")
	if observation.EventType != "observation" || observation.PantryItemID != "item-7" {
		t.Fatalf("observation = %+v", observation)
	}
}
