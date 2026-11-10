package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

func TestNotebookPageLoadsLatestMarkdown(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{
		RunID: "run-1", Title: "CCK-8",
		Events: []runs.RunEvent{
			{ID: "older", Stage: "notebook", Content: "# 旧记录", RecordedAt: "2026-11-10T09:00:00Z"},
			{ID: "latest", Stage: "notebook", Content: "# 今日观察\n\n- 细胞贴壁良好", RecordedAt: "2026-11-10T10:00:00Z"},
		},
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if !tt.HasText("Notebook") || !tt.HasText("实时") || !tt.HasText("编辑") || !tt.HasText("阅读") || !tt.HasText("时间线") {
		t.Fatalf("notebook page is missing its toolbar or modes: %q", tt.Texts())
	}
	if a.noteDoc == nil || a.noteDoc.Text().Markdown() != "# 今日观察\n\n- 细胞贴壁良好" {
		t.Fatalf("live document did not load the latest revision: %q", a.noteDoc.Text().Markdown())
	}
	if err := tt.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.noteMode != 1 || a.noteDraft != "# 今日观察\n\n- 细胞贴壁良好" {
		t.Fatalf("source mode did not carry the latest revision: mode=%d draft=%q", a.noteMode, a.noteDraft)
	}
	if err := tt.Click("阅读"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("今日观察") {
		t.Fatalf("reading mode did not render the document: %q", tt.Texts())
	}
	if a.noteDraft != "# 今日观察\n\n- 细胞贴壁良好" || a.noteDirty {
		t.Fatalf("loaded draft = %q dirty=%v", a.noteDraft, a.noteDirty)
	}
}

func TestNotebookTimelineReadsAuthoritativeEvents(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{
		RunID: "run-1", Title: "CCK-8",
		Events: []runs.RunEvent{
			{ID: "check-1", EventType: "operation", Content: "核对执行检查项", TaskKey: "task-1", RecordedAt: "2026-11-10T10:00:00Z", Checks: []runs.CheckAck{{CheckID: "plate", Status: "passed"}}},
			{ID: "batch-1", EventType: "measurement", Content: "Structured readout batch", BatchID: "batch-1", TaskKey: "task-1", RecordedAt: "2026-11-10T10:01:00Z", ResultCells: []runs.ResultCell{{Row: 0, Column: 0, SampleID: "Control-1", Value: ptrFloat(0.82)}}},
		},
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("时间线"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"运行时间线 · 只读", "batch-1", "check-1", "打开原始孔板"} {
		if !tt.HasText(want) {
			t.Fatalf("timeline missing %q: %q", want, tt.Texts())
		}
	}
	if err := tt.Click("打开原始孔板"); err != nil {
		t.Fatal(err)
	}
	if a.batchView != "batch-1" {
		t.Fatalf("batch drilldown = %q", a.batchView)
	}
}

func ptrFloat(v float64) *float64 { return &v }

func TestNotebookAsterDocModeRendersPlateModule(t *testing.T) {
	source := "# D3\n\n```asterlab\nkind: plate96\nid: plate-d3\nschema: 1\nassay: cck8\ngroups:\n  control: [A1, A2]\n  material: [B1]\n```\n"
	a := &app{snap: runs.RunSnapshot{
		RunID: "run-1", Title: "CCK-8",
		Events: []runs.RunEvent{{ID: "note", Stage: "notebook", Content: source, RecordedAt: "2026-11-10T10:00:00Z"}},
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("文档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.noteMode != 3 {
		t.Fatalf("document mode not selected: %d", a.noteMode)
	}
	if !tt.HasText("96 孔板") || !tt.HasText("plate-d3") {
		t.Fatalf("asterlab module did not render: %q", tt.Texts())
	}
}

func TestNotebookDocumentModeCanInsertPlateModule(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{RunID: "run-1", Title: "CCK-8"}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		notebookPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("文档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("插入孔板模块"); err != nil {
		t.Fatalf("insert plate action: %v", err)
	}
	tt.Frame()
	tt.Frame()
	if !strings.Contains(a.noteDraft, "kind: plate96") {
		t.Fatalf("plate module was not inserted: %q", a.noteDraft)
	}
	if !tt.HasText("96 孔板") {
		t.Fatalf("inserted module did not render: %q", tt.Texts())
	}
}
