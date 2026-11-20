package main

import (
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
)

func TestProtocolNeverFallsBackToLiveDesignForSelectedRun(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{RunID: "run-1", DesignID: "design-1", DesignRevision: 3}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		protocolPage(c, a)
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("已选择 Run，但它的冻结设计缺失。") {
		t.Fatal("missing frozen design did not render its error state")
	}
	if !tt.HasText("不会改用实时设计") {
		t.Fatal("error state does not state that the live design is refused")
	}
}

func TestProtocolFrozenDocOffersUnifiedRunRecordPDFExport(t *testing.T) {
	source := "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: HUVEC 成管\n```\n" +
		"```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: 成管检测\nprotocol_version_id: pv-1\n```\n" +
		"```asterlab\nkind: step\nid: step-2\nschema: 1\ntitle: 405nm 固化\nassay: assay-1\npath: root/cure\nblocks:\n  - kind: prose\n    id: p-1\n    schema: 1\n    title: 说明\n    body: 60s 固化\n```\n"
	a := &app{
		snap: runs.RunSnapshot{
			RunID:          "run-proto-1",
			Title:          "HUVEC 成管",
			DesignID:       "design-1",
			DesignRevision: 3,
			Status:         "completed",
			AsterDoc:       &runs.AsterDocRef{DesignID: "design-1", Revision: 5, SourceHash: "h1"},
		},
		asterDoc: &asterdoc.FrozenAsterDoc{DesignID: "design-1", Revision: 5, Source: source},
		protocols: []runs.ProtocolSummary{
			{ProtocolVersionID: "pv-1", Title: "HUVEC SOP", Version: 1},
		},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		protocolPage(c, a)
	}, 1080, 760)
	tt.Frame()
	for _, want := range []string{"关联方案快照 · 只读", "导出实验记录 PDF", "405nm 固化"} {
		if !tt.HasText(want) {
			t.Fatalf("protocol page missing %q: %q", want, tt.Texts())
		}
	}
	for _, forbidden := range []string{"终态归档", "导出归档包", "导出归档 PDF", "方案历史版本"} {
		if tt.HasText(forbidden) {
			t.Fatalf("legacy archive text %q should not appear on protocol page: %q", forbidden, tt.Texts())
		}
	}
}
