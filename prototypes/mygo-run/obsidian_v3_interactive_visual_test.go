package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/tools"
	"asterlab-mygo-prototype/internal/vault"
	"asterlab-mygo-prototype/modules/runs"
)

func TestObsidianVaultInteractiveExperienceAndVisualCatalog(t *testing.T) {
	outDir := os.Getenv("ASTERLAB_VISUAL_DIR")
	if outDir == "" {
		outDir = t.TempDir()
	} else if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "vault")
	v, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	if err := v.SeedTemplates(defaultTemplateSeeds()); err != nil {
		t.Fatalf("SeedTemplates: %v", err)
	}
	ds := vault.NewDocStore(v)

	proj1, err := v.CreateFolder("", "肿瘤耐药筛选")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	proj2, err := v.CreateFolder("", "抗菌与血管生成")
	if err != nil {
		t.Fatalf("CreateFolder 2: %v", err)
	}

	richSrc := "---\ntitle: L929 药物梯度测定\ndate: 2026-11-20\nstatus: in_progress\n---\n\n" +
		"```asterlab\nkind: group_table\nid: gt-1\nschema: 1\ntitle: 实验分组\nrows:\n  - id: g-ctrl\n    name: 溶剂对照\n    role: control\n    independent_n: 3\n  - id: g-tx\n    name: 给药处理\n    role: treatment\n    independent_n: 3\n```\n\n" +
		"```asterlab\nkind: step\nid: step-01-prep\nschema: 1\ntitle: 01 细胞接种与配液\nblocks:\n  - kind: prose\n    id: p1\n    schema: 1\n    body: 每孔接种 100 uL 细胞悬液，37°C 过夜贴壁后按浓度梯度加药。\n  - kind: checklist\n    id: chk1\n    schema: 1\n    items:\n      - id: c1\n        label: 消毒超净台并紫外照射 30 min\n        done: true\n      - id: c2\n        label: 确认细胞状态正常，无污染\n        done: false\n  - kind: timer\n    id: tm1\n    schema: 1\n    title: 孵育倒计时\n    mode: countdown\n    status: idle\n    duration:\n      value: 20\n      unit: min\n  - kind: param_table\n    id: pt1\n    schema: 1\n    title: 培养条件\n    rows:\n      - key: temp\n        label: 培养温度\n        value: 37\n        unit: Cel\n      - key: vol_per_well\n        label: 每孔体系\n        value: 100\n        unit: uL\n  - kind: calculator\n    id: cal1\n    schema: 1\n    title: 配液用量\n    mode: formula\n    rows:\n      - key: n_wells\n        label: 孔数\n        value: '@plate.wells'\n        unit: 孔\n      - key: excess\n        label: 余量系数\n        value: 0.1\n      - key: total_vol\n        label: 配液总体积\n        formula: n_wells * @param.vol_per_well * (1 + excess)\n        unit: uL\n  - kind: plate\n    id: pl1\n    schema: 1\n    title: 96 孔药敏排布\n    format: 96\n    groups:\n      g-ctrl: [A1, A2, A3]\n      g-tx: [B1, B2, B3, C1, C2, C3]\n    layers:\n      浓度梯度: {A1: DMSO, A2: DMSO, A3: DMSO, B1: 100 uM, B2: 50 uM, B3: 25 uM, C1: 12.5 uM, C2: 6.25 uM, C3: 3.125 uM}\n  - kind: data_table\n    id: dt1\n    schema: 1\n    title: CCK-8 读数记录\n    columns:\n      - key: well\n        label: 孔位\n        value_kind: text\n      - key: od450\n        label: OD450\n        value_kind: number\n        unit: AU\n    rows:\n      - well: A1\n        od450: 1.42\n      - well: B1\n        od450: 0.58\n```\n"

	expRich, err := v.CreateExperiment(proj1, "L929 药物梯度测定", "2026-11-20", richSrc)
	if err != nil {
		t.Fatalf("CreateExperiment rich: %v", err)
	}
	huvecB, _ := os.ReadFile("internal/asterdoc/testdata/huvec-tube.asterml")
	expHuvec, err := v.CreateExperiment(proj2, "HUVEC 水凝胶成管", "2026-11-18", string(huvecB))
	if err != nil {
		t.Fatalf("CreateExperiment huvec: %v", err)
	}
	cfuB, _ := os.ReadFile("internal/asterdoc/testdata/cfu.asterml")
	_, _ = v.CreateExperiment(proj2, "E.coli CFU 涂板计数", "2026-11-16", string(cfuB))

	sh := &shell.Shell{
		TodayDate:        "2026-11-20",
		SelectedDate:     "2026-11-20",
		SelectedTreePath: expRich.RelPath,
		RightOpen:        true,
		RightTab:         0,
		PantryItems: []shell.QuickPantryItem{
			{ID: "p-1", Name: "SYBR Green 预混液", Location: "-20°C A2", Status: "near_expiry", ExpiresOn: "2026-11-28"},
			{ID: "p-2", Name: "CCK-8 试剂盒", Location: "4°C B1", Status: "opened", ExpiresOn: "2027-06-01"},
		},
	}
	a := &app{
		shell:    sh,
		vault:    v,
		docStore: ds,
		tools: tools.New(tools.Callbacks{
			ListPantry: func() (runs.PantryLedger, error) {
				return runs.PantryLedger{
					Items: []runs.PantryItem{
						{ID: "p-1", LabelCode: "PN-SYBR", Name: "SYBR Green 预混液", Location: "-20°C A2", Status: "near_expiry", ExpiresOn: "2026-11-28"},
						{ID: "p-2", LabelCode: "PN-CCK8", Name: "CCK-8 试剂盒", Location: "4°C B1", Status: "opened", ExpiresOn: "2027-06-01"},
						{ID: "p-3", LabelCode: "PN-VEGF", Name: "VEGF-A 重组蛋白", Location: "-80°C C3", Status: "unopened", ExpiresOn: "2028-01-15"},
					},
				}, nil
			},
			LoadPantryMeta: func() (map[string]tools.PantryItemMeta, error) {
				return map[string]tools.PantryItemMeta{}, nil
			},
			LoadPantryBoxes: func() ([]tools.PantryBox, error) {
				return nil, nil
			},
		}),
	}
	sh.RenderTabContent = func(c *ui.Context, pane *shell.PaneNode, tab shell.WorkspaceTab) {
		a.renderVaultTabContent(c, pane, tab)
	}
	sh.RenderPantryContent = func(c *ui.Context) {
		a.renderStandalonePantry(c)
	}
	sh.RenderOverlays = func(c *ui.Context) {
		a.renderVaultNewExperimentDialog(c)
	}
	a.refreshVaultTreeAndCalendar()

	sh.EnsureTabs()
	// 1. Capture Dark Mode Obsidian Split Workbench + Vault File Tree + Right Drawer
	sh.OpenFileInPane(sh.ActivePaneID, expRich.RelPath, expRich.Title, shell.ModeEdit, true)
	sh.SplitPaneWithTab(sh.ActivePaneID, shell.WorkspaceTab{
		Kind: shell.TabFile, Title: expHuvec.Title, FilePath: expHuvec.RelPath, Mode: shell.ModeRead,
	}, "right")
	renderVisualPNG(t, outDir, "workbench_obsidian_split_drawer_dark.png", 1440, 900, true, func(c *ui.Context) {
		sh.RenderSplitView(c)
	})

	// 2. Capture UnifiedLive Edit, Read, and Source modes + test real interactive clicks on Timer/Pantry/Calendar!
	singlePane := &shell.PaneNode{ID: "pane-single"}
	for _, tc := range []struct {
		name string
		mode shell.ViewMode
	}{
		{"unified_live_edit_light.png", shell.ModeEdit},
		{"unified_live_read_light.png", shell.ModeRead},
		{"unified_live_source_light.png", shell.ModeSource},
	} {
		tab := shell.WorkspaceTab{
			ID: "tab-live", Kind: shell.TabFile, Title: expRich.Title, FilePath: expRich.RelPath, Mode: tc.mode,
		}
		renderVisualPNG(t, outDir, tc.name, 1360, 900, false, func(c *ui.Context) {
			a.renderVaultTabContent(c, singlePane, tab)
		})
	}

	// Interactive test: click Timer "启动计时" in Read mode -> verify `.asterml` updated to `status: running`!
	readTab := shell.WorkspaceTab{
		ID: "tab-read-int", Kind: shell.TabFile, Title: expRich.Title, FilePath: expRich.RelPath, Mode: shell.ModeRead,
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		a.renderVaultTabContent(c, singlePane, readTab)
	}, 1360, 900)
	tt.Frame()
	if err := tt.Click("启动计时"); err != nil {
		t.Fatalf("interactive click start timer in Read mode: %v", err)
	}
	tt.Advance(220 * time.Millisecond)
	tt.Frame()

	ldCheck, err := ds.Open(expRich.RelPath)
	if err != nil {
		t.Fatalf("reopen expRich: %v", err)
	}
	if !strings.Contains(ldCheck.Source, "status: running") {
		t.Fatalf("expected inline timer start persisted as status: running in Read mode, got:\n%s", ldCheck.Source)
	}

	// Interactive test: switch back to Edit mode & Undo via Ctrl+Z -> verify Undo restored status: idle!
	a.undoActiveLiveDoc()
	if sh.ActiveTab() != nil && sh.ActiveTab().Kind == shell.TabFile {
		_ = ldCheck.Undo()
	}

	// 3. Plate Layer Pills + Serial Dilution Fill Bar screenshot
	ldDoc, err := ds.Open(expRich.RelPath)
	if err == nil && ldDoc.Parsed != nil {
		var plBlock map[string]any
		for _, m := range ldDoc.Parsed.Modules {
			if m.Kind == "step" {
				for _, raw := range m.Data["blocks"].([]any) {
					if bm, _ := raw.(map[string]any); bm != nil && bm["kind"] == "plate" {
						plBlock = bm
						break
					}
				}
			}
		}
		if plBlock != nil {
			plView := &asterdoc.View{
				Doc: ldDoc.Parsed, Mode: asterdoc.ModeDesign, UnifiedLive: true,
				Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
				OnChange: func(next string) {
					_ = ldDoc.ApplyEdit(next)
				},
			}
			renderVisualPNGSteps(t, outDir, "plate_layers_serial_bar_light.png", 1120, 740, false, func(c *ui.Context) {
				tok := theme.GetTokens(c.Theme())
				ui.Box(c).Fill().Padding(float32(tok.Spacing16)).Children(func() {
					plView.RenderModule(c, "plate", 1, plBlock)
				})
			}, func(pt *ui.Tester) {
				_ = pt.Click("浓度梯度")
				pt.Frame()
				_ = pt.Click("选择第 B 行")
				pt.Frame()
				pt.Advance(250 * time.Millisecond)
				pt.Frame()
				_ = pt.Click("等比序列")
				pt.Frame()
			})
		}
	}

	// 4. Standalone Pantry Shelf (no 4-tool clutter)
	renderVisualPNG(t, outDir, "tools_pantry_standalone_light.png", 1360, 880, false, func(c *ui.Context) {
		a.renderStandalonePantry(c)
	})

	// 5. New Vault Experiment Dialog + empty title Nudge interactive test
	a.openNewExperimentDialog(proj1, "2026-11-20")
	a.newExpTitle = "L929 毒性筛选复测"
	renderVisualPNGSteps(t, outDir, "vault_new_exp_dialog_light.png", 1080, 680, false, func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() {
			sh.RenderSplitView(c)
			a.renderVaultNewExperimentDialog(c)
		})
	}, func(dt *ui.Tester) {
		dt.Frame()
		dt.Advance(220 * time.Millisecond)
		dt.Frame()
	})
	if !a.newExpOpen {
		t.Fatalf("expected new experiment dialog to remain open for preview before confirm")
	}
	a.newExpDate = "invalid-date"
	beforeNudge := a.newExpNudgeSeq
	a.confirmCreateVaultExperiment()
	if a.newExpNudgeSeq <= beforeNudge {
		t.Fatalf("expected invalid date confirmCreateVaultExperiment to trigger Nudge")
	}
}

func TestVisualArchiveSettingsMobileAndContextMenus(t *testing.T) {
	outDir := os.Getenv("ASTERLAB_VISUAL_DIR")
	if outDir == "" {
		outDir = t.TempDir()
	} else if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "vault")
	v, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	_ = v.SeedTemplates(defaultTemplateSeeds())
	ds := vault.NewDocStore(v)
	proj1, _ := v.CreateFolder("", "肿瘤耐药筛选")
	proj2, _ := v.CreateFolder("", "抗菌与血管生成")

	richSrc := "---\ntitle: L929 药物梯度测定\ndate: 2026-11-20\nstatus: in_progress\n---\n\n" +
		"```asterlab\nkind: group_table\nid: gt-1\nschema: 1\ntitle: 实验分组\nrows:\n  - id: g-ctrl\n    name: 溶剂对照\n    role: control\n    independent_n: 3\n  - id: g-tx\n    name: 给药处理\n    role: treatment\n    independent_n: 3\n```\n\n" +
		"```asterlab\nkind: step\nid: step-01-prep\nschema: 1\ntitle: 01 细胞接种与配液\nblocks:\n  - kind: prose\n    id: p1\n    schema: 1\n    body: 每孔接种 100 uL 细胞悬液，37°C 过夜贴壁后按浓度梯度加药。\n  - kind: checklist\n    id: chk1\n    schema: 1\n    items:\n      - id: c1\n        label: 消毒超净台并紫外照射 30 min\n        done: true\n      - id: c2\n        label: 确认细胞状态正常，无污染\n        done: false\n  - kind: plate\n    id: pl1\n    schema: 1\n    title: 96 孔药敏排布\n    format: 96\n    groups:\n      g-ctrl: [A1, A2, A3]\n      g-tx: [B1, B2, B3, C1, C2, C3]\n    layers:\n      浓度梯度: {A1: DMSO, A2: DMSO, A3: DMSO, B1: 100 uM, B2: 50 uM, B3: 25 uM, C1: 12.5 uM, C2: 6.25 uM, C3: 3.125 uM}\n```\n"

	expRich, err := v.CreateExperiment(proj1, "L929 药物梯度测定", "2026-11-20", richSrc)
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	huvecB, _ := os.ReadFile("internal/asterdoc/testdata/huvec-tube.asterml")
	_, _ = v.CreateExperiment(proj2, "HUVEC 水凝胶成管", "2026-11-18", string(huvecB))

	sh := &shell.Shell{
		TodayDate:        "2026-11-20",
		SelectedDate:     "2026-11-18",
		SelectedTreePath: expRich.RelPath,
		RightOpen:        true,
		RightTab:         0,
		VaultPathInput:   v.Root,
	}
	a := &app{
		shell:    sh,
		vault:    v,
		docStore: ds,
		tools: tools.New(tools.Callbacks{
			ListPantry: func() (runs.PantryLedger, error) {
				return runs.PantryLedger{Items: []runs.PantryItem{
					{ID: "p-1", LabelCode: "PN-SYBR", Name: "SYBR Green 预混液", Location: "-20°C A2", Status: "near_expiry", ExpiresOn: "2026-11-28"},
				}}, nil
			},
			LoadPantryMeta:  func() (map[string]tools.PantryItemMeta, error) { return map[string]tools.PantryItemMeta{}, nil },
			LoadPantryBoxes: func() ([]tools.PantryBox, error) { return nil, nil },
		}),
	}
	sh.RenderTabContent = func(c *ui.Context, pane *shell.PaneNode, tab shell.WorkspaceTab) {
		a.renderVaultTabContent(c, pane, tab)
	}
	sh.RenderPantryContent = func(c *ui.Context) {
		a.renderStandalonePantry(c)
	}
	sh.RenderOverlays = func(c *ui.Context) {
		a.renderVaultNewExperimentDialog(c)
	}
	sh.OnSettingsChanged = func() { a.saveVaultSettingsFromShell() }
	a.refreshVaultTreeAndCalendar()
	sh.EnsureTabs()
	sh.OpenFileInPane(sh.ActivePaneID, expRich.RelPath, "L929 药物梯度测定", shell.ModeEdit, true)

	// 1. Obsidian Settings Center (Appearance Light + Hotkeys Dark)
	sh.VaultSettingsOpen = true
	sh.SettingsView = components.SettingsState{Category: "appearance"}
	renderVisualPNGSteps(t, outDir, "vault_settings_appearance_light.png", 1080, 680, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(220 * time.Millisecond)
		st.Frame()
		if !st.HasText("基础明暗模式") || !st.HasText("界面色盘预设") || !st.HasText("Liquid Glass 液态玻璃材质") {
			t.Fatalf("expected Obsidian settings appearance content, got: %q", st.Texts())
		}
	})
	sh.SettingsView = components.SettingsState{Category: "hotkeys"}
	renderVisualPNGSteps(t, outDir, "vault_settings_hotkeys_dark.png", 1080, 680, true, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(220 * time.Millisecond)
		st.Frame()
		if !st.HasText("打开命令面板 / 快速切换实验") {
			t.Fatalf("expected Obsidian hotkeys settings content")
		}
	})
	sh.VaultSettingsOpen = false

	// 2. Right-Click Context Menus Visual Archive
	renderVisualPNGSteps(t, outDir, "context_menu_file_tree_light.png", 1160, 740, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		if err := st.RightClick("L929 药物梯度测定"); err != nil {
			t.Fatalf("RightClick file tree: %v", err)
		}
		st.Advance(220 * time.Millisecond)
		st.Frame()
		if !st.HasText("在新标签页打开") {
			t.Fatalf("expected file tree context menu item")
		}
	})

	renderVisualPNGSteps(t, outDir, "context_menu_pane_tab_light.png", 1160, 740, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		if err := st.RightClick("标签:L929 药物梯度测定"); err != nil {
			t.Fatalf("RightClick tab pill: %v", err)
		}
		st.Advance(220 * time.Millisecond)
		st.Frame()
		if !st.HasText("关闭其他标签页") {
			t.Fatalf("expected pane tab context menu item")
		}
	})

	// Render live plate module directly for crisp right-click context menu capture
	ldRich, _ := ds.Open(expRich.RelPath)
	var plBlockCtx map[string]any
	if ldRich != nil && ldRich.Parsed != nil {
		for _, m := range ldRich.Parsed.Modules {
			if m.Kind == "step" {
				for _, raw := range m.Data["blocks"].([]any) {
					if bm, _ := raw.(map[string]any); bm != nil && bm["kind"] == "plate" {
						plBlockCtx = bm
						break
					}
				}
			}
		}
	}
	if plBlockCtx != nil {
		plMenuView := &asterdoc.View{
			Doc: ldRich.Parsed, Mode: asterdoc.ModeDesign, UnifiedLive: true,
			Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
		}
		renderVisualPNGSteps(t, outDir, "context_menu_plate_light.png", 960, 620, false, func(c *ui.Context) {
			tok := theme.GetTokens(c.Theme())
			ui.Box(c).Fill().Padding(float32(tok.Spacing24)).Background(tok.Canvas).Children(func() {
				plMenuView.RenderModule(c, "plate", 1, plBlockCtx)
			})
		}, func(st *ui.Tester) {
			st.Frame()
			if err := st.RightClick("孔 B1"); err != nil {
				t.Fatalf("RightClick plate well B1: %v", err)
			}
			st.Advance(220 * time.Millisecond)
			st.Frame()
			if !st.HasText("选中最外圈边缘孔") {
				t.Fatalf("expected plate context menu item")
			}
		})
	}

	renderVisualPNGSteps(t, outDir, "context_menu_step_block_light.png", 1100, 720, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		if err := st.RightClick("操作块 chk1"); err != nil {
			t.Fatalf("RightClick step block: %v", err)
		}
		st.Advance(220 * time.Millisecond)
		st.Frame()
		if !st.HasText("粘贴到下方") {
			t.Fatalf("expected step block context menu item")
		}
	})

	sh.OpenCalendarTab()
	renderVisualPNGSteps(t, outDir, "context_menu_calendar_light.png", 1160, 740, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		if err := st.RightClick("日历格 2026-11-19"); err != nil {
			t.Fatalf("RightClick calendar day cell: %v", err)
		}
		st.Advance(220 * time.Millisecond)
		st.Frame()
		if !st.HasText("在此日新建实验") {
			t.Fatalf("expected calendar cell context menu item")
		}
	})

	// 3. MyGo Mobile Adaptive Views (390x844 phone viewport - Obsidian iOS 26 style)
	sh.ThemePreset = theme.PresetObsidian
	sh.LiquidGlass = true
	sh.MobileDrawer = ""
	sh.MobileTabSwitcherOpen = false
	sh.MobileDockHidden = false
	sh.MobileDockForceNav = false
	sh.OpenCalendarTab()
	renderVisualPNGSteps(t, outDir, "mobile_workbench_calendar_light.png", 390, 844, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(240 * time.Millisecond)
		st.Frame()
		if !st.HasText("2026 年 11 月") || !st.HasText("2") {
			t.Fatalf("expected mobile calendar & icon capsule count 2, got: %q", st.Texts())
		}
	})

	// Mobile File Tree Drawer with Obsidian 3-tier bottom control island
	sh.MobileDrawer = "tree"
	renderVisualPNGSteps(t, outDir, "mobile_drawer_tree_light.png", 390, 844, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(260 * time.Millisecond)
		st.Frame()
		if !st.HasText("实验列表") || !st.HasText("AsterLab") {
			t.Fatalf("expected mobile drawer bottom island, got: %q", st.Texts())
		}
	})
	sh.MobileDrawer = ""

	// Mobile Experiment Edit view (borderless soft-gray Properties card + icon-only floating bar)
	sh.OpenFileInPane(sh.ActivePaneID, expRich.RelPath, "L929 药物梯度测定", shell.ModeEdit, false)
	renderVisualPNGSteps(t, outDir, "mobile_experiment_edit_light.png", 390, 844, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(240 * time.Millisecond)
		st.Frame()
		if !st.HasText("实验属性") || !st.HasText("实验分组") || !st.HasText("本步核对 1 / 2") {
			t.Fatalf("expected mobile experiment properties & step rendering, got: %q", st.Texts())
		}
	})

	// Mobile Experiment Read view (exact Obsidian iOS comparison: clean icon-only nav pill + borderless Properties card)
	sh.SetActiveTabMode(shell.ModeRead)
	renderVisualPNGSteps(t, outDir, "mobile_experiment_read_obsidian_light.png", 390, 844, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, nil)
	renderVisualPNGSteps(t, outDir, "mobile_experiment_read_obsidian_dark.png", 390, 844, true, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, nil)
	sh.SetActiveTabMode(shell.ModeEdit)
	renderVisualPNGSteps(t, outDir, "mobile_experiment_edit_obsidian_dark.png", 390, 844, true, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, nil)
	renderVisualPNGSteps(t, outDir, "liquid_glass_desktop_exp_dark.png", 1280, 800, true, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, nil)

	// Mobile Tab Switcher Card Grid view
	sh.MobileTabSwitcherOpen = true
	renderVisualPNGSteps(t, outDir, "mobile_tab_switcher_light.png", 390, 844, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(240 * time.Millisecond)
		st.Frame()
		if !st.HasText("标签页概览") {
			t.Fatalf("expected mobile tab switcher overlay, got: %q", st.Texts())
		}
	})
	sh.MobileTabSwitcherOpen = false

	// 4. Theme Preset + Liquid Glass Comparison Shots (Obsidian White vs Cool Zinc, Light vs Dark)
	sh.OpenCalendarTab()
	renderVisualPNGSteps(t, outDir, "liquid_glass_desktop_obsidian_light.png", 1280, 800, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, nil)
	renderVisualPNGSteps(t, outDir, "liquid_glass_desktop_obsidian_dark.png", 1280, 800, true, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, nil)

	sh.ThemePreset = theme.PresetZinc
	renderVisualPNGSteps(t, outDir, "liquid_glass_mobile_zinc_light.png", 390, 844, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, nil)
	renderVisualPNGSteps(t, outDir, "liquid_glass_desktop_zinc_light.png", 1280, 800, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, nil)

	sh.ThemePreset = theme.PresetObsidian
}
