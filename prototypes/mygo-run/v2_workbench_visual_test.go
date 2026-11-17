package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/runworkspace"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

func renderVisualPNG(t *testing.T, dir, name string, w, h int, dark bool, draw func(c *ui.Context)) {
	t.Helper()
	var tt *ui.Tester
	tt = ui.NewTester(func(c *ui.Context) {
		if dark {
			c.SetTheme(theme.DarkTheme())
		} else {
			c.SetTheme(theme.LightTheme())
		}
		tok := theme.GetTokens(c.Theme())
		ui.Box(c).Fill().Background(tok.Canvas).Children(func() {
			draw(c)
		})
	}, w, h)
	tt.SetDark(dark)
	tt.Frame()
	img := tt.Image()
	if img == nil {
		t.Fatalf("%s: nil screenshot image", name)
	}
	// Verify non-uniform raster (caught blank/broken renders)
	b := img.Bounds()
	first := img.RGBAAt(b.Min.X+8, b.Min.Y+8)
	diffCount := 0
	for y := b.Min.Y; y < b.Max.Y; y += 17 {
		for x := b.Min.X; x < b.Max.X; x += 17 {
			px := img.RGBAAt(x, y)
			if px != first {
				diffCount++
			}
		}
	}
	if diffCount < 20 {
		t.Fatalf("%s: screenshot appears nearly blank (diff samples=%d)", name, diffCount)
	}
	outPath := filepath.Join(dir, name)
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestV2WorkbenchVisualGate(t *testing.T) {
	outDir := os.Getenv("ASTERLAB_VISUAL_DIR")
	if outDir == "" {
		outDir = t.TempDir()
	} else if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	readFixture := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	izSrc := readFixture("internal/asterdoc/testdata/inhibition-zone.asterml")
	cfuSrc := readFixture("internal/asterdoc/testdata/cfu.asterml")
	huvecSrc := readFixture("internal/asterdoc/testdata/huvec-tube.asterml")

	makeDesignApp := func(src, stepID string, tab int) *app {
		return &app{
			designDoc: &runs.AsterDocSnapshot{
				DesignID: "visual-design-1",
				Revision: 4,
				Source:   src,
			},
			readiness: &runs.AsterDocAssessment{
				State:    "executable",
				Errors:   nil,
				Warnings: nil,
			},
			designTab:      tab,
			designSelected: stepID,
		}
	}

	// 1. Design Workbench Tab 0 across 3 flagship experiments (Light + Dark)
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		renderVisualPNG(t, outDir, "design_iz_tab0_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			asterDesignPage(c, makeDesignApp(izSrc, "step-02-quadrants", 0))
		})
		renderVisualPNG(t, outDir, "design_cfu_tab0_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			asterDesignPage(c, makeDesignApp(cfuSrc, "step-03-count", 0))
		})
		renderVisualPNG(t, outDir, "design_huvec_tab0_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			asterDesignPage(c, makeDesignApp(huvecSrc, "step-02-coat-uv", 0))
		})
	}

	// 2. Design Workbench Tab 1 (Global Setup) & Tab 2 (AsterML Source + Live Preview)
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		renderVisualPNG(t, outDir, "design_iz_tab1_global_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			asterDesignPage(c, makeDesignApp(izSrc, "step-02-quadrants", 1))
		})
		renderVisualPNG(t, outDir, "design_iz_tab2_source_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			asterDesignPage(c, makeDesignApp(izSrc, "step-02-quadrants", 2))
		})
	}

	// 3. Compact viewport check (960x640)
	renderVisualPNG(t, outDir, "design_iz_compact_960_light.png", 960, 640, false, func(c *ui.Context) {
		asterDesignPage(c, makeDesignApp(izSrc, "step-02-quadrants", 0))
	})

	// 4. New Design Template Picker Dialog
	renderVisualPNG(t, outDir, "design_new_dialog_light.png", 1080, 680, false, func(c *ui.Context) {
		a := &app{newDesignOpen: true}
		a.renderNewDesignDialog(c)
	})

	// 5. Run Workspace ModeRun same-source execution (Light + Dark)
	izSnap := runs.RunSnapshot{
		RunID:    "run-iz-visual",
		Title:    "HAMA / 万古霉素 / 溶菌酶 四象限抑菌圈实验",
		Revision: 3,
		Status:   "active",
		Tasks: []runs.RunTask{
			{Key: "t-01", DesignStepID: "step-01-lawn", Path: "iz/01-lawn", Title: "01 菌悬液校正与 LB 琼脂均匀涂布", AssayID: "assay-iz", Timepoint: "16h", Status: "completed"},
			{Key: "t-02", DesignStepID: "step-02-quadrants", Path: "iz/02-quadrants", Title: "02 四象限贴片布局与处理分组", AssayID: "assay-iz", Timepoint: "16h", Status: "in_progress"},
			{Key: "t-03", DesignStepID: "step-03-incubate", Path: "iz/03-incubate", Title: "03 37 Cel 倒置孵育与定时提醒", AssayID: "assay-iz", Timepoint: "16h", Status: "pending"},
			{Key: "t-04", DesignStepID: "step-04-measure", Path: "iz/04-measure", Title: "04 双径十字抑菌圈测量与培养皿采图", AssayID: "assay-iz", Timepoint: "16h", Status: "pending"},
		},
	}
	izFrozen := &asterdoc.FrozenAsterDoc{DesignID: "visual-design-1", Revision: 4, Source: izSrc}
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		vm := runworkspace.BuildViewModel(izSnap, izFrozen)
		st := runworkspace.NewState(vm, runworkspace.Callbacks{})
		st.Selected = "t-02"
		renderVisualPNG(t, outDir, "run_iz_mode_run_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			runworkspace.View(c, st)
		})
	}
}
