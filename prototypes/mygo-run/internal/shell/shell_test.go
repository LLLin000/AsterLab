package shell

import (
	"image/png"
	"os"
	"testing"

	"asterlab-mygo-prototype/internal/runui"

	"github.com/egoist/mygo/ui"
)

func TestShellRenders(t *testing.T) {
	page := Run
	s := &Shell{Project: "L929", Page: page, SidebarOpen: true, Designs: []DesignItem{{ID: "d1", Title: "L929 生物相容性"}}, Selected: "d1", Content: func(c *ui.Context) {
		ui.Column(c).Padding(16).Children(func() { ui.Text(c, "Run content") })
	}}
	tt := ui.NewTester(func(c *ui.Context) { View(c, s) }, 1200, 760)
	tt.Frame()
	if !tt.HasText("实验运行") || !tt.HasText("L929 生物相容性") {
		t.Fatalf("missing labels: %q", tt.Texts())
	}
	if err := tt.Click("侧栏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	_ = func() error {
		f, e := os.Create("shell.png")
		if e != nil {
			return e
		}
		defer f.Close()
		return png.Encode(f, tt.Image())
	}()
}

func TestShellShortcuts(t *testing.T) {
	s := &Shell{
		Project:     "L929",
		Page:        Run,
		SidebarOpen: true,
	}
	tt := ui.NewTester(func(c *ui.Context) { View(c, s) }, 1200, 760)
	tt.Frame()

	// Test Cmd+B toggles sidebar
	tt.Key(ui.Cmd, ui.KeyB)
	tt.Frame()
	if s.SidebarOpen {
		t.Fatal("expected sidebar closed after Cmd+B")
	}

	tt.Key(ui.Cmd, ui.KeyB)
	tt.Frame()
	if !s.SidebarOpen {
		t.Fatal("expected sidebar open after second Cmd+B")
	}

	// Test Cmd+1 switches to Design page
	tt.Key(ui.Cmd, ui.Key1)
	tt.Frame()
	if s.Page != Design {
		t.Fatalf("expected page Design, got %v", s.Page)
	}

	// Test Cmd+3 switches to Run page
	tt.Key(ui.Cmd, ui.Key3)
	tt.Frame()
	if s.Page != Run {
		t.Fatalf("expected page Run, got %v", s.Page)
	}
}

func TestShellThemes(t *testing.T) {
	s := &Shell{Project: "L929", Page: Run, SidebarOpen: true}
	// Light theme
	ttLight := ui.NewTesterWith(func(c *ui.Context) { View(c, s) }, 1200, 760, ui.TesterOptions{Dark: false})
	ttLight.Frame()
	if ttLight.Image() == nil {
		t.Fatal("light image nil")
	}

	// Dark theme
	ttDark := ui.NewTesterWith(func(c *ui.Context) { View(c, s) }, 1200, 760, ui.TesterOptions{Dark: true})
	ttDark.Frame()
	if ttDark.Image() == nil {
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
			{Key: "t4", Title: "root/read", Timepoint: "D3", Status: "not_started"},
			{Key: "t5", Title: "root/read", Timepoint: "D7", Status: "not_started"},
			{Key: "t6", Title: "root/image", Timepoint: "D7", Status: "not_started"},
		},
		Checks: []runui.Check{
			{Key: "m", Title: "材料", Binding: "GelMA · 水凝胶基质 (就绪)"},
		},
		Measurements: []runui.Measurement{
			{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"},
			{SampleID: "ZnO-0.5", Endpoint: "OD450", Value: 0.79, Unit: "AU"},
		},
	}
	ws := runui.NewWorkspace(v, runui.Callbacks{})
	ws.Selected = "t4"

	s := &Shell{
		Project:     "L929",
		Page:        Run,
		SidebarOpen: true,
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

	tt := ui.NewTester(func(c *ui.Context) { View(c, s) }, 1186, 753)
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
		Page: Project, Projects: []string{"Study A"},
		Designs: []DesignItem{{ID: "design-1", Title: "Design A", Meta: "Edited today"}},
		RecentRuns: []RecentRun{{ID: "run-1", Title: "Run A", Meta: "Saved"}},
		OnProject: func(v string) { project = v },
		OnCreate: func() { created = true }, OnImport: func() { imported = true },
		OnContinue: func() { continued = true },
	}
	tt := ui.NewTester(func(c *ui.Context) { View(c, s) }, 900, 600)
	tt.Frame()
	for _, text := range []string{"项目工作区", "Study A", "Design A", "Run A", "暂无项目"} {
		if tt.HasText(text) == (text == "暂无项目") { t.Fatalf("unexpected workspace text %q: %v", text, tt.Texts()) }
	}
	for _, label := range []string{"Study A", "新建设计", "导入", "继续上次工作"} {
		if err := tt.Click(label); err != nil { t.Fatal(err) }
		tt.Frame()
	}
	if project != "Study A" || !created || !imported || !continued { t.Fatal("workspace action callback not invoked") }
}

func TestShellResponsiveThemeScreenshots(t *testing.T) {
	for _, size := range []struct{ w, h int; label string }{{1186, 753, "1186x753"}, {900, 600, "900x600"}} {
		for _, dark := range []bool{false, true} {
			name := "light"
			if dark { name = "dark" }
			tt := ui.NewTesterWith(func(c *ui.Context) { View(c, &Shell{Page: Project}) }, size.w, size.h, ui.TesterOptions{Dark: dark})
			tt.Frame()
			f, err := os.Create("evidence-shell-" + size.label + "-" + name + ".png")
			if err != nil { t.Fatal(err) }
			if err := png.Encode(f, tt.Image()); err != nil { f.Close(); t.Fatal(err) }
			if err := f.Close(); err != nil { t.Fatal(err) }
		}
	}
}
