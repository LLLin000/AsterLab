package asterdoc

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

func TestV2ShowcaseFixturesParseAndComputeBudget(t *testing.T) {
	cases := []struct {
		path           string
		minSteps       int
		minGroups      int
		minWells       int
		wantFormulaErr bool
	}{
		{"testdata/inhibition-zone.asterml", 4, 4, 4, false},
		{"testdata/cfu.asterml", 3, 2, 6, false},
		{"testdata/huvec-tube.asterml", 3, 3, 12, false},
		{"testdata/rt-qpcr.asterml", 3, 4, 33, false},
		{"../../blank.asterml", 1, 2, 6, false},
		{"../../starter.asterml", 2, 2, 6, false},
	}
	for _, tc := range cases {
		t.Run(filepath.Base(tc.path), func(t *testing.T) {
			raw, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read %s: %v", tc.path, err)
			}
			doc, err := Parse(string(raw))
			if err != nil {
				t.Fatalf("Parse(%s): %v", tc.path, err)
			}
			b := ComputeBudgetSummary(doc)
			if b.ExecutableSteps < tc.minSteps || b.Groups < tc.minGroups || b.Wells < tc.minWells {
				t.Fatalf("budget %s = %+v", tc.path, b)
			}
		})
	}
}

func TestFigureSVGMetricAndLabelExtraction(t *testing.T) {
	raw, err := os.ReadFile("testdata/inhibition-zone.asterml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	var svgRaw string
	for _, m := range doc.Modules {
		if m.Kind != "step" {
			continue
		}
		blocks, _ := m.Data["blocks"].([]any)
		for _, rb := range blocks {
			bm, _ := rb.(map[string]any)
			if textValue(bm, "kind") == "figure" {
				svgRaw = textValue(bm, "svg")
			}
		}
	}
	if svgRaw == "" {
		t.Fatal("inhibition-zone figure svg missing")
	}
	_, _, w, h, labels := parseSVGMetricsAndLabels(svgRaw)
	if w != 420 || h != 264 {
		t.Fatalf("viewBox = %gx%g, want 420x264", w, h)
	}
	if len(labels) != 4 {
		t.Fatalf("extracted labels = %d (%+v), want 4", len(labels), labels)
	}
}

func TestEvalFormulaAndCalculatorModes(t *testing.T) {
	zone, err := EvalFormula("(d1 + d2) / 2", map[string]float64{"d1": 11.4, "d2": 11.8})
	if err != nil || math.Abs(zone-11.6) > 1e-6 {
		t.Fatalf("zone = %g, err=%v", zone, err)
	}
	cfu, err := EvalFormula("colonies / (dilution * plated_ml)", map[string]float64{
		"colonies": 150, "dilution": 0.0001, "plated_ml": 0.1,
	})
	if err != nil || math.Abs(cfu-1.5e7) > 1 {
		t.Fatalf("cfu = %g, err=%v", cfu, err)
	}
	serial := EvalCalculator(nil, map[string]any{
		"mode":            "serial_dilution",
		"dilution_factor": 10.0,
		"steps_count":     3,
		"volume_per_unit": map[string]any{"value": 1.0, "unit": "mL"},
	})
	if serial.Err != nil || len(serial.Rows) != 3 {
		t.Fatalf("serial = %+v", serial)
	}
}

func TestStepBlockComposerRendersAndInsertsGenericModules(t *testing.T) {
	raw, err := os.ReadFile("testdata/inhibition-zone.asterml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	var step *Module
	for i := range doc.Modules {
		if doc.Modules[i].ID == "step-02-quadrants" {
			step = &doc.Modules[i]
			break
		}
	}
	if step == nil {
		t.Fatal("step-02-quadrants missing")
	}
	var insertedKind string
	v := &View{
		Doc:      doc,
		Registry: BuiltinRegistry(),
		Mode:     ModeDesign,
		Allowed:  map[string]bool{PermDocumentEdit: true},
		OnInsertStepBlock: func(stepID string, index int, blk map[string]any) {
			insertedKind = textValue(blk, "kind")
		},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		ui.Scroll(c).Fill().Children(func() {
			StepBlockComposer(v, c, step)
		})
	}, 1100, 1400)
	tt.Frame()
	joined := strings.Join(tt.Texts(), " ")
	for _, want := range []string{"滤纸片与加样规格", "四象限培养皿布片示意图", "贴片操作核查", "向本步骤插入通用模块", "文本/注意", "计时器"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in composer texts: %q", want, joined)
		}
	}
	if err := tt.Click("文本/注意"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if insertedKind != "prose" {
		t.Fatalf("insertedKind = %q, want prose", insertedKind)
	}
}

func TestMoveStepBlockToIndexAndCalculatorActualsAndPaste(t *testing.T) {
	raw, err := os.ReadFile("testdata/inhibition-zone.asterml")
	if err != nil {
		t.Fatal(err)
	}
	nextSrc, err := MoveStepBlockToIndex(string(raw), "step-02-quadrants", "blk-iz-quadrant-svg", 0)
	if err != nil {
		t.Fatalf("MoveStepBlockToIndex failed: %v", err)
	}
	doc, err := Parse(nextSrc)
	if err != nil {
		t.Fatalf("re-parse after move failed: %v", err)
	}
	for _, m := range doc.Modules {
		if m.ID == "step-02-quadrants" {
			blocks, _ := m.Data["blocks"].([]any)
			first, _ := blocks[0].(map[string]any)
			if textValue(first, "id") != "blk-iz-quadrant-svg" {
				t.Fatalf("first block after reorder = %q, want blk-iz-quadrant-svg", textValue(first, "id"))
			}
		}
	}

	// Upstream actual calculator resolution: formula + dilution + per-sample normalize
	actuals := &CalcActualContext{
		Params:     map[string]float64{"stock_van": 20.0, "dose": 2.5},
		ParamUnits: map[string]string{"stock_van": "mg/mL"},
		Samples: []CalcSampleInput{
			{SampleID: "S1", Value: 250, Unit: "ng/uL", Endpoint: "rna_conc"},
			{SampleID: "S2", Value: 10, Unit: "ng/uL", Endpoint: "rna_conc"},
		},
	}
	dilRes := EvalCalculatorWithActuals(doc, map[string]any{
		"mode":         "dilution",
		"stock_ref":    "@actual.stock_van",
		"stock":        map[string]any{"value": 10.0, "unit": "mg/mL"},
		"target":       map[string]any{"value": 2.0, "unit": "mg/mL"},
		"final_volume": map[string]any{"value": 100.0, "unit": "uL"},
	}, actuals)
	if dilRes.Err != nil || len(dilRes.UsedActuals) != 1 || math.Abs(dilRes.Rows[0].Amount-10.0) > 1e-4 {
		t.Fatalf("dilRes = %+v", dilRes)
	}
	normRes := EvalCalculatorWithActuals(doc, map[string]any{
		"mode":              "sample_normalize",
		"upstream_endpoint": "rna_conc",
		"target_mass":       map[string]any{"value": 500.0, "unit": "ng"},
		"volume_per_unit":   map[string]any{"value": 10.0, "unit": "uL"},
	}, actuals)
	if normRes.Err != nil || len(normRes.Rows) != 2 || math.Abs(normRes.Rows[0].Amount-2.0) > 1e-4 || !strings.Contains(normRes.Rows[1].Note, "超出单孔反应体积上限") {
		t.Fatalf("normRes = %+v", normRes)
	}

	// Multi-row paste with header skip, auto-expansion & typed ReadoutSchema generation
	v := &View{Doc: doc, Mode: ModeRun}
	cols := []map[string]any{
		{"key": "sample", "label": "样本", "value_kind": "text"},
		{"key": "d1", "label": "纵径", "value_kind": "number", "unit": "mm"},
		{"key": "d2", "label": "横径", "value_kind": "number", "unit": "mm"},
		{"key": "mean_mm", "label": "平均直径", "value_kind": "formula", "formula": "(d1 + d2) / 2", "unit": "mm"},
	}
	filled := v.applyClipboardToDataTableAt("dt-test", cols, 0, 0, 0, "样本\t纵径\t横径\nVan-1\t100\t12\nHAMA-1\t15\t16\nBlank-1\t6\t6")
	if filled != 3 {
		t.Fatalf("filled rows = %d, want 3", filled)
	}
	sub, err := v.buildDataTableSubmission("dt-test", "cfu_count", "zone_mm", cols, nil, 3)
	if err != nil {
		t.Fatalf("buildDataTableSubmission error = %v", err)
	}
	if sub["schema_id"] != "cfu_count.table" {
		t.Fatalf("builtin collision schema_id = %v", sub["schema_id"])
	}
	var parsedSchema map[string]any
	if err := json.Unmarshal([]byte(sub["schema_def_json"].(string)), &parsedSchema); err != nil {
		t.Fatal(err)
	}
	rows := sub["rows"].([][]any)
	if len(rows) != 3 || rows[0][0] != "Van-1" || rows[0][3] != 56.0 {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestDataTableMatrixPasteRoundTrip(t *testing.T) {
	src := "```asterlab\nkind: experiment\nid: exp-mtx\nschema: 1\ntitle: 孔板矩阵测试\nobjective: 验证 2D 整板粘贴\n```\n\n```asterlab\nkind: group\nid: grp-ctrl\nschema: 1\nname: 对照\nrole: control\nindependent_n: 3\n```\n\n```asterlab\nkind: plate\nid: pl-6\nschema: 1\nformat: 6\ngroups:\n  grp-ctrl: [A1, A2, A3]\nsamples:\n  A1: Ctrl-1\n  A2: Ctrl-2\n  A3: Ctrl-3\n```\n\n```asterlab\nkind: data_table\nid: dt-mtx\nschema: 1\ntitle: 6孔板 OD 读数\ncontainer_ref: pl-6\nlayout: matrix\ncolumns:\n  - {key: well, label: 孔位, value_kind: text}\n  - {key: sample_id, label: 样本, value_kind: text}\n  - {key: od450, label: OD450, value_kind: number, unit: AU}\n```\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	v := &View{Doc: doc, Mode: ModeRun}
	pl, _ := v.findPlate("pl-6")
	cols := []map[string]any{
		{"key": "well", "label": "孔位", "value_kind": "text"},
		{"key": "sample_id", "label": "样本", "value_kind": "text"},
		{"key": "od450", "label": "OD450", "value_kind": "number", "unit": "AU"},
	}
	// Excel/plate-reader TSV with column headers 1..3 and row headers A..B
	tsv := "<>\t1\t2\t3\nA\t0.412\t0.419\t0.405\nB\t1.102\t1.088\t1.140"
	filled := v.ApplyMatrixClipboardToDataTable("dt-mtx", pl, cols, nil, "od450", tsv)
	if filled != 6 {
		t.Fatalf("filled = %d, want 6", filled)
	}
	sub, err := v.buildDataTableSubmission("dt-mtx", "od450_matrix", "od450", cols, nil, 6)
	if err != nil {
		t.Fatalf("submission failed: %v", err)
	}
	rows := sub["rows"].([][]any)
	if len(rows) != 6 || rows[0][0] != "A1" || rows[0][1] != "Ctrl-1" || rows[0][2] != 0.412 || rows[5][0] != "B3" || rows[5][2] != 1.140 {
		t.Fatalf("unexpected submitted matrix rows: %#v", rows)
	}
}
