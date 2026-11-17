package asterdoc

import (
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
		{"testdata/rt-qpcr.asterml", 0, 1, 3, false},
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
