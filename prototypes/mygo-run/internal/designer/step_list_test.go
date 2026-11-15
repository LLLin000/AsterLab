package designer

import (
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/workflow"

	"github.com/egoist/mygo/ui"
)

// TestStepCardShowsRecordedParameters pins the card contract: the step's own
// structured parameters are the summary, the budget bar sums the durations
// they record, and a step with nothing recorded never shows a placeholder.
func TestStepCardShowsRecordedParameters(t *testing.T) {
	d := New("design-1", "HUVEC 成管", 2, &state.FakeDesignService{})
	d.Editor.Doc.Nodes = []workflow.Node{
		{
			ID: "n1", Kind: workflow.KindFlask, Title: "明胶成胶", Order: "01",
			Params: []workflow.NodeParam{
				{Key: "温度", Val: "80 °C"},
				{Key: "孵育时间", Val: "5m"},
				{Key: "占手", Val: "45m"},
			},
		},
	}
	d.SelectedNode, d.Editor.SelectedNode = "n1", "n1"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		ListView(c, d)
	}, 1360, 880)
	tt.Frame()

	for _, want := range []string{"温度 80 °C", "孵育时间 5m", "占手 45m"} {
		if !tt.HasText(want) {
			t.Fatalf("card summary missing %q: %q", want, tt.Texts())
		}
	}
	// The budget bar totals the recorded durations (5m total, 45m hands-on).
	if !tt.HasText("总跨度") || !tt.HasText("5m") || !tt.HasText("占手") {
		t.Fatalf("budget bar missing totals: %q", tt.Texts())
	}
	if tt.HasText("未填写参数") {
		t.Fatalf("placeholder leaked into the card: %q", tt.Texts())
	}
}

// TestStepCardWithoutParametersShowsNoPlaceholder proves the other half of the
// rule: nothing recorded means nothing rendered, not "未填写参数".
func TestStepCardWithoutParametersShowsNoPlaceholder(t *testing.T) {
	d := New("design-1", "空设计", 1, &state.FakeDesignService{})
	d.Editor.Doc.Nodes = []workflow.Node{{ID: "n1", Kind: workflow.KindFlask, Title: "待补充步骤"}}
	d.SelectedNode, d.Editor.SelectedNode = "n1", "n1"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		ListView(c, d)
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("待补充步骤") {
		t.Fatalf("card title missing: %q", tt.Texts())
	}
	if tt.HasText("未填写参数") {
		t.Fatalf("placeholder rendered for an unrecorded step: %q", tt.Texts())
	}
}

// TestSectionSwitchPreservesSelectionAndDraft pins UX-P5: 流程 / 12 孔板 /
// 配液单 is view state, so switching sections never drops the selected step or
// the unsaved draft.
func TestSectionSwitchPreservesSelectionAndDraft(t *testing.T) {
	d := New("design-1", "设计", 1, &state.FakeDesignService{})
	first := d.Editor.AddNode(workflow.KindFlask, "步骤 A", workflow.Position{})
	d.Editor.AddNode(workflow.KindPipette, "步骤 B", workflow.Position{X: 240})
	d.Editor.SelectNode(first)
	d.AssayIDDraft = "cck8"
	d.StepTimepointsDraft = "24 h"
	d.Touch()
	if d.SelectedNode != first || !d.Dirty {
		t.Fatalf("setup failed: selected=%q dirty=%v", d.SelectedNode, d.Dirty)
	}

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		ListView(c, d)
	}, 1200, 800)
	tt.Frame()

	for _, step := range []struct {
		label string
		want  Section
	}{
		{"12 孔板", SectionPlate},
		{"配液单", SectionDilution},
		{"流程", SectionFlow},
	} {
		if err := tt.Click(step.label); err != nil {
			t.Fatalf("click %q: %v", step.label, err)
		}
		tt.Frame()
		if d.Section != step.want {
			t.Fatalf("section = %v after %q, want %v", d.Section, step.label, step.want)
		}
		if d.SelectedNode != first {
			t.Fatalf("selection lost after %q: %q", step.label, d.SelectedNode)
		}
		if d.AssayIDDraft != "cck8" || d.StepTimepointsDraft != "24 h" {
			t.Fatalf("draft lost after %q: assay=%q timepoints=%q",
				step.label, d.AssayIDDraft, d.StepTimepointsDraft)
		}
		if !d.Dirty {
			t.Fatalf("dirty flag cleared by section switch %q", step.label)
		}
	}
}

// TestTechnicalBindingsFoldedByDefault pins UX-P2: no protocol path, reference
// id or block-version id is visible in the default card, and one click on the
// card's 技术属性 disclosure reveals them.
func TestTechnicalBindingsFoldedByDefault(t *testing.T) {
	d := New("design-1", "设计", 1, &state.FakeDesignService{})
	node := d.Editor.AddNode(workflow.KindFlask, "铺板", workflow.Position{})
	reference := "8b9e5e16-67ae-49f5-a942-c929833e12ba"
	d.Editor.Doc.Nodes[0].ComponentRefs = []string{reference}
	d.Instances[node] = []blocks.Instance{{
		ID: "i1", DefinitionID: "note1", DefinitionVersion: 3, Type: blocks.TypeNote,
		RawProps: []byte(`{"text":"hello"}`),
	}}
	d.StepTimepoints[node] = []string{"24 h"}
	d.StepTaskPaths[stepTaskKey(node, "24 h")] = "root/seed"

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		ListView(c, d)
	}, 1360, 880)
	tt.Frame()

	for _, leak := range []string{reference, "note1@3", "root/seed"} {
		if tt.HasText(leak) {
			t.Fatalf("technical binding %q visible in the default view: %q", leak, tt.Texts())
		}
	}
	if !tt.HasText("技术属性") {
		t.Fatalf("properties disclosure missing: %q", tt.Texts())
	}

	if err := tt.Click("技术属性"); err != nil {
		t.Fatalf("open properties: %v", err)
	}
	tt.Frame()
	tt.Frame()
	for _, want := range []string{reference, "note1@3", "root/seed"} {
		if !tt.HasText(want) {
			t.Fatalf("binding %q not revealed after opening 技术属性: %q", want, tt.Texts())
		}
	}
}

// TestStepCardSummaryText checks the summary derivation directly, including
// the assay/timepoint fallback and the duration label parsing.
func TestStepCardSummaryText(t *testing.T) {
	d := New("design-1", "设计", 1, nil)
	node := d.Editor.AddNode(workflow.KindFlask, "孵育", workflow.Position{})
	d.StepAssays[node] = "cck8"
	d.StepTimepoints[node] = []string{"D1", "D3"}

	cards := d.stepCardViews()
	if len(cards) != 1 {
		t.Fatalf("cards = %d", len(cards))
	}
	got := cards[0].summary()
	if !strings.Contains(got, "检测 cck8") || !strings.Contains(got, "时间点 D1, D3") {
		t.Fatalf("fallback summary = %q", got)
	}
	if cards[0].durationLabel() != "" {
		t.Fatalf("duration invented with no recorded cost: %q", cards[0].durationLabel())
	}
	budget := d.designBudget()
	if budget.Steps != 1 || budget.HasTotal || budget.HasHandsOn {
		t.Fatalf("budget invented durations: %+v", budget)
	}
}
