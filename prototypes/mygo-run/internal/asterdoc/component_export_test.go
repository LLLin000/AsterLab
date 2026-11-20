package asterdoc

import (
	"strings"
	"testing"
)

func TestRenderPlateSVGAndMarkdownExport(t *testing.T) {
	src := `---
title: CCK-8 Export Check
---

` + "```asterlab\nkind: group_table\nid: g1\nschema: 1\ntitle: 分组对照表\nrows:\n  - id: ctrl\n    name: 对照组\n    role: control\n    condition: DMSO\n    independent_n: 3\n  - id: treat\n    name: 给药组\n    role: treatment\n    condition: 10 uM\n    independent_n: 3\n```" + `

` + "```asterlab\nkind: plate\nid: p1\nschema: 1\ntitle: 96 孔药敏板\nformat: 96\ngroups:\n  ctrl: [A1, A2]\n  treat: [B1, B2]\nexcluded_wells: [B2]\n```" + `
`
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	v := &View{Doc: doc, Mode: ModeDesign, UnifiedLive: true}

	plateData := doc.Modules[1].Data
	plateSVG := v.RenderModuleSVG("plate", plateData, false)
	if !strings.Contains(plateSVG, "<svg") || !strings.Contains(plateSVG, "96 孔药敏板") || !strings.Contains(plateSVG, "<circle") {
		t.Fatalf("expected valid SVG with title & circles, got: %s", plateSVG)
	}
	plateMD := v.FormatModuleAsMarkdown("plate", plateData)
	if !strings.Contains(plateMD, "| **A** |") || !strings.Contains(plateMD, "对照组") {
		t.Fatalf("expected markdown plate grid with A row and group name, got:\n%s", plateMD)
	}

	tableData := doc.Modules[0].Data
	tableSVG := v.RenderModuleSVG("group_table", tableData, true)
	if !strings.Contains(tableSVG, "<svg") || !strings.Contains(tableSVG, "分组对照表") || !strings.Contains(tableSVG, "DMSO") {
		t.Fatalf("expected valid SVG table card, got: %s", tableSVG)
	}
	tableMD := v.FormatModuleAsMarkdown("group_table", tableData)
	if !strings.Contains(tableMD, "| 组别 | 角色 |") || !strings.Contains(tableMD, "n=3") {
		t.Fatalf("expected markdown group_table, got:\n%s", tableMD)
	}
}
