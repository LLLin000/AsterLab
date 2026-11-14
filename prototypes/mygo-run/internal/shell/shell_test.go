package shell

import (
	"image/png"
	"os"
	"testing"

	"asterlab-mygo-prototype/internal/runui"
	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
)

// render draws one region of the shell — the navigation column or the page —
// the way the window builds it.
func render(region func(*ui.Context), w, h int) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		region(c)
	}, w, h)
}

func TestNavigationShowsPagesAndDesigns(t *testing.T) {
	s := &Shell{
		Page:     Run,
		Designs:  []DesignItem{{ID: "d1", Title: "L929 生物相容性", Meta: "rev 2"}},
		Selected: "d1",
	}
	tt := render(s.navigation, 260, 700)
	tt.Frame()
	for _, label := range []string{"项目工作区", "实验设计", "实验运行", "Notebook", "资料与工具", "最近设计", "L929 生物相容性"} {
		if !tt.HasText(label) {
			t.Fatalf("missing %q in %q", label, tt.Texts())
		}
	}
	if err := tt.Click("实验设计"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.Page != Design {
		t.Fatalf("page = %v, want Design", s.Page)
	}
}

func TestPageShortcuts(t *testing.T) {
	s := &Shell{Project: "L929", Page: Run}
	tt := render(s.page, 1200, 760)
	tt.Frame()

	// Cmd+B asks the window to toggle; without a window it must stay harmless.
	tt.Key(ui.Cmd, ui.KeyB)
	tt.Frame()

	tt.Key(ui.Cmd, ui.Key2)
	tt.Frame()
	if s.Page != Design {
		t.Fatalf("page = %v, want Design", s.Page)
	}

	tt.Key(ui.Cmd, ui.Key3)
	tt.Frame()
	if s.Page != Protocol {
		t.Fatalf("page = %v, want Protocol", s.Page)
	}

	tt.Key(ui.Cmd, ui.Key4)
	tt.Frame()
	if s.Page != Run {
		t.Fatalf("page = %v, want Run", s.Page)
	}

	tt.Key(ui.Cmd, ui.Key5)
	tt.Frame()
	if s.Page != Notebook {
		t.Fatalf("page = %v, want Notebook", s.Page)
	}

	tt.Key(ui.Cmd, ui.Key6)
	tt.Frame()
	if s.Page != Tools {
		t.Fatalf("page = %v, want Tools", s.Page)
	}
}

func TestShellThemes(t *testing.T) {
	s := &Shell{Project: "L929", Page: Run}
	for _, dark := range []bool{false, true} {
		tt := ui.NewTesterWith(func(c *ui.Context) { s.page(c) }, 1200, 760, ui.TesterOptions{Dark: dark})
		tt.Frame()
		if tt.Image() == nil {
			t.Fatalf("dark=%v image nil", dark)
		}
	}
}

func TestShellUserDimensions1186x753(t *testing.T) {
	v := runui.RunView{
		Title:    "GelMA/ZnO L929 biocompatibility",
		Status:   "in_progress",
		Revision: 16,
		Tasks: []runui.Task{
			{Key: "t1", Title: "root/seed", Timepoint: "准备", Status: "completed"},
			{Key: "t2", Title: "root/treat", Timepoint: "准备", Status: "completed"},
			{Key: "t3", Title: "root/read", Timepoint: "D1", Status: "completed"},
			{Key: "t4", Title: "root/read", Timepoint: "D3", Status: "not_started",
				Checks: []runui.TaskCheck{{CheckID: "blank", Label: "空白孔已确认", Blocking: true}}},
			{Key: "t5", Title: "root/read", Timepoint: "D7", Status: "not_started"},
			{Key: "t6", Title: "root/image", Timepoint: "D7", Status: "not_started"},
		},
		Measurements: []runui.Measurement{
			{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"},
			{SampleID: "ZnO-0.5", Endpoint: "OD450", Value: 0.79, Unit: "AU"},
		},
	}
	ws := runui.NewWorkspace(v, runui.Callbacks{})
	ws.Selected = "t4"

	s := &Shell{
		Project: "L929",
		Page:    Run,
		Designs: []DesignItem{
			{ID: "d1", Title: "GelMA/ZnO L929 biocompatibility"},
			{ID: "d2", Title: "Timed design"},
			{ID: "d3", Title: "p-AKT Western blot"},
			{ID: "d4", Title: "ELISA design"},
			{ID: "d5", Title: "RT-qPCR 基因表达比较"},
			{ID: "d6", Title: "CCK-8 设计工作区"},
		},
		Selected: "d1",
		Content: func(c *ui.Context) {
			runui.View(c, ws)
		},
	}

	tt := render(s.page, 1186, 753)
	tt.Frame()
	f, err := os.Create("evidence-shell-1186x753.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestProjectWorkspaceDataAndActions(t *testing.T) {
	project, created, imported, continued := "", false, false, false
	s := &Shell{
		Page:     Project,
		Projects: []string{"Study A"},
		Designs:  []DesignItem{{ID: "design-1", Title: "Design A", Meta: "rev 1"}},
		RecentRuns: []RecentRun{
			{ID: "run-1", Title: "Run A", Meta: "已保存"},
		},
		OnProject:  func(v string) { project = v },
		OnCreate:   func() { created = true },
		OnImport:   func() { imported = true },
		OnContinue: func() { continued = true },
	}
	tt := render(s.page, 900, 600)
	tt.Frame()
	if f, err := os.Create("evidence-home-workspace.png"); err == nil {
		png.Encode(f, tt.Image())
		f.Close()
	}
	for _, text := range []string{"项目工作区", "Study A", "Run A", "1 个项目 · 1 个设计 · 1 个运行"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q in %q", text, tt.Texts())
		}
	}
	if tt.HasText("尚无项目") {
		t.Fatalf("empty state shown with a project: %q", tt.Texts())
	}
	for _, label := range []string{"Study A", "新建设计", "导入设计", "继续上次工作"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	if project != "Study A" || !created || !imported || !continued {
		t.Fatalf("workspace actions: project=%q created=%v imported=%v continued=%v", project, created, imported, continued)
	}
}
func TestEmptyProjectWorkspaceOffersNextAction(t *testing.T) {
	for _, tc := range []struct {
		name, project, want string
		projects            []string
	}{
		{name: "no project", want: "尚无项目"},
		{name: "empty project", project: "Study A", projects: []string{"Study A"}, want: "还没有实验设计"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			created := false
			s := &Shell{Page: Project, Project: tc.project, Projects: tc.projects, OnCreate: func() { created = true }}
			tt := render(s.page, 900, 600)
			tt.Frame()
			// UX-P5 / UX-STATE / UX-DONE: each empty state names 未开始 and offers new design.
			for _, text := range []string{"未开始", tc.want, "新建设计"} {
				if !tt.HasText(text) {
					t.Fatalf("empty workspace missing %q: %q", text, tt.Texts())
				}
			}
			if err := tt.Click("新建设计"); err != nil {
				t.Fatal(err)
			}
			if !created {
				t.Fatal("新建设计 did not invoke its action")
			}
		})
	}
}

func TestCoreWriteErrorOffersRecoveryAction(t *testing.T) {
	s := &Shell{Page: Project, Error: "permission denied: asset root is read-only"}
	tt := render(s.page, 900, 600)
	tt.Frame()
	// UX-P5 / UX-STATE / UX-DONE: preserve Core cause and identify permission repair plus retry.
	for _, text := range []string{"未保存", "permission denied: asset root is read-only", "检查目标目录权限", "重试保存"} {
		if !tt.HasText(text) {
			t.Fatalf("failed-write state missing %q: %q", text, tt.Texts())
		}
	}
}

func TestShellResponsiveThemeScreenshots(t *testing.T) {
	for _, size := range []struct {
		w, h  int
		label string
	}{{1186, 753, "1186x753"}, {900, 600, "900x600"}} {
		for _, dark := range []bool{false, true} {
			name := "light"
			if dark {
				name = "dark"
			}
			tt := ui.NewTesterWith(func(c *ui.Context) { (&Shell{Page: Project}).page(c) }, size.w, size.h, ui.TesterOptions{Dark: dark})
			tt.Frame()
			f, err := os.Create("evidence-shell-" + size.label + "-" + name + ".png")
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
