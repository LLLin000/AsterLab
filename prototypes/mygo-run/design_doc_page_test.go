package main

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

func TestAsterDesignPageRendersFrozenDocumentSurface(t *testing.T) {
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	a := &app{designDoc: &runs.AsterDocSnapshot{
		DesignID: "design-1", Revision: 3, Source: string(raw),
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		asterDesignPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if !tt.HasText("AsterDoc 设计") || !tt.HasText("创建 Run") ||
		!tt.HasText("流程") || !tt.HasText("rev 3") {
		t.Fatalf("design surface missing: %q", tt.Texts())
	}
	if err := tt.Click("文档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("96 孔板") {
		t.Fatalf("document tab missing: %q", tt.Texts())
	}
}

func TestWorkflowTabRendersDocStepsAndInspector(t *testing.T) {
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ReplaceAll(string(raw), "path: root/seed", `path: ""`)
	source = strings.ReplaceAll(source, "path: root/read", `path: ""`)
	a := &app{designDoc: &runs.AsterDocSnapshot{
		DesignID: "design-1", Revision: 2, Source: source,
	}, designTab: 0}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		asterDesignPage(c, a)
	}, 1200, 760)
	tt.Frame()
	texts := strings.Join(tt.Texts(), " ")
	if !tt.HasText("seed") || !tt.HasText("read") || !strings.Contains(texts, "读取 OD450") ||
		!tt.HasText("上移") || !tt.HasText("下移") || !tt.HasText("缩进") ||
		!tt.HasText("提升") || !tt.HasText("删除") || !tt.HasText("需要先设置步骤路径") {
		t.Fatalf("step list missing row actions or disabled path reason: %q", tt.Texts())
	}
	if err := tt.Click("seed"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.designSelected != "step-seed" || !tt.HasText("步骤属性") ||
		!tt.HasText("操作摘要") || !tt.HasText("检测") || !tt.HasText("时间点") {
		t.Fatalf("properties drawer missing: %q", tt.Texts())
	}
}

func TestPreflightDialogShowsCoreIssues(t *testing.T) {
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	a := &app{
		designDoc: &runs.AsterDocSnapshot{DesignID: "design-1", Revision: 4, Source: string(raw)},
		readiness: &runs.AsterDocAssessment{
			DesignID: "design-1", Revision: 4, State: "incomplete",
			Errors:   []string{"还没有可执行步骤"},
			Warnings: []string{"检测缺少 protocol"},
		},
		preflightOpen: true,
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		asterDesignPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if !tt.HasText("创建运行前检查") || !tt.HasText("还没有可执行步骤") ||
		!tt.HasText("检测缺少 protocol") || !tt.HasText("返回修改") {
		t.Fatalf("preflight dialog missing: %q", tt.Texts())
	}
}

func TestStepInspectorShowsAsterDocProperties(t *testing.T) {
	source := "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: SDK\n```\n```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: Assay\n```\n```asterlab\nkind: dilution\nid: dilution-01\nschema: 1\nstock: {value: 20, unit: mg/mL}\ntarget: {value: 2, unit: mg/mL}\nfinal_volume: {value: 5, unit: mL}\n```\n```asterlab\nkind: instrument_profile\nid: uv-1\nschema: 1\nclass: uv_curing\ncapabilities:\n  - name: 365 nm\nsource: built_in\n```\n```asterlab\nkind: step\nid: step-uv\nschema: 2\ntitle: UV cure\nassay: assay-1\npath: root/uv\n```\n"
	a := &app{designDoc: &runs.AsterDocSnapshot{DesignID: "design-1", Revision: 1, Source: source}, designTab: 0}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		asterDesignPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("UV cure"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("步骤属性") || !tt.HasText("标题") ||
		!tt.HasText("检测") || !tt.HasText("时间点") || !tt.HasText("保存属性") {
		t.Fatalf("properties drawer missing: %q", tt.Texts())
	}
}
