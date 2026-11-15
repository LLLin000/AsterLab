package asterdoc

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestSharedParityFixture consumes the same sample file as the Rust test
// `asterdoc_parity_samples`, so a validator drift shows up on both sides from
// one source of truth.
func TestSharedParityFixture(t *testing.T) {
	raw, err := os.ReadFile("../../../../tests/asterdoc_parity_samples.json")
	if err != nil {
		t.Fatalf("read shared parity fixture: %v", err)
	}
	var samples []struct {
		Name   string `json:"name"`
		Valid  bool   `json:"valid"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(raw, &samples); err != nil {
		t.Fatalf("parse shared parity fixture: %v", err)
	}
	if len(samples) < 8 {
		t.Fatalf("fixture has %d samples, want at least 8", len(samples))
	}
	for _, sample := range samples {
		sample := sample
		t.Run(sample.Name, func(t *testing.T) {
			_, err := Parse(sample.Source)
			if (err == nil) != sample.Valid {
				t.Fatalf("valid=%v but err=%v", sample.Valid, err)
			}
		})
	}
}

// The Rust mirror is `calculators::tests` in crates/asterlab-core. Both sides
// pin the same HUVEC case: 3%/5%/7% GelMA x 3 replicate wells on a 12-well
// plate, 200 uL/well, 10% excess, 0.5% LAP stock diluted to 0.05%.
const plateMixSource = "```asterlab\nkind: group\nid: \"3%\"\nschema: 1\nname: 3% GelMA\n```\n```asterlab\nkind: group\nid: \"5%\"\nschema: 1\nname: 5% GelMA\n```\n```asterlab\nkind: group\nid: \"7%\"\nschema: 1\nname: 7% GelMA\n```\n```asterlab\nkind: plate\nid: plate-huvec\nschema: 1\nformat: 12\ngroups:\n  \"3%\": [A1, A2, A3]\n  \"5%\": [B1, B2, B3]\n  \"7%\": [C1, C2, C3]\nmix_volume_per_well: {value: 200, unit: uL}\nmix_excess_ratio: 0.1\nmix_gelma_percent_by_group:\n  \"3%\": 3\n  \"5%\": 5\n  \"7%\": 7\nmix_lap_stock_percent: 0.5\nmix_lap_final_percent: 0.05\nmix_cell_density: {value: 20000, unit: cells/well}\nmix_factor_stock_concentration: {value: 10, unit: ug/mL}\nmix_factor_final_concentration: {value: 50, unit: ng/mL}\n```\n"

func TestPlateMixPlanDerivesGelAndSeedingRows(t *testing.T) {
	doc, err := Parse(plateMixSource)
	if err != nil {
		t.Fatalf("parse plate mix: %v", err)
	}
	if len(doc.Modules) != 4 {
		t.Fatalf("modules = %+v", doc.Modules)
	}
	plate, ok := doc.PlateByID("plate-huvec")
	if !ok || plate == nil {
		t.Fatalf("plate missing: %+v", doc.Modules)
	}
	var plateData map[string]any
	for _, module := range doc.Modules {
		if module.Kind == "plate" {
			plateData = module.Data
		}
	}
	gel, cells, err := plateMixPlansFor(plateData)
	if err != nil {
		t.Fatalf("plateMixPlansFor: %v", err)
	}
	if len(gel.Groups) != 3 || len(cells.Groups) != 3 {
		t.Fatalf("gel rows=%d cell rows=%d", len(gel.Groups), len(cells.Groups))
	}
	first := gel.Groups[0]
	if first.Name != "3%" || first.Wells != 3 ||
		!closeEnough(first.VolumeUL, 660) ||
		!closeEnough(first.GelmaMassMG, 19.8) ||
		!closeEnough(first.LapStockVolumeUL, 66) ||
		!closeEnough(first.SolventVolumeUL, 594) {
		t.Fatalf("3%% row = %+v", first)
	}
	if !closeEnough(gel.VolumeUL, 1980) || !closeEnough(gel.GelmaMassMG, 99) ||
		!closeEnough(gel.LapStockVolumeUL, 198) || !closeEnough(gel.SolventVolumeUL, 1782) {
		t.Fatalf("gel totals = %+v", gel)
	}
	if cells.TotalCells != 198000 || !closeEnough(cells.SuspensionVolumeUL, 1980) ||
		!closeEnough(cells.FactorStockVolumeUL, 9.9) {
		t.Fatalf("cell totals = %+v", cells)
	}
}

func TestPlateMixRejectsInconsistentPlans(t *testing.T) {
	cases := map[string]string{
		"lap final above stock":    strings.Replace(plateMixSource, "mix_lap_final_percent: 0.05", "mix_lap_final_percent: 0.6", 1),
		"missing group percent":    strings.Replace(plateMixSource, "  \"7%\": 7\n", "", 1),
		"unknown percent group":    strings.Replace(plateMixSource, "mix_lap_stock_percent", "  \"9%\": 9\nmix_lap_stock_percent", 1),
		"factor final above stock": strings.Replace(plateMixSource, "mix_factor_final_concentration: {value: 50, unit: ng/mL}", "mix_factor_final_concentration: {value: 20, unit: mg/mL}", 1),
		"volume without plan":      "```asterlab\nkind: plate\nid: p\nschema: 1\nformat: 12\ngroups:\n  a: [A1]\nmix_volume_per_well: {value: 200, unit: uL}\n```\n",
	}
	for name, source := range cases {
		name, source := name, source
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(source); err == nil {
				t.Fatalf("%s was accepted", name)
			}
		})
	}
}

func closeEnough(actual, expected float64) bool {
	delta := actual - expected
	return delta < 1e-9 && delta > -1e-9
}

func TestManifestDeclaresPlateMixDerivedOutputs(t *testing.T) {
	spec, ok := BuiltinRegistry().Lookup("plate", 1)
	if !ok {
		t.Fatal("plate@1 missing")
	}
	outputs := map[string]Port{}
	for _, port := range spec.Outputs {
		outputs[port.Name] = port
	}
	for name, calculator := range map[string]string{
		"mix_gel_mastermix": "gel.mastermix.v1",
		"mix_cell_seeding":  "cell.seeding.v1",
	} {
		port, ok := outputs[name]
		if !ok {
			t.Fatalf("plate missing derived output %s", name)
		}
		if port.Origin != "derived" || port.Calculator != calculator {
			t.Fatalf("%s = %+v", name, port)
		}
	}
}
func TestManifestRuleMustBeRegisteredInGo(t *testing.T) {
	module := specModule{Descriptor: Descriptor{
		Kind: "probe", Schema: 1, SDK: "1.0.0", Category: "workflow",
		Rules: []string{"not-a-real-rule"},
	}}
	err := validateSpecModule(module, "1.0.0")
	if err == nil || !strings.Contains(err.Error(), "unregistered rule") {
		t.Fatalf("validateSpecModule() = %v", err)
	}
}
