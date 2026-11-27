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
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/tools"
	"asterlab-mygo-prototype/internal/uikit/labsticker"
	"asterlab-mygo-prototype/internal/vault"
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

	// 0. Experiment Calendar Home (实验日程 · 每月/每周 + 当日实验安排)
	calendarExperiments := []shell.ExperimentItem{
		{
			ID: "exp-1116", DesignID: "d-cfu-prev", Title: "E.coli CFU 梯度涂板计数",
			Date: "2026-11-16", Stage: shell.StageCompleted, ActiveRunID: "run-cfu-prev",
			Summary: "已完成 · 实测 18 个涂板计数菌落", StartHour: 14.0, DurationMin: 90,
		},
		{
			ID: "exp-1118-a", DesignID: "d-iz-ready", Title: "HAMA / 万古霉素 四象限抑菌圈实验",
			Date: "2026-11-18", Stage: shell.StageReady,
			Summary: "4 组处理 · 4 个可执行步骤 · 约 18 小时", StartHour: 9.0, DurationMin: 120,
		},
		{
			ID: "exp-1118-b", DesignID: "d-huvec-live", Title: "HUVEC 水凝胶成管（GelMA 梯度）",
			Date: "2026-11-18", Stage: shell.StageInProgress, ActiveRunID: "run-huvec-live",
			Summary: "步骤 2/3 执行中 · 等待 12h 采图", StartHour: 13.5, DurationMin: 90,
		},
		{
			ID: "exp-1118-c", DesignID: "d-qpcr-draft", Title: "RT-qPCR 多靶点表达定量（草稿）",
			Date: "2026-11-18", Stage: shell.StageDraft,
			Summary: "已建 2 组 · 待补充数据记录表", GapHint: "待补充测量方案或数据记录表",
		},
		{
			ID: "exp-1120", DesignID: "d-next", Title: "L929 CCK-8 细胞毒性复测",
			Date: "2026-11-20", Stage: shell.StageReady,
			Summary: "3 组对比 · 96 孔板 · 预计 24 小时", StartHour: 10.0, DurationMin: 60,
		},
	}
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		calShell := &shell.Shell{
			Page:             shell.Project,
			TodayDate:        "2026-11-18",
			SelectedDate:     "2026-11-18",
			CalendarCursor:   "2026-11-18",
			CalendarMode:     0,
			Experiments:      calendarExperiments,
			OnCreateForDate:  func(string) {},
			OnOpenExperiment: func(shell.ExperimentItem) {},
			OnQuickStartRun:  func(string) {},
		}
		renderVisualPNGSteps(t, outDir, "home_calendar_month_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			calShell.RenderSplitView(c)
		}, func(tt *ui.Tester) {
			if !dark {
				if err := tt.Click("2026-11-18"); err != nil {
					t.Fatalf("open apple day popover: %v", err)
				}
				tt.Frame()
				if !tt.HasText("11 月 18 日（周三） · 实验安排") {
					t.Fatalf("apple day popover missing agenda: %q", tt.Texts())
				}
			}
		})
	}
	{
		weekShell := &shell.Shell{
			Page:             shell.Project,
			TodayDate:        "2026-11-18",
			SelectedDate:     "2026-11-18",
			CalendarCursor:   "2026-11-18",
			CurrentHour:      10.5,
			CalendarMode:     1,
			Experiments:      calendarExperiments,
			OnCreateForDate:  func(string) {},
			OnOpenExperiment: func(shell.ExperimentItem) {},
			OnQuickStartRun:  func(string) {},
		}
		renderVisualPNG(t, outDir, "home_calendar_week_light.png", 1360, 880, false, func(c *ui.Context) {
			weekShell.RenderSplitView(c)
		})
	}

	// 0b. Obsidian 3-Pane Peer Split (Left Edit + Right Read) + File Tree + Right Drawer Showcase
	{
		tmpVault := filepath.Join(t.TempDir(), "vault")
		if v, err := vault.Open(tmpVault); err == nil {
			_ = v.SeedTemplates(defaultTemplateSeeds())
			projDir, _ := v.CreateFolder("", "抗菌与细胞实验")
			e1, _ := v.CreateExperiment(projDir, "四象限抑菌圈实验", "2026-11-18", izSrc)
			e2, _ := v.CreateExperiment(projDir, "HUVEC 水凝胶成管", "2026-11-18", huvecSrc)
			_, _ = v.CreateExperiment("", "L929 CCK-8 细胞毒性复测", "2026-11-20", cfuSrc)
			ds := vault.NewDocStore(v)

			obsShell := &shell.Shell{
				TodayDate:        "2026-11-18",
				SelectedDate:     "2026-11-18",
				SelectedTreePath: e1.RelPath,
				RightOpen:        true,
				RightTab:         0,
				PantryItems: []shell.QuickPantryItem{
					{ID: "p-1", Name: "SYBR Green 预混液", Location: "-20°C A2", Status: "near_expiry", ExpiresOn: "2026-11-28"},
					{ID: "p-2", Name: "兔抗-VEGFA 一抗", Location: "-80°C C2", Status: "opened", ExpiresOn: "2027-05-12"},
				},
			}
			primaryApp := &app{
				shell:    obsShell,
				vault:    v,
				docStore: ds,
			}
			obsShell.RenderTabContent = func(c *ui.Context, pane *shell.PaneNode, tab shell.WorkspaceTab) {
				primaryApp.renderVaultTabContent(c, pane, tab)
			}
			obsShell.RenderPantryContent = func(c *ui.Context) {
				primaryApp.renderStandalonePantry(c)
			}
			obsShell.RenderOverlays = func(c *ui.Context) {
				primaryApp.renderVaultNewExperimentDialog(c)
			}
			primaryApp.refreshVaultTreeAndCalendar()
			obsShell.EnsureTabs()
			obsShell.OpenFileInPane(obsShell.ActivePaneID, e1.RelPath, e1.Title, shell.ModeEdit, true)
			obsShell.SplitPaneWithTab(obsShell.ActivePaneID, shell.WorkspaceTab{
				Kind: shell.TabFile, Title: e2.Title, FilePath: e2.RelPath, Mode: shell.ModeRead,
			}, "right")

			renderVisualPNG(t, outDir, "workbench_obsidian_split_drawer_light.png", 1440, 900, false, func(c *ui.Context) {
				obsShell.RenderSplitView(c)
			})
		}
	}

	// 0c. Floating Ctrl+K Command Palette Showcase
	{
		palShell := &shell.Shell{
			Page:         shell.Project,
			TodayDate:    "2026-11-18",
			SelectedDate: "2026-11-18",
			Experiments:  calendarExperiments,
			PantryItems: []shell.QuickPantryItem{
				{ID: "p-1", Name: "SYBR Green 预混液 (2×)", Location: "-20°C 酶盒 A2", Status: "near_expiry", ExpiresOn: "2026-11-28"},
				{ID: "p-2", Name: "兔抗-VEGFA 一抗", Location: "-80°C 冻存盒 C2", Status: "opened", ExpiresOn: "2027-05-12"},
			},
			PaletteCommands: []shell.PaletteCommand{
				{ID: "cmd-pdf", Title: "导出当前实验 PDF", Subtitle: "按组件级 A4 版式生成方案与记录打印件"},
				{ID: "cmd-fork", Title: "复刻当前实验为新草稿", Subtitle: "生成独立未封存 .asterml 并写入 repeat_of 溯源"},
			},
			PaletteOpen: true,
		}
		renderVisualPNG(t, outDir, "workbench_command_palette_light.png", 1360, 880, false, func(c *ui.Context) {
			palShell.RenderSplitView(c)
		})
	}

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

	// 2. Design Workbench Tab 1 (Single-Page Sheet Preview) & Tab 2 (AsterML Source + Live Preview)
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		renderVisualPNG(t, outDir, "design_iz_tab1_sheet_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
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

	liveRunCallbacks := func() runworkspace.Callbacks {
		return runworkspace.Callbacks{
			Complete: func(string, []runworkspace.CheckAckView) error { return nil },
			AppendEvent: func(string, runs.ExecutionEventInput) (string, error) { return "", nil },
			ExecuteCommand: func(runs.ExecuteCommandInput) (runs.ExecuteCommandResult, error) {
				return runs.ExecuteCommandResult{ResultJSON: `{"id":"ev-iz-saved"}`}, nil
			},
		}
	}

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
		st := runworkspace.NewState(vm, liveRunCallbacks())
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
		stCompact := runworkspace.NewState(vmCompact, liveRunCallbacks())
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
		if err := tt.Click("放大"); err != nil {
			t.Fatalf("click 放大 zoom button: %v", err)
		}
		tt.Frame()
		if !tt.HasText("150%") {
			t.Fatalf("expected 150%% zoom indicator in lightbox: %q", tt.Texts())
		}
		if _, ok := tt.Find("关闭灯箱"); !ok {
			t.Fatalf("figure lightbox close button not found: %q", tt.Texts())
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
		stTbl := runworkspace.NewState(vmTbl, liveRunCallbacks())
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
		cbDish := liveRunCallbacks()
		cbDish.TaskAssets = func(taskKey string) []runworkspace.LinkedAsset {
			if taskKey != "t-03" {
				return nil
			}
			return []runworkspace.LinkedAsset{{
				AssetID: "asset-dish-q4", RootID: "root-dish", RelativePath: "dish_q4_16h.png",
				MediaType: "image", Available: true, Timepoint: "16h",
			}}
		}
		cbDish.ResolveAssetPath = func(asset runworkspace.LinkedAsset) string {
			return dishPath
		}
		stDish := runworkspace.NewState(vmDish, cbDish)
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
		stH := runworkspace.NewState(vmH, liveRunCallbacks())
		stH.Selected = "t-h1"
		renderVisualPNG(t, outDir, spec.name, spec.w, spec.h, false, func(c *ui.Context) {
			runworkspace.View(c, stH)
		})
	}

	// 11. Multi-Dimensional 96-Well qPCR Plate & Zoom Overlay (No redundant top mode strip)
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
			if err := tt.Click("孔 B2"); err != nil {
				t.Fatalf("click B2 well in overlay: %v", err)
			}
			tt.Frame()
		})
	}

	// 12. Streamlined 3-Mode Notebook ELN Split (阅读 + 编辑 showcase + 右侧实测模块)
	izNoteMarkdown := `# 抑菌圈实验阶段研判

## 关键现象
- Q3 万古霉素组形成直径 **21.0 mm** 清晰抑菌透明圈，阳性对照符合预期。
- Q2 HAMA 水凝胶浸提片平均抑菌圈 **11.6 mm**，呈现温和抑菌活性。

` + "```asterlab\nkind: param_table\nid: note-pt-summary\nschema: 1\ntitle: 实验条件\nrows:\n  - {key: temp, label: 倒置培养温度, value: 37, unit: Cel, record_actual: true}\n  - {key: duration, label: 孵育时长, value: 16, unit: h, record_actual: true}\n  - {key: disc_mm, label: 滤纸片直径, value: 6, unit: mm, record_actual: false}\n```"
	izNoteProj := boardTestProj("")
	tp16 := "16h"
	izNoteProj.Readouts = []record.ReadoutRecord{{
		EventID:   "ev-iz-tbl",
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
			runSvc:   &sealStubSvc{snap: splitSnap, proj: izNoteProj},
			asterDoc: &asterdoc.FrozenAsterDoc{DesignID: "visual-design-1", Revision: 4, Source: izSrc},
		}
		renderVisualPNGSteps(t, outDir, "notebook_eln_split_"+suffix+".png", 1360, 880, dark, func(c *ui.Context) {
			notebookPage(c, splitApp)
		}, func(tt *ui.Tester) {
			if err := tt.Click("阅读"); err != nil {
				t.Fatalf("click reading mode tab: %v", err)
			}
			tt.Frame()
			if !tt.HasText("实测模块") || !tt.HasText("导出 PDF") || !tt.HasText("实验条件") {
				t.Fatalf("3-mode notebook reading view missing controls/module: %q", tt.Texts())
			}
		})
	}
	{
		liveSnap := izRichSnap
		liveSnap.Events = append([]runs.RunEvent(nil), izRichSnap.Events...)
		liveSnap.Events = append(liveSnap.Events, runs.RunEvent{
			ID: "ev-nb-live", EventType: "observation", Stage: "notebook",
			Content: izNoteMarkdown, RecordedAt: "2026-11-18T11:30:00Z",
		})
		liveApp := &app{
			snap:     liveSnap,
			runSvc:   &sealStubSvc{snap: liveSnap, proj: izNoteProj},
			asterDoc: &asterdoc.FrozenAsterDoc{DesignID: "visual-design-1", Revision: 4, Source: izSrc},
		}
		renderVisualPNGSteps(t, outDir, "notebook_live_and_source_v2.png", 1360, 880, false, func(c *ui.Context) {
			notebookPage(c, liveApp)
		}, func(tt *ui.Tester) {
			if !tt.HasText("编辑") || !tt.HasText("阅读") || !tt.HasText("源码") {
				t.Fatalf("3-mode segmented control missing: %q", tt.Texts())
			}
			if !tt.HasText("实测模块") || !tt.HasText("导出 Markdown") || !tt.HasText("导出 PDF") {
				t.Fatalf("3-mode toolbar/companion missing: %q", tt.Texts())
			}
		})
	}

	// 13. Modernized Tools & Pantry Workbench (Nested Storage Boxes/Racks + Sticker Shelf + Detail Dialog)
	{
		nearDays := uint32(180)
		pantryBoxes := []tools.PantryBox{
			{ID: "box-cryo-1", Name: "抗体冻存盒 #1", Kind: tools.BoxCryo9x9, ZoneKey: "-80", ShelfPos: "冻存架一层左", Rows: 9, Cols: 9},
			{ID: "box-ep-1", Name: "分子酶 EP 管盒 #1", Kind: tools.BoxEP50, ZoneKey: "-20", ShelfPos: "二层左", Rows: 5, Cols: 10},
			{ID: "box-pcr-1", Name: "qPCR 引物 96 孔盒", Kind: tools.BoxPCR96, ZoneKey: "-20", ShelfPos: "二层右", Rows: 8, Cols: 12},
			{ID: "box-f15-1", Name: "细胞房 15 mL 离心管架", Kind: tools.BoxFalcon15, ZoneKey: "4C", ShelfPos: "三层右", Rows: 5, Cols: 5},
			{ID: "box-f50-1", Name: "血清与母液 50 mL 管架", Kind: tools.BoxFalcon50, ZoneKey: "4C", ShelfPos: "二层中", Rows: 4, Cols: 4},
		}
		pantryItems := []runs.PantryItem{
			{ID: "p-1", Name: "SYBR Green 预混液 (2×)", Location: "-20°C 冰箱 · 分子酶 EP 管盒 #1 · A2", Status: "near_expiry", PurchasedOn: "2026-06-01", OpenedOn: "2026-10-20", ShelfLifeDays: &nearDays, ExpiresOn: "2026-11-28", Revision: 3},
			{ID: "p-2", Name: "兔抗-VEGFA 一抗", Location: "-80°C 超低温 · 抗体冻存盒 #1 · C2", Status: "opened", PurchasedOn: "2026-05-12", OpenedOn: "2026-09-04", ShelfLifeDays: &nearDays, ExpiresOn: "2027-05-12", Revision: 1},
			{ID: "p-3", Name: "T4 DNA 连接酶", Location: "-80°C 超低温 · 抗体冻存盒 #1 · A3", Status: "opened", PurchasedOn: "2026-07-10", OpenedOn: "2026-10-08", ShelfLifeDays: &nearDays, ExpiresOn: "2027-07-10", Revision: 1},
			{ID: "p-4", Name: "GAPDH 引物八联条", Location: "-20°C 冰箱 · qPCR 引物 96 孔盒 · B1–B8", Status: "exhausted", PurchasedOn: "2026-04-01", OpenedOn: "2026-06-15", ShelfLifeDays: &nearDays, ExpiresOn: "2027-04-01", Revision: 2},
			{ID: "p-5", Name: "DMEM 高糖培养基", Location: "4°C 冷藏架一层", Status: "opened", PurchasedOn: "2026-10-15", OpenedOn: "2026-11-02", ShelfLifeDays: &nearDays, ExpiresOn: "2027-04-15", Revision: 2},
			{ID: "p-6", Name: "1× 无菌 PBS 缓冲液", Location: "4°C 冷藏架二层", Status: "unopened", PurchasedOn: "2026-10-28", ShelfLifeDays: &nearDays, ExpiresOn: "2027-10-28", Revision: 1},
			{ID: "p-7", Name: "胎牛血清 (FBS) 分装", Location: "4°C 冷藏 · 血清与母液 50 mL 管架 · A1", Status: "opened", PurchasedOn: "2026-09-20", OpenedOn: "2026-10-25", ShelfLifeDays: &nearDays, ExpiresOn: "2027-03-20", Revision: 1},
			{ID: "p-8", Name: "万古霉素标准品", Location: "常温避光干燥柜 B1", Status: "unopened", PurchasedOn: "2026-09-15", ShelfLifeDays: &nearDays, ExpiresOn: "2027-09-15", Revision: 1},
			{ID: "p-9", Name: "4% 多聚甲醛固定液", Location: "常温避光试剂柜 A1", Status: "unopened", PurchasedOn: "2026-08-18", ShelfLifeDays: &nearDays, ExpiresOn: "2027-08-18", Revision: 1},
			{ID: "p-10", Name: "抗-CD31 一抗", Location: "-80°C 超低温 · 抗体冻存盒 #1 · C3", Status: "unopened", PurchasedOn: "2026-08-01", ShelfLifeDays: &nearDays, ExpiresOn: "2027-08-01", Revision: 1},
			{ID: "p-11", Name: "抗-GAPDH 内参", Location: "-80°C 超低温 · 抗体冻存盒 #1 · A1", Status: "opened", PurchasedOn: "2026-06-12", OpenedOn: "2026-09-10", ShelfLifeDays: &nearDays, ExpiresOn: "2027-06-12", Revision: 1},
			{ID: "p-12", Name: "慢病毒 GFP 浓缩", Location: "-80°C 超低温 · 抗体冻存盒 #1 · E5", Status: "near_expiry", PurchasedOn: "2026-05-01", OpenedOn: "2026-10-10", ShelfLifeDays: &nearDays, ExpiresOn: "2026-12-01", Revision: 1},
			{ID: "p-13", Name: "HUVEC P3 冻存", Location: "-80°C 超低温 · 抗体冻存盒 #1 · G8", Status: "unopened", PurchasedOn: "2026-09-01", ShelfLifeDays: &nearDays, ExpiresOn: "2027-09-01", Revision: 1},
			{ID: "p-14", Name: "Taq DNA 聚合酶", Location: "-20°C 冰箱 · 分子酶 EP 管盒 #1 · A4", Status: "opened", PurchasedOn: "2026-07-05", OpenedOn: "2026-10-01", ShelfLifeDays: &nearDays, ExpiresOn: "2027-07-05", Revision: 1},
			{ID: "p-15", Name: "dNTP Mix 10mM", Location: "-20°C 冰箱 · 分子酶 EP 管盒 #1 · B1", Status: "unopened", PurchasedOn: "2026-08-10", ShelfLifeDays: &nearDays, ExpiresOn: "2027-08-10", Revision: 1},
			{ID: "p-16", Name: "VEGFA 引物 F/R", Location: "-20°C 冰箱 · qPCR 引物 96 孔盒 · D3", Status: "unopened", PurchasedOn: "2026-09-18", ShelfLifeDays: &nearDays, ExpiresOn: "2027-09-18", Revision: 1},
			{ID: "p-17", Name: "CD31 引物 F/R", Location: "-20°C 冰箱 · qPCR 引物 96 孔盒 · D4", Status: "unopened", PurchasedOn: "2026-09-18", ShelfLifeDays: &nearDays, ExpiresOn: "2027-09-18", Revision: 1},
			{ID: "p-18", Name: "0.25% 胰酶 EDTA", Location: "4°C 冷藏 · 细胞房 15 mL 离心管架 · A1", Status: "near_expiry", PurchasedOn: "2026-06-15", OpenedOn: "2026-10-18", ShelfLifeDays: &nearDays, ExpiresOn: "2026-11-30", Revision: 1},
			{ID: "p-19", Name: "青链霉素双抗 100×", Location: "4°C 冷藏 · 细胞房 15 mL 离心管架 · A2", Status: "opened", PurchasedOn: "2026-09-10", OpenedOn: "2026-10-15", ShelfLifeDays: &nearDays, ExpiresOn: "2027-03-10", Revision: 1},
			{ID: "p-20", Name: "Matrigel 基底胶分装", Location: "4°C 冷藏 · 细胞房 15 mL 离心管架 · C3", Status: "unopened", PurchasedOn: "2026-10-01", ShelfLifeDays: &nearDays, ExpiresOn: "2027-10-01", Revision: 1},
			{ID: "p-21", Name: "10× PBS 无菌母液", Location: "4°C 冷藏 · 血清与母液 50 mL 管架 · B2", Status: "unopened", PurchasedOn: "2026-09-25", ShelfLifeDays: &nearDays, ExpiresOn: "2027-09-25", Revision: 1},
		}
		pantryMeta := map[string]tools.PantryItemMeta{
			"p-1": {
				VesselID:   labsticker.VesselScrewTube2,
				BoxID:      "box-ep-1",
				BoxSlot:    "A2",
				CatalogNo:  "Takara RR820A",
				LotNo:      "AJF1042A",
				VolumeNote: "2 mL · 余 40%",
				ProductURL: "https://www.takarabio.com/products/real-time-pcr/qpcr-master-mixes/tb-green-premix-ex-taq-ii",
				Notes:      "避光融化后轻柔颠倒混匀，建议分装成 200 uL 小管减少反复冻融。",
				Manuals: []tools.PantryManual{
					{ID: "m-1", FileName: "TB_Green_Premix_Ex_Taq_II_说明书.pdf", StoredRel: "p-1/manual.pdf", SizeBytes: 1843200, AddedOn: "2026-10-20"},
					{ID: "m-2", FileName: "Lot_AJF1042A_COA_质检报告.pdf", StoredRel: "p-1/coa.pdf", SizeBytes: 428100, AddedOn: "2026-10-20"},
				},
			},
			"p-2":  {VesselID: labsticker.VesselAmberMicrovial, BoxID: "box-cryo-1", BoxSlot: "C2", CatalogNo: "Abcam ab46154", LotNo: "GR341902", VolumeNote: "100 uL · 1:1000", ProductURL: "https://www.abcam.com/products/primary-antibodies/vegfa-antibody-ab46154.html", Manuals: []tools.PantryManual{{ID: "m-2b", FileName: "ab46154_VEGF_Datasheet.pdf", SizeBytes: 724000}}},
			"p-3":  {VesselID: labsticker.VesselCryovial2, BoxID: "box-cryo-1", BoxSlot: "A3", CatalogNo: "NEB M0202S", VolumeNote: "2 mL · Box A3", Manuals: []tools.PantryManual{{ID: "m-3", FileName: "NEB_M0202_Protocol.pdf", SizeBytes: 612000}}},
			"p-4":  {VesselID: labsticker.VesselPCRStrip, BoxID: "box-pcr-1", BoxSlot: "B1", BoxSlots: []string{"B1", "B2", "B3", "B4", "B5", "B6", "B7", "B8"}, VolumeNote: "0.2 mL×8 · 待补购"},
			"p-5":  {VesselID: labsticker.VesselMediaBottle500, CatalogNo: "Gibco 11965092", VolumeNote: "500 mL · 含谷氨酰胺", ProductURL: "https://www.thermofisher.com/order/catalog/product/11965092", Manuals: []tools.PantryManual{{ID: "m-5", FileName: "Gibco_DMEM_COA.pdf", SizeBytes: 912000}}},
			"p-6":  {VesselID: labsticker.VesselReagentGL45, VolumeNote: "500 mL · pH 7.4", ProductURL: "https://www.thermofisher.com/order/catalog/product/10010023"},
			"p-7":  {VesselID: labsticker.VesselFalcon50, BoxID: "box-f50-1", BoxSlot: "A1", CatalogNo: "Gibco 10099141C", VolumeNote: "50 mL · 澳洲血源", Manuals: []tools.PantryManual{{ID: "m-7", FileName: "FBS_Endotoxin_Report.pdf", SizeBytes: 540000}}},
			"p-8":  {VesselID: labsticker.VesselSerumVial, VolumeNote: "10 mL · 冻干标准品", ProductURL: "https://www.sigmaaldrich.com/catalog/product/sial/v2002"},
			"p-9":  {VesselID: labsticker.VesselAmberBottle500, VolumeNote: "500 mL · 通风橱旁", Manuals: []tools.PantryManual{{ID: "m-9", FileName: "PFA_4pct_SDS_安全说明.pdf", SizeBytes: 780000}}},
			"p-10": {VesselID: labsticker.VesselAmberMicrovial, BoxID: "box-cryo-1", BoxSlot: "C3", CatalogNo: "CST #3528", LotNo: "L0814", VolumeNote: "100 uL · WB/IF"},
			"p-11": {VesselID: labsticker.VesselCryovial2, BoxID: "box-cryo-1", BoxSlot: "A1", CatalogNo: "Proteintech 60004", VolumeNote: "150 uL · 1:5000"},
			"p-12": {VesselID: labsticker.VesselCryovial2, BoxID: "box-cryo-1", BoxSlot: "E5", VolumeNote: "250 uL · 1e8 TU/mL"},
			"p-13": {VesselID: labsticker.VesselCryovial2, BoxID: "box-cryo-1", BoxSlot: "G8", VolumeNote: "1.5 mL · 1e6 cells"},
			"p-14": {VesselID: labsticker.VesselScrewTube2, BoxID: "box-ep-1", BoxSlot: "A4", VolumeNote: "5 U/uL · 250 U"},
			"p-15": {VesselID: labsticker.VesselEP15, BoxID: "box-ep-1", BoxSlot: "B1", VolumeNote: "10 mM · 1 mL"},
			"p-16": {VesselID: labsticker.VesselEP15, BoxID: "box-pcr-1", BoxSlot: "D3", VolumeNote: "10 uM · qPCR 通用"},
			"p-17": {VesselID: labsticker.VesselEP15, BoxID: "box-pcr-1", BoxSlot: "D4", VolumeNote: "10 uM · qPCR 通用"},
			"p-18": {VesselID: labsticker.VesselFalcon15, BoxID: "box-f15-1", BoxSlot: "A1", VolumeNote: "15 mL · 含酚红"},
			"p-19": {VesselID: labsticker.VesselFalcon15, BoxID: "box-f15-1", BoxSlot: "A2", VolumeNote: "10 mL · 100×"},
			"p-20": {VesselID: labsticker.VesselFalcon15, BoxID: "box-f15-1", BoxSlot: "C3", VolumeNote: "5 mL · 4°C 过夜预融"},
			"p-21": {VesselID: labsticker.VesselFalcon50, BoxID: "box-f50-1", BoxSlot: "B2", VolumeNote: "45 mL · 无 RNase"},
		}
		toolApp := &app{
			tools: tools.New(tools.Callbacks{
				ListPantry: func() (runs.PantryLedger, error) {
					return runs.PantryLedger{Items: pantryItems}, nil
				},
				LoadPantryMeta: func() (map[string]tools.PantryItemMeta, error) {
					return pantryMeta, nil
				},
				LoadPantryBoxes: func() ([]tools.PantryBox, error) {
					return pantryBoxes, nil
				},
				ListSOPs: func() []tools.SOPEntry {
					return []tools.SOPEntry{
						{ID: "sop-rt-qpcr", Title: "RT-qPCR 多靶点相对定量", Category: "分子生物学", Summary: "96 孔三复孔 × 靶基因 + MasterMix 配平", Source: qpcrSrc},
						{ID: "sop-huvec", Title: "HUVEC 水凝胶血管成管测定", Category: "细胞与组织工程", Summary: "12 孔水凝胶 + VEGF 梯度稀释与成管定量", Source: huvecSrc},
						{ID: "sop-cfu", Title: "梯度稀释涂板菌落计数 (CFU)", Category: "微生物学", Summary: "10 倍梯度涂板计数与 CFU/mL 计算", Source: cfuSrc},
						{ID: "sop-iz", Title: "90 mm 琼脂平皿四象限抑菌圈测定", Category: "微生物与抗菌", Summary: "1 mm = 2.4 px 纯 SVG 排布 + 双径卡尺测量", Source: izSrc},
					}
				},
			}),
		}
		renderVisualPNG(t, outDir, "tools_pantry_workbench_v4.png", 1360, 880, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})

		// Capture 5 BioRender box/rack die-cut sticker catalog
		renderVisualPNG(t, outDir, "comp_16e_labsticker_boxes_v1.png", 1280, 420, false, func(c *ui.Context) {
			labsticker.RenderBoxCatalogSheet(c)
		})

		// Capture Level 2 Drill-Down: 9x9 Cryobox interior (Light + Dark)
		toolApp.tools.OpenPantryBox("box-cryo-1", "C2")
		renderVisualPNG(t, outDir, "tools_pantry_box_cryo_9x9_light.png", 1360, 900, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})
		renderVisualPNG(t, outDir, "tools_pantry_box_cryo_9x9_dark.png", 1360, 900, true, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})

		// Capture Level 2 Drill-Down: 8x12 96-well PCR Box showing 8-strip + tubes + empty slot assignment
		toolApp.tools.OpenPantryBox("box-pcr-1", "B1")
		renderVisualPNG(t, outDir, "tools_pantry_box_pcr_96_light.png", 1360, 900, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})

		// Capture Level 2 Drill-Down: 5x5 15mL Conical Tube Rack
		toolApp.tools.OpenPantryBox("box-f15-1", "A1")
		renderVisualPNG(t, outDir, "tools_pantry_box_falcon_rack_light.png", 1360, 900, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})
		toolApp.tools.ClosePantryBox()

		// Capture opened Reagent Detail Dialog with BioRender hero sticker + live status badge + box/slot selector + vendor URL + attached PDF manuals
		toolApp.tools.OpenPantryDetail(pantryItems[0])
		renderVisualPNG(t, outDir, "comp_16d_pantry_reagent_detail_v3.png", 1360, 880, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})
		toolApp.tools.PantryDialogOpen = false

		// 13b. 标准方案库 (SOP Library)
		toolApp.tools.Active = tools.ToolSOP
		renderVisualPNG(t, outDir, "tools_sop_library_v1.png", 1360, 1020, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})

		// 13c. 实验台速算 (4-in-1 Bench Calculator)
		toolApp.tools.Active = tools.ToolDilution
		toolApp.tools.BenchCalc.Mode = 3 // PCR MasterMix showcase (or toggle cleanly)
		renderVisualPNG(t, outDir, "tools_bench_calc_v1.png", 1360, 880, false, func(c *ui.Context) {
			toolsPage(c, toolApp)
		})

		// 14. 孔板速排（7 规格不含培养皿 + `+ 追加标签` 精修悬浮条 + SVG/PNG 导出 + 两列孔板历史）
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

		plateStudio.SelectPlatePreset("rack-8")
		plateStudio.SelectPlateWells([]string{"A1", "A2", "A3", "A4"})
		_ = plateStudio.AnnotateSelectedWells("100/2 uM")
		plateStudio.SaveCurrentPlateToHistory()

		plateStudio.SelectPlatePreset("plate-24")
		plateStudio.SelectPlateWells([]string{"A1", "A2", "A3"})
		_ = plateStudio.AnnotateSelectedWells("空载对照")
		plateStudio.SelectPlateWells([]string{"B1", "B2", "B3"})
		_ = plateStudio.AnnotateSelectedWells("梯度处理")
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

		renderVisualPNG(t, outDir, "tools_plate_studio_v3.png", 1360, 1400, false, func(c *ui.Context) {
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
		renderVisualPNGSteps(t, outDir, "tools_plate_studio_overlay_v3.png", 1360, 920, false, func(c *ui.Context) {
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

		// 15. Regenerate updated plate@1 component specimens (no dish-4; 6/12/24/8-strip) & fixed CJK figure@1 SVG
		geomSrc := "```asterlab\nkind: group\nid: g-pbs\nschema: 1\nname: 空白对照\nrole: control\n```\n" +
			"```asterlab\nkind: group\nid: g-hama\nschema: 1\nname: 实验处理\nrole: treatment\n```\n" +
			"```asterlab\nkind: group\nid: g-van\nschema: 1\nname: 阳性对照\nrole: comparator\n```\n" +
			"```asterlab\nkind: plate\nid: pl-6\nschema: 1\ntitle: 6 孔细胞培养板（48px 大孔）\nformat: 6\ngroups:\n  g-pbs: [A1, A2, A3]\n  g-hama: [B1, B2, B3]\nsamples: {A1: Ctrl-1, A2: Ctrl-2, A3: Ctrl-3, B1: Drug-1, B2: Drug-2, B3: Drug-3}\n```\n" +
			"```asterlab\nkind: plate\nid: pl-12\nschema: 1\ntitle: 12 孔细胞铺板（42px 孔）\nformat: 12\ngroups:\n  g-pbs: [A1, A2]\n  g-hama: [B1, B2]\n  g-van: [C1, C2]\n```\n" +
			"```asterlab\nkind: plate\nid: pl-24\nschema: 1\ntitle: 24 孔梯度传代板（36px 孔）\nformat: 24\ngroups:\n  g-pbs: [A1, A2, A3]\n  g-hama: [B1, B2, B3]\n  g-van: [C1, C2, C3]\n```\n" +
			"```asterlab\nkind: plate\nid: pl-rack\nschema: 1\ntitle: 8 联管 / 电泳泳道（40×58px 竖向胶囊）\nformat: 8\ntopology: rack_1d\nrows: 1\ncolumns: 8\ngroups:\n  g-pbs: [A1, A2]\n  g-hama: [A3, A4, A5]\n  g-van: [A6, A7, A8]\nsamples: {A1: Ladder, A2: Ctrl, A3: Sample, A4: Sample, A5: Sample, A6: KO1, A7: KO2, A8: NTC}\n```\n"
		geomDoc, err := asterdoc.Parse(geomSrc)
		if err != nil {
			t.Fatalf("parse geomSrc: %v", err)
		}
		geomView := &asterdoc.View{
			Doc: geomDoc, Mode: asterdoc.ModeDesign,
			Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
		}
		renderVisualPNG(t, outDir, "comp_12_plate_geometries_v3.png", 1240, 920, false, func(c *ui.Context) {
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

		// Capture updated comp_04_figure_v2.png showcasing <g transform> + CJK alignment on 90 mm Petri Dish SVG
		if izDoc, err := asterdoc.Parse(izSrc); err == nil {
			for _, m := range izDoc.Modules {
				if m.Kind == "step" && m.ID == "step-02-quadrants" {
					if seq, ok := m.Data["blocks"].([]any); ok {
						for _, raw := range seq {
							if bm, _ := raw.(map[string]any); bm != nil && bm["kind"] == "figure" {
								figView := &asterdoc.View{Doc: izDoc, Mode: asterdoc.ModeDesign, Allowed: map[string]bool{asterdoc.PermDocumentEdit: true}}
								renderVisualPNG(t, outDir, "comp_04_figure_v2.png", 760, 420, false, func(c *ui.Context) {
									tok := theme.GetTokens(c.Theme())
									ui.Box(c).Fill().Padding(float32(tok.Spacing16)).Children(func() {
										figView.RenderModule(c, "figure", 1, bm)
									})
								})
								break
							}
						}
					}
				}
			}
		}

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
			renderVisualPNGSteps(t, outDir, "comp_13_plate_96_v3.png", 1120, 740, false, func(c *ui.Context) {
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
			renderVisualPNGSteps(t, outDir, "comp_13b_plate_overlay_bar_v3.png", 1200, 840, false, func(c *ui.Context) {
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

		// 16. Single-Page Continuous Sheet Preview (阅读 + 导出 PDF) on RT-qPCR
		sheetApp := makeDesignApp(qpcrSrc, "", 1)
		renderVisualPNG(t, outDir, "design_single_page_sheet_v2.png", 1360, 1180, false, func(c *ui.Context) {
			asterDesignPage(c, sheetApp)
		})
	}
}
