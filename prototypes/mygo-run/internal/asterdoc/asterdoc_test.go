package asterdoc

import (
	"encoding/json"
	"image/png"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

const sample = "# L929\n\n说明文字。\n\n```asterlab\nkind: assay\nid: cck8\nschema: 1\ntitle: CCK-8\n```\n\n```asterlab\nkind: group\nid: control\nschema: 1\nname: Control\nrole: control\n```\n\n```asterlab\nkind: group\nid: material\nschema: 1\nname: Material\nrole: treatment\n```\n\n```asterlab\nkind: plate\nid: plate-cck8-d3\nschema: 1\nformat: 96\nassay: cck8\ngroups:\n  control: [A1, A2, A3]\n  material: [B1, B2, B3]\n```\n\n尾注。\n"

func TestParseRoundTripsProseAndPlate(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Modules) != 4 {
		t.Fatalf("modules = %+v", doc.Modules)
	}
	plate, ok := doc.PlateByID("plate-cck8-d3")
	if !ok || plate == nil {
		t.Fatalf("plate missing: %+v", doc.Modules)
	}
	if plate.ID != "plate-cck8-d3" || len(plate.Groups["control"]) != 3 {
		t.Fatalf("plate = %+v", plate)
	}
	if doc.Source != sample {
		t.Fatal("parse must not rewrite the source")
	}
}

func TestUpdatePlateReplacesOnlyTheFenceBody(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	source, err := doc.UpdatePlate("plate-cck8-d3", map[string][]string{
		"control":  {"A1", "A2"},
		"material": {"B1", "C1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(source, "# L929") || !contains(source, "尾注。") {
		t.Fatalf("prose was not preserved:\n%s", source)
	}
	next, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	plate, ok := next.PlateByID("plate-cck8-d3")
	if !ok {
		t.Fatal("updated plate missing")
	}
	if got := plate.WellGroups(); got["A1"] != "control" || got["C1"] != "material" {
		t.Fatalf("wells = %+v", got)
	}
}

func TestRejectsDuplicateWellAndUnknownKind(t *testing.T) {
	duplicate := "```asterlab\nkind: plate\nid: p\nschema: 1\nformat: 96\ngroups:\n  a: [A1]\n  b: [A1]\n```\n"
	if _, err := Parse(duplicate); err == nil {
		t.Fatal("duplicate well accepted")
	}
	legacyPlate := "```asterlab\nkind: plate96\nid: p-legacy\nschema: 1\nformat: 96\ngroups: {}\n```\n"
	if _, err := Parse(legacyPlate); err == nil {
		t.Fatal("removed plate96 module kind accepted")
	}
	unknown := "```asterlab\nkind: calculator\nid: c\nschema: 1\n```\n"
	if _, err := Parse(unknown); err == nil {
		t.Fatal("unknown module kind accepted")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestViewRendersNativePlateAndAssignRewritesSource(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	var changed string
	v := &View{Doc: doc, Allowed: map[string]bool{PermDocumentEdit: true},
		OnChange: func(source string) { changed = source }}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		v.Render(c)
	}, 1000, 900)
	tt.Frame()
	if f, err := os.Create("evidence-asterdoc.png"); err == nil {
		png.Encode(f, tt.Image())
		f.Close()
	}
	if !tt.HasText("96 孔板") || !tt.HasText("plate-cck8-d3") {
		t.Fatalf("plate module did not render: %q", tt.Texts())
	}
	if err := v.Assign([]string{"A1"}, "material"); err != nil {
		t.Fatal(err)
	}
	if changed == "" {
		t.Fatal("assign did not report a new source")
	}
	next, err := Parse(changed)
	if err != nil {
		t.Fatal(err)
	}
	plate, _ := next.PlateByID("plate-cck8-d3")
	if plate.WellGroups()["A1"] != "material" {
		t.Fatalf("A1 was not moved: %+v", plate.WellGroups())
	}
}

func TestRegistryManifestDescribesInputsOutputsAndPermissions(t *testing.T) {
	descriptors := BuiltinRegistry().Descriptors()
	if len(descriptors) != 17 {
		t.Fatalf("expected 17 module kinds, got %d", len(descriptors))
	}
	var plate *Descriptor
	for i := range descriptors {
		if descriptors[i].Kind == "plate" {
			plate = &descriptors[i]
		}
		if descriptors[i].Kind == "plate96" {
			t.Fatal("duplicate plate96 module is still registered")
		}
	}
	if plate == nil {
		t.Fatal("plate descriptor missing")
	}
	inputs := map[string]bool{}
	for _, in := range plate.Inputs {
		inputs[in.Name] = true
	}
	if !inputs["groups"] || !inputs["id"] || !inputs["format"] || !inputs["samples"] || !inputs["replicates"] {
		t.Fatalf("plate inputs = %+v", plate.Inputs)
	}
	if len(plate.Actions) == 0 || plate.Actions[0].Permission != PermDocumentEdit {
		t.Fatalf("plate actions = %+v", plate.Actions)
	}
	manifest, err := BuiltinRegistry().ManifestJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), "\"permission\": \"document.edit\"") {
		t.Fatalf("manifest lacks permissions:\n%s", manifest)
	}
}

func TestCustomModuleCanBeRegisteredAndParsed(t *testing.T) {
	registry := NewRegistry()
	err := registry.Register(ModuleSpec{
		Descriptor: Descriptor{Kind: "counter", Schema: 1, Title: "计数器",
			Inputs: []Port{{Name: "value", Type: "int"}}},
		Validate: func(data map[string]any) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	source := "```asterlab\nkind: counter\nid: c1\nschema: 1\nvalue: 3\n```\n"
	if _, err := Parse(source); err == nil {
		t.Fatal("built-in registry accepted an unknown module")
	}
	doc, err := ParseWith(source, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Modules) != 1 || doc.Modules[0].Kind != "counter" {
		t.Fatalf("modules = %+v", doc.Modules)
	}
}

func TestEditRequiresExplicitPermission(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	v := &View{Doc: doc}
	if err := v.Assign([]string{"A1"}, "material"); err == nil {
		t.Fatal("assign succeeded without document.edit")
	}
}

func TestPlateZoomOverlaySelectsWellsAndReportsAssignment(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	var assigned []string
	v := &View{
		Doc: doc, Allowed: map[string]bool{PermDocumentEdit: true},
		OnAssignWells: func(moduleID, group string, wells []string) {
			assigned = append([]string(nil), wells...)
		},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		v.Render(c)
	}, 1200, 900)
	tt.Frame()
	if err := tt.Click("放大孔板"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("重置缩放") || !tt.HasText("关闭放大孔板") {
		t.Fatalf("zoom overlay missing floating toolbar: %q", tt.Texts())
	}
	if err := tt.Click("孔 A1"); err != nil {
		t.Fatalf("click A1: %v", err)
	}
	tt.Frame()
	if !tt.HasText("1 孔") {
		t.Fatalf("selection missing: %q", tt.Texts())
	}
	if f, err := os.Create("evidence-plate-zoom.png"); err == nil {
		png.Encode(f, tt.Image())
		f.Close()
	}
	if err := tt.Click("分配到 control"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if len(assigned) != 1 || assigned[0] != "A1" {
		t.Fatalf("assigned = %+v", assigned)
	}
	if err := tt.Click("选择第 1 列"); err != nil {
		t.Fatalf("column selection: %v", err)
	}
	tt.Frame()
	if !tt.HasText("8 孔") {
		t.Fatalf("column selection count missing: %q", tt.Texts())
	}
	if err := tt.Click("选择第 A 行"); err != nil {
		t.Fatalf("row selection: %v", err)
	}
	tt.Frame()
	if !tt.HasText("12 孔") {
		t.Fatalf("row selection count missing: %q", tt.Texts())
	}
	// Drag from A1 to A3: the grid's marquee selects the intersected cells.
	center := func(well string) (float32, float32) {
		rect, ok := tt.Find("孔 " + well)
		if !ok {
			t.Fatalf("well %s not found", well)
		}
		return rect.X + rect.W/2, rect.Y + rect.H/2
	}
	ax, ay := center("A1")
	cx, cy := center("A3")
	tt.Press(ax, ay)
	tt.Frame()
	tt.Move(cx, cy)
	tt.Frame()
	tt.Release(cx, cy)
	tt.Frame()
	if !tt.HasText("3 孔") {
		t.Fatalf("marquee selection missing: %q", tt.Texts())
	}
}

func TestPlateFormatsUseEditableOverlay(t *testing.T) {
	for _, tc := range []struct {
		format int
		well   string
	}{
		{6, "B3"},
		{12, "C4"},
		{24, "D6"},
		{48, "F8"},
		{96, "H12"},
		{384, "P24"},
	} {
		tc := tc
		t.Run(strconv.Itoa(tc.format), func(t *testing.T) {
			source := strings.Replace(sample, "format: 96", "format: "+strconv.Itoa(tc.format), 1)
			doc, err := Parse(source)
			if err != nil {
				t.Fatal(err)
			}
			var gotModule, gotGroup string
			var gotWells []string
			v := &View{
				Doc:     doc,
				Allowed: map[string]bool{PermDocumentEdit: true},
				OnAssignWells: func(moduleID, group string, wells []string) {
					gotModule, gotGroup = moduleID, group
					gotWells = append([]string(nil), wells...)
				},
			}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.LightTheme())
				v.Render(c)
			}, 1200, 900)
			tt.Frame()
			if err := tt.Click("放大孔板"); err != nil {
				t.Fatalf("%d-well overlay: %v", tc.format, err)
			}
			tt.Frame()
			if err := tt.Click("孔 " + tc.well); err != nil {
				t.Fatalf("%d-well %s selection: %v", tc.format, tc.well, err)
			}
			tt.Frame()
			if !tt.HasText("1 孔") {
				t.Fatalf("%d-well selection missing: %q", tc.format, tt.Texts())
			}
			if err := tt.Click("分配到 control"); err != nil {
				t.Fatalf("%d-well assignment: %v", tc.format, err)
			}
			if gotModule != "plate-cck8-d3" || gotGroup != "control" ||
				len(gotWells) != 1 || gotWells[0] != tc.well {
				t.Fatalf("%d-well assignment = %s/%s/%v", tc.format, gotModule, gotGroup, gotWells)
			}
		})
	}
}

func TestPlateGridLayoutKeepsCanvasAndScalesWells(t *testing.T) {
	formats := []int{6, 12, 24, 48, 96, 384}
	var viewportW, viewportH, previousCell float32
	for i, format := range formats {
		geometry, ok := geometryFor(format)
		if !ok {
			t.Fatalf("%d-well geometry is unavailable", format)
		}
		width, height, cell := plateGridLayout(geometry, 1)
		if i == 0 {
			viewportW, viewportH = width, height
		} else if width != viewportW || height != viewportH {
			t.Fatalf("%d-well viewport = %gx%g; want %gx%g",
				format, width, height, viewportW, viewportH)
		}
		if i > 0 && cell >= previousCell {
			t.Fatalf("%d-well cell = %g; want smaller than %g",
				format, cell, previousCell)
		}
		gridW := plateGridLabelWidth + plateGridGap +
			float32(geometry.Columns)*cell +
			float32(geometry.Columns-1)*plateGridGap
		gridH := plateGridHeaderHeight + plateGridGap +
			float32(geometry.Rows)*cell +
			float32(geometry.Rows-1)*plateGridGap
		if gridW > width+0.01 || gridH > height+0.01 {
			t.Fatalf("%d-well grid %gx%g exceeds viewport %gx%g",
				format, gridW, gridH, width, height)
		}
		previousCell = cell
	}
}

func TestHUVECTubeExperimentParses(t *testing.T) {
	raw, err := os.ReadFile("testdata/huvec-tube.asterml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Modules) != 10 {
		t.Fatalf("modules = %d", len(doc.Modules))
	}
	plate, ok := doc.PlateByID("plate-huvec")
	if !ok || plate.Format != 12 || len(plate.Groups) != 3 {
		t.Fatalf("plate = %+v", plate)
	}
}

func TestWellsInRectUsesGeometry(t *testing.T) {
	g, _ := geometryFor(12)
	all := WellsInRect(g, 24, 4, ui.Rect{X: 0, Y: 0, W: 3 * 28, H: 1 * 28})
	if len(all) != 3 || all[0] != "A1" || all[2] != "A3" {
		t.Fatalf("row selection = %+v", all)
	}
	none := WellsInRect(g, 24, 4, ui.Rect{X: -10, Y: -10, W: 5, H: 5})
	if len(none) != 0 {
		t.Fatalf("outside selection = %+v", none)
	}
}

const dilutionDoc = "```asterlab\nkind: dilution\nid: dilution-01\nschema: 1\nstock:\n  value: 20\n  unit: mg/mL\ntarget:\n  value: 2\n  unit: mg/mL\nfinal_volume:\n  value: 5\n  unit: mL\n```\n"

func TestDilutionModuleValidatesAndDerivesVolumes(t *testing.T) {
	doc, err := Parse(dilutionDoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Modules) != 1 || doc.Modules[0].Kind != "dilution" {
		t.Fatalf("modules = %+v", doc.Modules)
	}
	result, err := dilutionResultFor(doc.Modules[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stock != 0.5 || result.Diluent != 4.5 || result.Unit != "mL" {
		t.Fatalf("result = %+v", result)
	}
}

func TestDilutionRejectsUnknownUnitAndConcentration(t *testing.T) {
	for name, source := range map[string]string{
		"unknown unit":         strings.Replace(dilutionDoc, "unit: mg/mL", "unit: mg/ml", 1),
		"wrong dimension":      strings.Replace(dilutionDoc, "unit: mL", "unit: h", 1),
		"target exceeds stock": strings.Replace(dilutionDoc, "value: 2\n  unit: mg/mL", "value: 30\n  unit: mg/mL", 1),
	} {
		if _, err := Parse(source); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}

const instrumentDoc = "```asterlab\nkind: instrument_profile\nid: instrument-reader\nschema: 1\nclass: plate_reader\nmodel: Synergy H1\ncapabilities:\n  - name: absorbance\n    wavelength: 450 nm\n  - name: fluorescence\nsource: built_in\n```\n"

func TestInstrumentProfileDeclaresCapabilitiesWithoutHardware(t *testing.T) {
	doc, err := Parse(instrumentDoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Modules) != 1 || doc.Modules[0].Kind != "instrument_profile" {
		t.Fatalf("modules = %+v", doc.Modules)
	}
	if _, err := Parse(strings.Replace(instrumentDoc, "capabilities:\n  - name: absorbance\n    wavelength: 450 nm\n  - name: fluorescence\n", "", 1)); err == nil {
		t.Fatal("instrument profile without capabilities was accepted")
	}
}

func TestTypedReferenceReportsTargetKind(t *testing.T) {
	source := "```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: A\ngroup_refs: [missing-group]\n```\n"
	_, err := Parse(source)
	if err == nil || !strings.Contains(err.Error(), "field \"group_refs\" references missing group") {
		t.Fatalf("error = %v", err)
	}
}

func TestManifestDeclaresFoundationAndReferenceModules(t *testing.T) {
	manifest, err := BuiltinRegistry().ManifestJSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		FoundationVersion int          `json:"foundationVersion"`
		SDKVersion        string       `json:"sdkVersion"`
		Modules           []Descriptor `json:"modules"`
	}
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.FoundationVersion != 1 || decoded.SDKVersion != "1.0.0" || len(decoded.Modules) != 17 {
		t.Fatalf("manifest header/modules = %+v", decoded)
	}
	for _, module := range decoded.Modules {
		if module.SDK == "" || module.Category == "" || module.Renderers == nil || module.Provenance == nil {
			t.Fatalf("module %s lacks SDK metadata", module.Kind)
		}
	}
	text := string(manifest)
	for _, want := range []string{"\"sdk\": \"1.0.0\"", "\"category\": \"measurement\"", "dilution.v1", "instrument_profile", "\"renderers\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("manifest lacks %s:\n%s", want, text)
		}
	}
}

func TestFoundationSchemaIsShippedAndHasCoreDefinitions(t *testing.T) {
	var schema struct {
		Schema string                     `json:"$schema"`
		Defs   map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(FoundationSchema(), &schema); err != nil {
		t.Fatal(err)
	}
	if schema.Schema != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("schema dialect = %q", schema.Schema)
	}
	for _, name := range []string{"quantity", "reference", "artifact", "moduleManifest", "diagnostic", "templateManifest"} {
		if _, ok := schema.Defs[name]; !ok {
			t.Fatalf("schema lacks $defs.%s", name)
		}
	}
}

func TestManifestContractRejectsDerivedWithoutCalculator(t *testing.T) {
	module := specModule{Descriptor: Descriptor{
		Kind: "bad", Schema: 1, SDK: "1.0.0", Category: "measurement",
		Outputs: []Port{{Name: "x", Type: "quantity", Origin: "derived"}},
	}}
	if err := validateSpecModule(module, "1.0.0"); err == nil {
		t.Fatal("derived output without calculator was accepted")
	}
}

func TestStep2UsesStableModuleReferences(t *testing.T) {
	source := "```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: Assay\n```\n```asterlab\nkind: dilution\nid: dilution-01\nschema: 1\nstock: {value: 20, unit: mg/mL}\ntarget: {value: 2, unit: mg/mL}\nfinal_volume: {value: 5, unit: mL}\n```\n```asterlab\nkind: instrument_profile\nid: uv-1\nschema: 1\nclass: uv_curing\ncapabilities:\n  - name: 365 nm\nsource: built_in\n```\n```asterlab\nkind: step\nid: step-uv\nschema: 2\ntitle: UV cure\nassay: assay-1\nuses:\n  - ref: uv-1\n    role: instrument\n  - ref: dilution-01\n    role: calculator\n```\n"
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Modules) != 4 || doc.Modules[3].Schema != 2 {
		t.Fatalf("modules = %+v", doc.Modules)
	}
	if _, err := Parse(strings.Replace(source, "ref: uv-1", "ref: missing", 1)); err == nil {
		t.Fatal("missing module use was accepted")
	}
	if _, err := Parse(strings.Replace(source, "role: calculator", "role: instrument", 1)); err == nil {
		t.Fatal("calculator module accepted instrument role")
	}
	if _, err := Parse(strings.Replace(source, "role: calculator", "role: acme:imaging", 1)); err == nil {
		t.Fatal("namespaced role without capability was accepted")
	}

	if _, err := Parse(strings.Replace(source, "    role: instrument", "    role: instrument\n  - ref: uv-1\n    role: instrument", 1)); err == nil {
		t.Fatal("duplicate module use was accepted")
	}
}

func TestStep3DeclaresExecutionSemantics(t *testing.T) {
	source := "```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: Assay\n```\n```asterlab\nkind: step\nid: step-run\nschema: 3\ntitle: Readout\nexecution: executable\nassay: assay-1\npath: cck8/readout\n```\n```asterlab\nkind: step\nid: step-group\nschema: 3\ntitle: Incubation\nexecution: organizational\n```\n"
	if _, err := Parse(source); err != nil {
		t.Fatalf("valid executable and organizational steps rejected: %v", err)
	}
	if _, err := Parse(strings.Replace(source, "execution: organizational", "execution: organizational\npath: cck8/incubation", 1)); err == nil {
		t.Fatal("organizational step accepted an execution path")
	}
	if _, err := Parse(strings.Replace(source, "assay: assay-1\n", "", 1)); err == nil {
		t.Fatal("executable step without an assay was accepted")
	}
	if _, err := Parse(strings.Replace(source, "execution: organizational\n", "", 1)); err == nil {
		t.Fatal("step without an execution mode was accepted")
	}
}

func TestBuiltinModuleConformance(t *testing.T) {
	seen := map[string]bool{}
	kinds := map[string]bool{}
	for _, module := range BuiltinRegistry().Descriptors() {
		key := specKey(module.Kind, module.Schema)
		if seen[key] {
			t.Fatalf("duplicate module %s", key)
		}
		seen[key] = true
		kinds[module.Kind] = true
		if module.SDK == "" || module.Category == "" || module.Renderers == nil || module.Lifecycle == nil || module.Provenance == nil {
			t.Fatalf("%s lacks SDK contract metadata", key)
		}
		for _, port := range append(append([]Port{}, module.Inputs...), module.Outputs...) {
			if port.Type == "reference" || port.ItemType == "reference" {
				if port.Target == "" {
					t.Fatalf("%s port %s lacks reference target", key, port.Name)
				}
			}
			if port.Origin == "derived" && port.Calculator == "" {
				t.Fatalf("%s derived output %s lacks calculator", key, port.Name)
			}
		}
	}
	for _, kind := range []string{"plate", "dilution", "instrument_profile", "step"} {
		if !kinds[kind] {
			t.Fatalf("missing reference module %s", kind)
		}
	}
	if !kinds["step"] || !seen[specKey("step", 2)] || !seen[specKey("step", 3)] {
		t.Fatal("step@2 binding and step@3 execution modules are not registered")
	}
}
