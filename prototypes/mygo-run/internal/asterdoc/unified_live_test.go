package asterdoc

import (
	"testing"

	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
)

func TestEvalCalculatorRowsWithPlateAndParamRefs(t *testing.T) {
	src := "# Test Doc\n\n" +
		"```asterlab\nkind: group_table\nid: gt-1\nschema: 1\nrows:\n  - id: g1\n    name: 处理组\n    role: treatment\n    independent_n: 3\n```\n\n" +
		"```asterlab\nkind: plate\nid: plate-1\nschema: 1\nformat: 12\ngroups:\n  g1: [A1, A2, A3, A4]\nexcluded_wells: [A4]\n```\n\n" +
		"```asterlab\nkind: param_table\nid: pt-1\nschema: 1\nrows:\n  - key: vol_per_well\n    label: 每孔体积\n    value: 100\n    unit: uL\n```\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	calcData := map[string]any{
		"kind":   "calculator",
		"id":     "calc-1",
		"schema": 1,
		"mode":   "formula",
		"rows": []any{
			map[string]any{"key": "n_wells", "label": "孔数", "value": "@plate.wells", "unit": "孔"},
			map[string]any{"key": "excess", "label": "余量", "value": 0.1, "unit": ""},
			map[string]any{"key": "total", "label": "总配液", "formula": "n_wells * @param.vol_per_well * (1 + excess)", "unit": "uL"},
		},
	}
	res := EvalCalculatorWithActuals(doc, calcData, nil)
	if res.Err != nil {
		t.Fatalf("EvalCalculatorWithActuals failed: %v", res.Err)
	}
	if len(res.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(res.Rows))
	}
	if res.Rows[0].Amount != 3 {
		t.Fatalf("expected n_wells=3 (excluded A4 skipped), got %v", res.Rows[0].Amount)
	}
	if res.Rows[2].Amount != 330 {
		t.Fatalf("expected total=330, got %v", res.Rows[2].Amount)
	}
}

func TestUnifiedLiveRenderSmokeEditAndReadModes(t *testing.T) {
	src := "---\ntitle: Live Smoke\ndate: 2026-11-20\n---\n\n" +
		"```asterlab\nkind: group_table\nid: gt-1\nschema: 1\nrows:\n  - id: g1\n    name: 处理组\n    role: treatment\n    independent_n: 3\n```\n\n" +
		"```asterlab\nkind: step\nid: step-1\nschema: 1\ntitle: 接种与加药\nblocks:\n  - kind: prose\n    id: p1\n    schema: 1\n    body: 每孔 100 uL\n  - kind: checklist\n    id: chk1\n    schema: 1\n    items:\n      - id: c1\n        label: 消毒超净台\n        done: true\n  - kind: timer\n    id: tm1\n    schema: 1\n    mode: countdown\n    status: idle\n    duration:\n      value: 15\n      unit: min\n  - kind: param_table\n    id: pt1\n    schema: 1\n    rows:\n      - key: temp\n        label: 温度\n        value: 37\n        unit: Cel\n  - kind: calculator\n    id: cal1\n    schema: 1\n    mode: formula\n    rows:\n      - key: v\n        label: 体积\n        value: 100\n        unit: uL\n      - key: tot\n        label: 2倍\n        formula: v * 2\n        unit: uL\n  - kind: data_table\n    id: dt1\n    schema: 1\n    columns:\n      - key: well\n        label: 孔\n        value_kind: text\n      - key: od\n        label: OD\n        value_kind: number\n    rows:\n      - well: A1\n        od: 1.25\n  - kind: figure\n    id: fig1\n    schema: 1\n    svg: \"<svg viewBox='0 0 100 60'></svg>\"\n    caption: 示意\n  - kind: plate\n    id: pl1\n    schema: 1\n    format: 12\n    groups:\n      g1: [A1, A2]\n    layers:\n      浓度: {A1: 10uM, A2: 1uM}\n```\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, mode := range []Mode{ModeDesign, ModeProtocol} {
		view := &View{
			Doc:         doc,
			Mode:        mode,
			UnifiedLive: true,
			OnChange:    func(string) {},
			OnInsertStepBlock: func(string, int, map[string]any) {
			},
			OnUpdateStepBlock: func(string, string, map[string]any) {
			},
			OnRemoveStepBlock: func(string, string) {
			},
		}
		_ = ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			view.Render(c)
		}, 960, 720)
	}
}
