package main

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

func TestAsterDesignPageRendersThreeTabWorkbench(t *testing.T) {
	raw, err := os.ReadFile("internal/asterdoc/testdata/inhibition-zone.asterml")
	if err != nil {
		t.Fatal(err)
	}
	a := &app{designDoc: &runs.AsterDocSnapshot{
		DesignID: "design-1", Revision: 3, Source: string(raw),
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		asterDesignPage(c, a)
	}, 1280, 820)
	tt.Frame()
	texts := strings.Join(tt.Texts(), " ")
	for _, want := range []string{"步骤工作台", "实验全局设定", "AsterML 源码", "步骤大纲", "创建 Run", "rev 3"} {
		if !strings.Contains(texts, want) {
			t.Fatalf("design workbench missing %q in %q", want, texts)
		}
	}
	if err := tt.Click("实验全局设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("实验全局设定") || !tt.HasText("四象限给药分组") {
		t.Fatalf("global setup tab missing group_table: %q", tt.Texts())
	}
	if err := tt.Click("AsterML 源码"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("应用并保存修订") || !tt.HasText("方案格式正常") {
		t.Fatalf("source tab missing editor toolbar: %q", tt.Texts())
	}
}

func TestWorkflowTabRendersDocStepsAndBlockComposer(t *testing.T) {
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	a := &app{designDoc: &runs.AsterDocSnapshot{
		DesignID: "design-1", Revision: 2, Source: string(raw),
	}, designTab: 0}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		asterDesignPage(c, a)
	}, 1280, 820)
	tt.Frame()
	if !tt.HasText("seed") || !tt.HasText("read") || !tt.HasText("01") || !tt.HasText("02") {
		t.Fatalf("step outline rail missing steps: %q", tt.Texts())
	}
	if err := tt.Click("read"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.designSelected != "step-read" {
		t.Fatalf("selected = %q, want step-read", a.designSelected)
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

func TestStepComposerOpensWithAsterMLV2Modules(t *testing.T) {
	source := "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: SDK\n```\n```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: Assay\n```\n```asterlab\nkind: step\nid: step-uv\nschema: 1\ntitle: UV cure\nassay: assay-1\npath: root/uv\nblocks:\n  - kind: prose\n    id: blk-p1\n    schema: 1\n    title: 操作注意\n    body: Irradiate 30s\n```\n"
	a := &app{designDoc: &runs.AsterDocSnapshot{DesignID: "design-1", Revision: 1, Source: source}, designTab: 0}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		asterDesignPage(c, a)
	}, 1280, 820)
	tt.Frame()
	if err := tt.Click("UV cure"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("操作注意") || !tt.HasText("向本步骤插入通用模块") {
		t.Fatalf("step block composer missing blocks/palette: %q", tt.Texts())
	}
}
