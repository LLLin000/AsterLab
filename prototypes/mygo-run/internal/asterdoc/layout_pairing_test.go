package asterdoc

import (
	"strings"
	"testing"
)

func TestCanPairVisualSegments(t *testing.T) {
	src := "---\ntitle: Pairing Unit Test\n---\n\n" +
		"```asterlab\nkind: timer\nid: tm-1\nschema: 1\ntitle: Timer 1\nmode: countdown\nstatus: idle\nduration: {value: 20, unit: min}\n```\n\n" +
		"```asterlab\nkind: timer\nid: tm-2\nschema: 1\ntitle: Timer 2\nmode: countdown\nstatus: idle\nduration: {value: 15, unit: min}\n```\n\n" +
		"```asterlab\n# @layout: full\nkind: timer\nid: tm-full\nschema: 1\ntitle: Timer Full\nmode: countdown\nstatus: idle\nduration: {value: 10, unit: min}\n```\n\n" +
		"```asterlab\nkind: calculator\nid: cal-1\nschema: 1\nmode: formula\nrows:\n  - {key: a, label: A, value: 10, unit: uL}\n  - {key: b, label: B, formula: a * 2, unit: uL}\n```\n\n" +
		"```asterlab\nkind: calculator\nid: cal-2\nschema: 1\nmode: formula\nrows:\n  - {key: c, label: C, value: 5, unit: uL}\n  - {key: d, label: D, formula: c * 3, unit: uL}\n```\n\n" +
		"```asterlab\nkind: calculator\nid: cal-sd\nschema: 1\nmode: serial_dilution\ndilution_factor: 2\nsteps_count: 6\nstock: {value: 100, unit: ug/mL}\nvolume_per_unit: {value: 100, unit: uL}\n```\n\n" +
		"```asterlab\nkind: group_table\nid: gt-3\nschema: 1\nrows:\n  - {id: g1, name: Ctrl, role: control}\n  - {id: g2, name: Low, role: treatment}\n  - {id: g3, name: High, role: treatment}\n```\n\n" +
		"```asterlab\nkind: plate\nid: pl-12\nschema: 1\nformat: 12\ngroups:\n  g1: [A1, A2]\n  g2: [B1, B2]\n```\n\n" +
		"```asterlab\nkind: plate\nid: pl-96\nschema: 1\nformat: 96\ngroups:\n  g1: [A1, A2]\n```\n\n" +
		"```asterlab\nkind: group_table\nid: gt-9\nschema: 1\nrows:\n  - {id: ga1, name: G1, role: control}\n  - {id: ga2, name: G2, role: treatment}\n  - {id: ga3, name: G3, role: treatment}\n  - {id: ga4, name: G4, role: treatment}\n  - {id: ga5, name: G5, role: treatment}\n  - {id: ga6, name: G6, role: treatment}\n  - {id: ga7, name: G7, role: treatment}\n  - {id: ga8, name: G8, role: treatment}\n  - {id: ga9, name: G9, role: treatment}\n```\n\n" +
		"```asterlab\nkind: figure\nid: fig-1\nschema: 1\nsvg: \"<svg viewBox='0 0 120 80'><circle cx='60' cy='40' r='20'/></svg>\"\n```\n\n" +
		"```asterlab\nkind: figure\nid: fig-2\nschema: 1\nsvg: \"<svg viewBox='0 0 120 80'><rect x='10' y='10' width='80' height='60'/></svg>\"\n```\n"

	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	byID := map[string]visualFlowSeg{}
	for _, m := range doc.Modules {
		mc := m
		byID[m.ID] = visualFlowSeg{seg: DocSegment{Kind: SegmentModule, Module: &mc}}
	}

	if !canPairVisualSegments(doc, byID["tm-1"], byID["tm-2"], 820) {
		t.Fatal("expected timer+timer to pair at 820px")
	}
	if canPairVisualSegments(doc, byID["tm-1"], byID["tm-2"], 520) {
		t.Fatal("expected timer+timer NOT to pair below 640px")
	}
	if canPairVisualSegments(doc, byID["tm-full"], byID["tm-2"], 820) {
		t.Fatal("expected # @layout: full timer NOT to pair")
	}
	if !canPairVisualSegments(doc, byID["cal-1"], byID["cal-2"], 820) {
		t.Fatal("expected calc+calc to pair at 820px")
	}
	if canPairVisualSegments(doc, byID["cal-1"], byID["cal-sd"], 820) {
		t.Fatal("expected serial_dilution calculator NOT to pair")
	}
	if !canPairVisualSegments(doc, byID["gt-3"], byID["pl-12"], 820) {
		t.Fatal("expected 3-row group_table + 12-well plate to pair at 820px")
	}
	if !canPairVisualSegments(doc, byID["pl-12"], byID["gt-3"], 820) {
		t.Fatal("expected 12-well plate + 3-row group_table to pair symmetrically")
	}
	if canPairVisualSegments(doc, byID["gt-3"], byID["pl-96"], 820) {
		t.Fatal("expected 96-well plate NOT to pair")
	}
	if canPairVisualSegments(doc, byID["gt-9"], byID["pl-12"], 820) {
		t.Fatal("expected >8 row group_table NOT to pair")
	}
	if !canPairVisualSegments(doc, byID["fig-1"], byID["fig-2"], 820) {
		t.Fatal("expected figure+figure to pair at 820px")
	}
	if canPairVisualSegments(doc, byID["tm-1"], byID["cal-1"], 820) {
		t.Fatal("expected cross-kind timer+calculator NOT to pair")
	}
}

func TestToggleFullWidthAndPreservationInUpdateModule(t *testing.T) {
	src := "---\ntitle: Layout Toggle\n---\n\n" +
		"```asterlab\nkind: timer\nid: tm-a\nschema: 1\ntitle: First\nmode: countdown\nstatus: idle\nduration: {value: 10, unit: min}\n```\n"

	withFull, err := ToggleTopLevelModuleFullWidthInSource(src, "tm-a", true)
	if err != nil {
		t.Fatalf("Toggle full=true failed: %v", err)
	}
	if !strings.Contains(withFull, layoutFullDirectiveLine) {
		t.Fatalf("expected directive in source, got:\n%s", withFull)
	}
	doc, err := Parse(withFull)
	if err != nil {
		t.Fatalf("Parse with directive failed: %v", err)
	}
	if !hasLayoutFullDirective(doc.Modules[0].Body) {
		t.Fatal("expected hasLayoutFullDirective true")
	}
	updated, err := doc.UpdateModule("tm-a", map[string]any{"title": "Renamed"})
	if err != nil {
		t.Fatalf("UpdateModule failed: %v", err)
	}
	if !strings.Contains(updated, layoutFullDirectiveLine) {
		t.Fatalf("expected UpdateModule to preserve # @layout: full directive, got:\n%s", updated)
	}
	cleared, err := ToggleTopLevelModuleFullWidthInSource(updated, "tm-a", false)
	if err != nil {
		t.Fatalf("Toggle full=false failed: %v", err)
	}
	if strings.Contains(cleared, layoutFullDirectiveLine) {
		t.Fatalf("expected directive removed, got:\n%s", cleared)
	}
}
