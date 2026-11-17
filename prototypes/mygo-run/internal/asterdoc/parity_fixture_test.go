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
// plate, 200 uL/well, 10% excess, 0.5% LAP stock diluted to 0.05%, plus VEGF
// dilution from 10 ug/mL to 50 ng/mL.
const huvecMastermixSource = "```asterlab\nkind: group_table\nid: gt-huvec\nschema: 1\ntitle: HUVEC Groups\nrows:\n  - id: \"3%\"\n    name: 3% GelMA\n  - id: \"5%\"\n    name: 5% GelMA\n  - id: \"7%\"\n    name: 7% GelMA\n```\n```asterlab\nkind: plate\nid: plate-huvec\nschema: 1\nformat: 12\ngroups:\n  \"3%\": [A1, A2, A3]\n  \"5%\": [B1, B2, B3]\n  \"7%\": [C1, C2, C3]\n```\n```asterlab\nkind: calculator\nid: calc-gelma\nschema: 1\ntitle: GelMA & LAP Mastermix\nmode: group_mastermix\nplate_ref: plate-huvec\nvolume_per_unit: {value: 200, unit: uL}\nexcess_ratio: 0.1\ncomponents:\n  - name: GelMA\n    target_percent: 5\n  - name: LAP\n    stock_percent: 0.5\n    target_percent: 0.05\n```\n```asterlab\nkind: calculator\nid: calc-vegf\nschema: 1\ntitle: VEGF Dilution\nmode: dilution\nstock: {value: 10, unit: ug/mL}\ntarget: {value: 50, unit: ng/mL}\nfinal_volume: {value: 1980, unit: uL}\n```\n"

func TestCalculatorMastermixAndDilutionParity(t *testing.T) {
	doc, err := Parse(huvecMastermixSource)
	if err != nil {
		t.Fatalf("parse huvec calculators: %v", err)
	}
	if len(doc.Modules) != 4 {
		t.Fatalf("modules = %+v", doc.Modules)
	}
	mix := EvalCalculator(doc, doc.Modules[2].Data)
	if mix.Err != nil {
		t.Fatalf("EvalCalculator(group_mastermix): %v", mix.Err)
	}
	if len(mix.Rows) != 5 {
		t.Fatalf("mastermix rows = %+v", mix.Rows)
	}
	// 3 groups x 3 wells x 200 uL x 1.1 = 1980 uL; each group = 660 uL; 5% w/v GelMA = 99 mg; 0.05/0.5 LAP = 198 uL.
	if !closeEnough(mix.Rows[0].Amount, 660) ||
		!closeEnough(mix.Rows[3].Amount, 99) ||
		!closeEnough(mix.Rows[4].Amount, 198) {
		t.Fatalf("mastermix rows = %+v", mix.Rows)
	}
	dil := EvalCalculator(doc, doc.Modules[3].Data)
	if dil.Err != nil {
		t.Fatalf("EvalCalculator(dilution): %v", dil.Err)
	}
	if len(dil.Rows) < 2 || !closeEnough(dil.Rows[0].Amount, 9.9) || !closeEnough(dil.Rows[1].Amount, 1970.1) || dil.Rows[0].Unit != "uL" {
		t.Fatalf("vegf dilution = %+v", dil)
	}
}

func TestCalculatorRejectsInconsistentInputs(t *testing.T) {
	cases := map[string]string{
		"negative excess ratio":   strings.Replace(huvecMastermixSource, "excess_ratio: 0.1", "excess_ratio: -0.1", 1),
		"target exceeds stock":    strings.Replace(huvecMastermixSource, "target: {value: 50, unit: ng/mL}", "target: {value: 20, unit: ug/mL}", 1),
		"invalid component empty": strings.Replace(huvecMastermixSource, "name: GelMA", "name: ''", 1),
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

func TestManifestDeclaresCalculatorDerivedOutputs(t *testing.T) {
	spec, ok := BuiltinRegistry().Lookup("calculator", 1)
	if !ok {
		t.Fatal("calculator@1 missing")
	}
	outputs := map[string]Port{}
	for _, port := range spec.Outputs {
		outputs[port.Name] = port
	}
	port, ok := outputs["derived_table"]
	if !ok {
		t.Fatalf("calculator missing derived output derived_table")
	}
	if port.Origin != "derived" || port.Calculator != "calculator.v2" {
		t.Fatalf("derived_table = %+v", port)
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
