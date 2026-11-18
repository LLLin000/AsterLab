package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/runworkspace"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

func renderVisualPNG(t *testing.T, dir, name string, w, h int, dark bool, draw func(c *ui.Context)) {
	renderVisualPNGSteps(t, dir, name, w, h, dark, draw, nil)
}

func renderVisualPNGSteps(t *testing.T, dir, name string, w, h int, dark bool, draw func(c *ui.Context), afterFirstFrame func(tt *ui.Tester)) {
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
	if afterFirstFrame != nil {
		afterFirstFrame(tt)
		tt.Advance(400 * time.Millisecond)
		tt.Frame()
	}
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

func writeSyntheticPetriDishPNG(t *testing.T, path string) {
	t.Helper()
	const size = 240
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := color.RGBA{26, 30, 36, 255}
	agar := color.RGBA{226, 212, 168, 255}
	lawn := color.RGBA{204, 188, 138, 255}
	halo := color.RGBA{176, 164, 132, 255}
	disc := color.RGBA{248, 249, 250, 255}
	cross := color.RGBA{120, 128, 138, 255}

	cx, cy := float64(size)/2, float64(size)/2
	type qdisc struct{ x, y, rHalo, rDisc float64 }
	discs := []qdisc{
		{74, 74, 9, 8},   // Q1 PBS (no inhibition zone)
		{166, 74, 22, 8}, // Q2 HAMA
		{74, 166, 34, 8}, // Q3 Vancomycin
		{166, 166, 26, 8}, // Q4 Lysozyme
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			r := math.Hypot(dx, dy)
			c := bg
			if r <= 102 {
				c = agar
				if r <= 96 {
					c = lawn
				}
				if math.Abs(dx) < 1 || math.Abs(dy) < 1 {
					c = cross
				}
				for _, d := range discs {
					dr := math.Hypot(float64(x)-d.x, float64(y)-d.y)
					if dr <= d.rDisc {
						c = disc
						break
					} else if dr <= d.rHalo {
						c = halo
					}
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
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

	// 6. Widescreen 2560x1440 & Compact 960x640 Multi-Resolution Gate
	renderVisualPNG(t, outDir, "design_huvec_2560_light.png", 2560, 1440, false, func(c *ui.Context) {
		asterDesignPage(c, makeDesignApp(huvecSrc, "step-01-prep", 0))
	})
	{
		vmCompact := runworkspace.BuildViewModel(izSnap, izFrozen)
		stCompact := runworkspace.NewState(vmCompact, runworkspace.Callbacks{})
		stCompact.Selected = "t-02"
		renderVisualPNG(t, outDir, "run_iz_compact_960_light.png", 960, 640, false, func(c *ui.Context) {
			runworkspace.View(c, stCompact)
		})
	}

	// 7. Figure@1 Lightbox 150% Zoom Inspection
	zoomApp := makeDesignApp(izSrc, "step-02-quadrants", 0)
	renderVisualPNGSteps(t, outDir, "design_iz_figure_zoom_light.png", 1360, 880, false, func(c *ui.Context) {
		asterDesignPage(c, zoomApp)
	}, func(tt *ui.Tester) {
		if err := tt.Click("放大查看"); err != nil {
			t.Fatalf("click zoom button: %v", err)
		}
		tt.Frame()
		if err := tt.Click("150%"); err != nil {
			t.Fatalf("click 150%% zoom button: %v", err)
		}
		tt.Frame()
		if !tt.HasText("关闭") {
			t.Fatalf("figure lightbox dialog did not open: %q", tt.Texts())
		}
	})

	// 8. Run ModeRun: Persisted Table Readouts + Saved Badge (Light + Dark) & Petri Dish Thumbnail
	dishDir := t.TempDir()
	dishPath := filepath.Join(dishDir, "dish_q4_16h.png")
	writeSyntheticPetriDishPNG(t, dishPath)

	tableEvJSON, _ := json.Marshal(map[string]any{
		"result_cells": []map[string]any{{
			"kind": "record",
			"readout": map[string]any{
				"schema_id":      "inhibition_zone_mm",
				"schema_version": 1,
				"value": map[string]any{
					"kind":    "table",
					"columns": []string{"sample", "d1", "d2", "mean_d"},
					"rows":    [][]any{{"Q1-PBS", 6.0, 6.0, 6.0}, {"Q2-HAMA", 11.4, 11.8, 11.6}, {"Q3-Van", 21.2, 20.8, 21.0}, {"Q4-Lys", 14.5, 14.1, 14.3}},
				},
			},
		}},
	})
	izRichSnap := izSnap
	izRichSnap.Tasks = append([]runs.RunTask(nil), izSnap.Tasks...)
	izRichSnap.Tasks[3].ProtocolVersionID = "pv-iz"
	izRichSnap.Tasks[3].Status = "in_progress"
	izRichSnap.Events = []runs.RunEvent{
		{ID: "ev-iz-tbl", EventType: "measurement", TaskKey: "t-04", ProtocolStepPath: "iz/04-measure", Timepoint: "16h", DataJSON: string(tableEvJSON)},
	}
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		vmTbl := runworkspace.BuildViewModel(izRichSnap, izFrozen)
		stTbl := runworkspace.NewState(vmTbl, runworkspace.Callbacks{
			ExecuteCommand: func(input runs.ExecuteCommandInput) (runs.ExecuteCommandResult, error) {
				return runs.ExecuteCommandResult{ResultJSON: `{"id":"ev-iz-saved"}`}, nil
			},
		})
		stTbl.Selected = "t-04"
		renderVisualPNGSteps(t, outDir, "run_iz_actuals_and_table_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			runworkspace.View(c, stTbl)
		}, func(tt *ui.Tester) {
			if err := tt.Click("保存本表数据"); err != nil {
				t.Fatalf("submit readout table: %v", err)
			}
			tt.Frame()
			if !tt.HasText("数据已记录") {
				t.Fatalf("expected save confirmation badge in table showcase: %q", tt.Texts())
			}
		})
	}

	{
		vmDish := runworkspace.BuildViewModel(izSnap, izFrozen)
		stDish := runworkspace.NewState(vmDish, runworkspace.Callbacks{
			TaskAssets: func(taskKey string) []runworkspace.LinkedAsset {
				if taskKey != "t-03" {
					return nil
				}
				return []runworkspace.LinkedAsset{{
					AssetID: "asset-dish-q4", RootID: "root-dish", RelativePath: "dish_q4_16h.png",
					MediaType: "image", Available: true, Timepoint: "16h",
				}}
			},
			ResolveAssetPath: func(asset runworkspace.LinkedAsset) string {
				return dishPath
			},
		})
		stDish.Selected = "t-03"
		renderVisualPNG(t, outDir, "run_iz_dish_photo_light.png", 1360, 880, false, func(c *ui.Context) {
			runworkspace.View(c, stDish)
		})
	}

	// 9. HUVEC 12-Well Run ModeRun with Live Actuals Calculator Substitution & Widescreen 2560x1440
	huvecLiveSrc := strings.Replace(huvecSrc, "    mode: dilution\n", "    mode: dilution\n    stock_ref: \"@actual.vegf_stock\"\n", 1)
	huvecActualJSON, _ := json.Marshal(map[string]any{
		"actual_parameters": []map[string]any{{"key": "vegf_stock", "value": 200.0, "unit": "ug/mL"}},
	})
	huvecSnap := runs.RunSnapshot{
		RunID: "run-huvec-visual", Title: "HUVEC 成管实验（GelMA 梯度 + VEGF 诱导）", Revision: 2, Status: "active",
		Tasks: []runs.RunTask{
			{Key: "t-h1", DesignStepID: "step-01-prep", Path: "huvec/01-prep", Title: "01 水凝胶热冷消泡与分装体系计算", AssayID: "assay-tube", Timepoint: "6 h", Status: "in_progress"},
			{Key: "t-h2", DesignStepID: "step-02-coat-uv", Path: "huvec/02-coat-uv", Title: "02 12 孔板铺胶与光固化接种", AssayID: "assay-tube", Timepoint: "6 h", Status: "pending"},
			{Key: "t-h3", DesignStepID: "step-03-imaging", Path: "huvec/03-imaging", Title: "03 6h / 12h / 24h 明场多视野采图与成管定量", AssayID: "assay-tube", Timepoint: "6 h", Status: "pending"},
		},
		Events: []runs.RunEvent{
			{ID: "ev-huvec-act", EventType: "operation", TaskKey: "t-h1", ProtocolStepPath: "huvec/01-prep", DataJSON: string(huvecActualJSON)},
		},
	}
	huvecFrozen := &asterdoc.FrozenAsterDoc{DesignID: "visual-huvec", Revision: 2, Source: huvecLiveSrc}
	for _, spec := range []struct {
		name string
		w, h int
	}{
		{"run_huvec_calc_actuals_light.png", 1360, 880},
		{"run_huvec_2560_light.png", 2560, 1440},
	} {
		vmH := runworkspace.BuildViewModel(huvecSnap, huvecFrozen)
		stH := runworkspace.NewState(vmH, runworkspace.Callbacks{})
		stH.Selected = "t-h1"
		renderVisualPNG(t, outDir, spec.name, spec.w, spec.h, false, func(c *ui.Context) {
			runworkspace.View(c, stH)
		})
	}

	// 10. Notebook Record Board Page with Decoded Core ReadoutValue::Table (Light + Dark)
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		nbSnap, nbAssets := boardTestSnap("run-v2-board-" + suffix)
		nbProj := boardTestProj("")
		tp16 := "16h"
		nbProj.Readouts = []record.ReadoutRecord{{
			EventID:   "ev-iz-ro-1",
			StepPath:  "iz/04-measure",
			Timepoint: &tp16,
			Readout: json.RawMessage(`{
				"schema_id":"inhibition_zone_mm",
				"schema_version":1,
				"value":{
					"kind":"table",
					"columns":["sample","d1","d2","mean_d"],
					"rows":[["Q1-PBS",6.0,6.0,6.0],["Q2-HAMA",11.4,11.8,11.6],["Q3-Van",21.2,20.8,21.0],["Q4-Lys",14.5,14.1,14.3]]
				},
				"source":{"kind":"raw","derived_from":[]}
			}`),
		}}
		nbSvc := &sealStubSvc{snap: nbSnap, proj: nbProj}
		nbApp := &app{snap: nbSnap, runAssets: nbAssets, runSvc: nbSvc}
		renderVisualPNGSteps(t, outDir, "notebook_record_board_table_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			notebookPage(c, nbApp)
		}, func(tt *ui.Tester) {
			if err := tt.Click("图板"); err != nil {
				t.Fatalf("click notebook board tab: %v", err)
			}
			tt.Frame()
			if !tt.HasText("结构化表格与分类读数") || !tt.HasText("Q2-HAMA") {
				t.Fatalf("notebook board missing decoded table section: %q", tt.Texts())
			}
		})
	}
}
