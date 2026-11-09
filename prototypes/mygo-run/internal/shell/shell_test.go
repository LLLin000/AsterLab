package shell

import (
	"image/png"
	"os"
	"testing"

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
