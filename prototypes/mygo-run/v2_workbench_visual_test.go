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
	"asterlab-mygo-prototype/internal/tools"
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
	qpcrSrc := readFixture("internal/asterdoc/testdata/rt-qpcr.asterml")

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
		nbApp := &app{
			snap:      nbSnap,
			runAssets: nbAssets,
			runSvc:    nbSvc,
			asterDoc:  &asterdoc.FrozenAsterDoc{DesignID: "visual-huvec", Revision: 3, Source: huvecSrc},
		}
		renderVisualPNGSteps(t, outDir, "notebook_record_board_table_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			notebookPage(c, nbApp)
		}, func(tt *ui.Tester) {
			if err := tt.Click("图板"); err != nil {
				t.Fatalf("click notebook board tab: %v", err)
			}
			tt.Frame()
			if !tt.HasText("结构化表格与分类读数") || !tt.HasText("Q2-HAMA") || !tt.HasText("实验方案与实测模块总览") {
				t.Fatalf("notebook board missing expected sections: %q", tt.Texts())
			}
		})
	}

	// 11. Multi-Dimensional 96-Well qPCR Plate (Samples + Target Genes + Triplicates + Std/NTC) & Zoom Overlay
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		qApp := makeDesignApp(qpcrSrc, "step-02-plate-setup", 0)
		renderVisualPNGSteps(t, outDir, "design_qpcr_plate_multilayer_"+suffix+".png", 1360, 980, dark, func(c *ui.Context) {
			asterDesignPage(c, qApp)
		}, func(tt *ui.Tester) {
			if err := tt.Click("孔 C1"); err != nil {
				t.Fatalf("click inline C1 well: %v", err)
			}
			tt.Frame()
			if err := tt.Click("靶点基因"); err != nil {
				t.Fatalf("click factor color mode: %v", err)
			}
			tt.Frame()
		})
	}
	{
		qOverlayApp := makeDesignApp(qpcrSrc, "step-02-plate-setup", 0)
		renderVisualPNGSteps(t, outDir, "design_qpcr_plate_overlay_light.png", 1360, 880, false, func(c *ui.Context) {
			asterDesignPage(c, qOverlayApp)
		}, func(tt *ui.Tester) {
			if err := tt.Click("放大孔板"); err != nil {
				t.Fatalf("click expand plate: %v", err)
			}
			tt.Frame()
			if err := tt.Click("样本"); err != nil {
				t.Fatalf("click sample color mode in overlay: %v", err)
			}
			tt.Frame()
			if err := tt.Click("孔 B2"); err != nil {
				t.Fatalf("click B2 well in overlay: %v", err)
			}
			tt.Frame()
		})
	}

	// 12. Redesigned Notebook ELN Split (Reading with embedded asterlab widget + Companion Hydrated Modules)
	izNoteMarkdown := `# 抑菌圈实验阶段研判

## 关键现象
- Q3 万古霉素组形成直径 **21.0 mm** 清晰抑菌透明圈，阳性对照符合预期。
- Q2 HAMA 水凝胶浸提片平均抑菌圈 **11.6 mm**，呈现温和抑菌活性 @matrix(16h)。

` + "```asterlab\nkind: param_table\nid: note-pt-summary\nschema: 1\ntitle: 实验条件\nrows:\n  - {key: temp, label: 倒置培养温度, value: 37, unit: Cel, record_actual: true}\n  - {key: duration, label: 孵育时长, value: 16, unit: h, record_actual: true}\n  - {key: disc_mm, label: 滤纸片直径, value: 6, unit: mm, record_actual: false}\n```"
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		splitSnap := izRichSnap
		splitSnap.Events = append([]runs.RunEvent(nil), izRichSnap.Events...)
		splitSnap.Events = append(splitSnap.Events, runs.RunEvent{
			ID: "ev-nb-split", EventType: "observation", Stage: "notebook",
			Content: izNoteMarkdown, RecordedAt: "2026-11-18T11:30:00Z",
		})
		splitApp := &app{
			snap:     splitSnap,
			asterDoc: &asterdoc.FrozenAsterDoc{DesignID: "visual-design-1", Revision: 4, Source: izSrc},
		}
		renderVisualPNGSteps(t, outDir, "notebook_eln_split_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			notebookPage(c, splitApp)
		}, func(tt *ui.Tester) {
			if err := tt.Click("阅读"); err != nil {
				t.Fatalf("click reading tab: %v", err)
			}
			tt.Frame()
			if err := tt.Click("实验模块"); err != nil {
				t.Fatalf("click companion modules tab: %v", err)
			}
			tt.Frame()
			if !tt.HasText("实验条件") {
				t.Fatalf("embedded asterlab block did not render in reading mode: %q", tt.Texts())
			}
		})
	}

	// 13. Modernized Tools & Pantry Workbench
	{
		nearDays := uint32(180)
		toolApp := &app{
			tools: tools.New(tools.Callbacks{
				ListPantry: func() (runs.PantryLedger, error) {
					return runs.PantryLedger{
						Items: []runs.PantryItem{
							{ID: "p-1", Name: "SYBR Green MasterMix (2×)", Location: "-20°C 酶盒 A2", Status: "near_expiry", PurchasedOn: "2026-06-01", OpenedOn: "2026-10-20", ShelfLifeDays: &nearDays, ExpiresOn: "2026-11-28", Revision: 3},
							{ID: "p-2", Name: "GelMA 光固化水凝胶前驱液", Location: "4°C 避光冷藏层", Status: "opened", PurchasedOn: "2026-10-01", OpenedOn: "2026-11-05", ShelfLifeDays: &nearDays, ExpiresOn: "2027-04-01", Revision: 2},
							{ID: "p-3", Name: "万古霉素标准品滤纸片 (30 ug)", Location: "干燥器 B1", Status: "unopened", PurchasedOn: "2026-09-15", ShelfLifeDays: &nearDays, ExpiresOn: "2027-09-15", Revision: 1},
						},
					}, nil
				},
			}),
		}
		renderVisualPNG(t, outDir, "tools_workbench_light.png", 1360, 880, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})
		renderVisualPNG(t, outDir, "tools_pantry_workbench_v2.png", 1360, 880, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})

		// 14. 孔板速排（先选孔板 + 悬浮单框选孔标注 + SVG/PNG 导出 + 两列孔板历史）
		plateStudio := toolApp.tools
		plateStudio.Active = tools.ToolPlate
		plateStudio.SelectPlatePreset("plate-96")
		plateStudio.SelectPlateWells([]string{"A1", "A2", "A3"})
		_ = plateStudio.AnnotateSelectedWells("对照组")
		plateStudio.SelectPlateWells([]string{"A4", "A5", "A6"})
		_ = plateStudio.AnnotateSelectedWells("VEGF-50")
		plateStudio.SelectPlateWells([]string{"H12"})
		_ = plateStudio.AnnotateSelectedWells("NTC")
		hist96, _ := plateStudio.SaveCurrentPlateToHistory()

		plateStudio.SelectPlatePreset("dish-4")
		plateStudio.SelectPlateWells([]string{"A1", "A2"})
		_ = plateStudio.AnnotateSelectedWells("万古霉素-30ug")
		plateStudio.SelectPlateWells([]string{"B1"})
		_ = plateStudio.AnnotateSelectedWells("空白对照")
		plateStudio.SaveCurrentPlateToHistory()

		plateStudio.SelectPlatePreset("rack-8")
		plateStudio.SelectPlateWells([]string{"A1", "A2", "A3", "A4"})
		_ = plateStudio.AnnotateSelectedWells("浓度:100/2 uM")
		plateStudio.SaveCurrentPlateToHistory()

		plateStudio.SelectPlatePreset("plate-12")
		plateStudio.SelectPlateWells([]string{"A1", "A2", "A3"})
		_ = plateStudio.AnnotateSelectedWells("对照组")
		plateStudio.SelectPlateWells([]string{"A4", "B1", "B2"})
		_ = plateStudio.AnnotateSelectedWells("VEGF-50ng/mL")
		plateStudio.SelectPlateWells([]string{"B4"})
		_ = plateStudio.AnnotateSelectedWells("NTC")
		plateStudio.SelectPlateWells([]string{"C1", "C2", "C3"})
		plateStudio.Status = ""

		renderVisualPNG(t, outDir, "comp_15b_tools_plate_legacy.png", 1360, 1400, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})
		renderVisualPNG(t, outDir, "tools_plate_studio_v2.png", 1360, 1400, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})

		// Export genuine 2x Retina PNG of the active 12-well plate studio design
		plateStudio.SelectPlateWells(nil)
		if pngBytes, _, err := plateStudio.ExportActivePlate("png", false); err != nil {
			t.Fatalf("ExportActivePlate png failed: %v", err)
		} else if err := os.WriteFile(filepath.Join(outDir, "tools_plate_exported_png_v2.png"), pngBytes, 0o644); err != nil {
			t.Fatalf("write tools_plate_exported_png_v2.png: %v", err)
		}

		// Load 96-well history card back into studio & open zoomed overlay with row B selected
		if hist96.ID != "" && !plateStudio.LoadHistoryEntry(hist96.ID) {
			t.Fatalf("LoadHistoryEntry failed for %s", hist96.ID)
		}
		plateStudio.Status = ""
		renderVisualPNGSteps(t, outDir, "tools_plate_studio_overlay_v2.png", 1360, 920, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		}, func(tt *ui.Tester) {
			if err := tt.Click("放大孔板"); err != nil {
				t.Fatalf("tools studio click expand plate: %v", err)
			}
			tt.Frame()
			if err := tt.Click("选择第 B 行"); err != nil {
				t.Fatalf("tools studio overlay click row B: %v", err)
			}
			tt.Frame()
		})

		// 15. Regenerate updated plate@1 component specimens (Chinese GroupLabels legends)
		geomSrc := "```asterlab\nkind: group\nid: g-pbs\nschema: 1\nname: PBS对照\nrole: control\n```\n" +
			"```asterlab\nkind: group\nid: g-hama\nschema: 1\nname: HAMA水凝胶\nrole: treatment\n```\n" +
			"```asterlab\nkind: group\nid: g-van\nschema: 1\nname: 万古霉素\nrole: comparator\n```\n" +
			"```asterlab\nkind: plate\nid: pl-dish4\nschema: 1\ntitle: 4区圆形培养皿（56px大孔 + 双皿壁十字）\nformat: 4\ntopology: dish\nrows: 2\ncolumns: 2\ngroups:\n  g-pbs: [A1]\n  g-hama: [A2, B1]\n  g-van: [B2]\nsamples: {A1: PBS, A2: HAMA, B1: Lys, B2: Van}\n```\n" +
			"```asterlab\nkind: plate\nid: pl-6\nschema: 1\ntitle: 6 孔细胞培养板（48px 大孔）\nformat: 6\ngroups:\n  g-pbs: [A1, A2, A3]\n  g-hama: [B1, B2, B3]\nsamples: {A1: Ctrl-1, A2: Ctrl-2, A3: Ctrl-3, B1: Drug-1, B2: Drug-2, B3: Drug-3}\n```\n" +
			"```asterlab\nkind: plate\nid: pl-12\nschema: 1\ntitle: 12 孔细胞铺板（42px 孔）\nformat: 12\ngroups:\n  g-pbs: [A1, A2]\n  g-hama: [B1, B2]\n  g-van: [C1, C2]\n```\n" +
			"```asterlab\nkind: plate\nid: pl-rack\nschema: 1\ntitle: 8 联管 / 电泳泳道（40×58px 竖向胶囊）\nformat: 8\ntopology: rack_1d\nrows: 1\ncolumns: 8\ngroups:\n  g-pbs: [A1, A2]\n  g-hama: [A3, A4, A5]\n  g-van: [A6, A7, A8]\nsamples: {A1: Ladder, A2: Ctrl, A3: Sample, A4: Sample, A5: Sample, A6: KO1, A7: KO2, A8: NTC}\n```\n"
		geomDoc, err := asterdoc.Parse(geomSrc)
		if err != nil {
			t.Fatalf("parse geomSrc: %v", err)
		}
		geomView := &asterdoc.View{
			Doc: geomDoc, Mode: asterdoc.ModeDesign,
			Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
		}
		renderVisualPNG(t, outDir, "comp_12_plate_geometries_v2.png", 1240, 920, false, func(c *ui.Context) {
			tok := theme.GetTokens(c.Theme())
			ui.Column(c).Fill().Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
				var plates []*asterdoc.Module
				for i := range geomDoc.Modules {
					if geomDoc.Modules[i].Kind == "plate" {
						plates = append(plates, &geomDoc.Modules[i])
					}
				}
				for i := 0; i < len(plates); i += 2 {
					left := plates[i]
					right := plates[i+1]
					ui.Row(c).FillWidth().Gap(float32(tok.Spacing12)).AlignItems(ui.Stretch).Children(func() {
						ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
							geomView.RenderModule(c, "plate", 1, left.Data)
						})
						ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
							geomView.RenderModule(c, "plate", 1, right.Data)
						})
					})
				}
			})
		})

		qpcrDoc, err := asterdoc.Parse(qpcrSrc)
		if err != nil {
			t.Fatalf("parse qpcrSrc: %v", err)
		}
		var qpcrPlateData map[string]any
		for _, m := range qpcrDoc.Modules {
			if m.Kind == "plate" {
				qpcrPlateData = m.Data
				break
			}
			if m.Kind == "step" && m.ID == "step-02-plate-setup" {
				if seq, ok := m.Data["blocks"].([]any); ok {
					for _, raw := range seq {
						if bm, _ := raw.(map[string]any); bm != nil && bm["kind"] == "plate" {
							qpcrPlateData = bm
							break
						}
					}
				}
			}
		}
		if qpcrPlateData != nil {
			qCompView := &asterdoc.View{
				Doc: qpcrDoc, Mode: asterdoc.ModeDesign,
				Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
			}
			renderVisualPNGSteps(t, outDir, "comp_13_plate_96_v2.png", 1120, 740, false, func(c *ui.Context) {
				tok := theme.GetTokens(c.Theme())
				ui.Box(c).Fill().Padding(float32(tok.Spacing16)).Children(func() {
					qCompView.RenderModule(c, "plate", 1, qpcrPlateData)
				})
				qCompView.RenderPlateOverlay(c)
			}, func(tt *ui.Tester) {
				if err := tt.Click("孔 C1"); err != nil {
					t.Fatalf("comp13 click C1: %v", err)
				}
				tt.Frame()
			})

			qOvView := &asterdoc.View{
				Doc: qpcrDoc, Mode: asterdoc.ModeDesign,
				Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
			}
			renderVisualPNGSteps(t, outDir, "comp_13b_plate_overlay_bar_v2.png", 1200, 840, false, func(c *ui.Context) {
				tok := theme.GetTokens(c.Theme())
				ui.Box(c).Fill().Padding(float32(tok.Spacing16)).Children(func() {
					qOvView.RenderModule(c, "plate", 1, qpcrPlateData)
				})
				qOvView.RenderPlateOverlay(c)
			}, func(tt *ui.Tester) {
				if err := tt.Click("放大孔板"); err != nil {
					t.Fatalf("comp13b expand plate: %v", err)
				}
				tt.Frame()
				if err := tt.Click("选择第 B 行"); err != nil {
					t.Fatalf("comp13b select row B: %v", err)
				}
				tt.Frame()
			})
		}
	}
}
