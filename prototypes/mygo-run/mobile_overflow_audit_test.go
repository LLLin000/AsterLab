package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/tools"
	"asterlab-mygo-prototype/internal/uikit/labsticker"
)

func saveAuditSnapshot(t *testing.T, tt *ui.Tester, filename string) {
	t.Helper()
	dir := os.Getenv("ASTERLAB_GALLERY_SNAPSHOT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir snapshot dir %s: %v", dir, err)
	}
	img := tt.Image()
	if img == nil {
		t.Fatalf("nil image for %s", filename)
	}
	outPath := filepath.Join(dir, filename)
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create %s: %v", outPath, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", outPath, err)
	}
}

func assertNoHorizontalOverflow402(t *testing.T, tt *ui.Tester, screen string) {
	t.Helper()
	var offenders []string
	for _, info := range tt.AccessAll() {
		if info.Offscreen || info.Bounds.W <= 0 || info.Bounds.H <= 0 {
			continue
		}
		if info.Bounds.X < -1.5 || info.Bounds.X+info.Bounds.W > 403.0 {
			offenders = append(offenders, fmt.Sprintf("role=%v name=%q value=%q bounds=%+v",
				info.Role, info.Name, info.Value, info.Bounds))
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("[%s] found %d horizontal overflow element(s) on 402dp viewport:\n  %s",
			screen, len(offenders), strings.Join(offenders, "\n  "))
	}
}

func assertHeaderSafeArea402(t *testing.T, tt *ui.Tester, screen, label string, topInset float32) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("[%s] expected header action/title %q, texts=%v", screen, label, tt.Texts())
	}
	if r.Y < topInset-2 {
		t.Fatalf("[%s] header element %q collides with top SafeArea (inset=%.1f): %+v", screen, label, topInset, r)
	}
	if r.X < 0 || r.X+r.W > 403 {
		t.Fatalf("[%s] header element %q out of 402dp bounds: %+v", screen, label, r)
	}
}

func TestOverflowAuditAllMobileSurfaces402x874(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "OverflowAuditVault")
	a, sh := newWiredFSMTestApp(t, vaultRoot)
	sh.ForceMobilePreview = true

	// Seed rich sample experiment containing ---, group_table, param_table, calculator, data_table, figure
	richExpRel := "2026-11-23-overflow-audit.asterml"
	richExpSrc := `---
title: "Mobile Overflow Full Audit"
date: "2026-11-23"
status: "in_progress"
---

#@ step-1 实验设计
准备实验分组与操作参数，以下为标准分割线与全部科学模块：

---

` + "```asterlab\nkind: group_table\nid: grp-1\nschema: 1\ntitle: 实验分组\nrows:\n  - id: ctrl\n    name: 空白对照组\n    role: control\n    condition: PBS Buffer\n    independent_n: 3\n    technical_repeats: 3\n  - id: treat\n    name: 高浓度处理组\n    role: treatment\n    condition: 100 ug/mL GelMA-Cu\n    independent_n: 3\n    technical_repeats: 3\n```" + `

` + "```asterlab\nkind: param_table\nid: par-1\nschema: 1\ntitle: 反应条件\nrows:\n  - key: temp\n    label: 孵育温度\n    value: 37\n    unit: °C\n  - key: dur\n    label: 反应时间\n    value: 60\n    unit: min\n```" + `

` + "```asterlab\nkind: calculator\nid: calc-1\nschema: 1\ntitle: 稀释配液\nmode: dilution\nstock:\n  value: 1000\n  unit: ug/mL\ntarget:\n  value: 100\n  unit: ug/mL\nfinal_volume:\n  value: 10\n  unit: mL\n```" + `

` + "```asterlab\nkind: data_table\nid: dt-1\nschema: 1\ntitle: 读数记录表\ncolumns:\n  - key: sample_id\n    label: 样本编号\n    value_kind: text\n  - key: od450\n    label: OD450 吸光度\n    value_kind: number\n    unit: AU\nrows:\n  - sample_id: Ctrl-1\n    od450: 0.214\n  - sample_id: Treat-1\n    od450: 0.892\n```" + `

` + "```asterlab\nkind: figure\nid: fig-1\nschema: 1\ntitle: 创面愈合示意图\ncaption: Day 7 荧光共聚焦成像示意\nsvg: |\n  <svg viewBox=\"0 0 360 220\" xmlns=\"http://www.w3.org/2000/svg\">\n    <rect x=\"12\" y=\"12\" width=\"336\" height=\"196\" rx=\"12\" fill=\"#e8eef8\" stroke=\"#5b7bb2\" stroke-width=\"2\"/>\n    <circle cx=\"120\" cy=\"110\" r=\"48\" fill=\"#7ca3dc\" opacity=\"0.75\"/>\n    <circle cx=\"230\" cy=\"110\" r=\"48\" fill=\"#3f8f6b\" opacity=\"0.75\"/>\n    <text x=\"180\" y=\"190\" text-anchor=\"middle\" font-size=\"14\" fill=\"#1f2937\">Sample Figure Preview</text>\n  </svg>\n```" + `
`
	if err := os.WriteFile(filepath.Join(vaultRoot, richExpRel), []byte(richExpSrc), 0o644); err != nil {
		t.Fatalf("write sample experiment: %v", err)
	}
	a.refreshVaultTreeAndCalendar()

	// Seed sample vivarium cage so Rack, Ledger, and SidePeek all render realistic data
	a.tools.VivariumCages = []tools.VivariumCage{
		{
			ID:            "cage-1",
			CageCode:      "SPF-A01",
			Species:       labsticker.AnimalSpeciesID("mouse"),
			Strain:        "C57BL/6J",
			Genotype:      "WT",
			MaleCount:     3,
			FemaleCount:   2,
			FacilityGrade: "SPF",
			Room:          "SPF-01",
			RackPos:       "R1-A2",
			DOB:           "2026-09-20",
			WeightG:       22.4,
			IACUCNo:       "IACUC-2026-0412",
			Status:        "in_study",
		},
	}

	const topInset = float32(47)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		sh.RenderSplitView(c)
	}, 402, 874)
	tt.SetInsets(topInset, 0, 34, 0)
	settleIPhoneFrames(tt, 200)

	// 1. Calendar Home
	sh.OpenCalendarTab()
	settleIPhoneFrames(tt, 160)
	assertNoHorizontalOverflow402(t, tt, "CalendarHome")

	// 2. Vivarium Rack & Ledger & SidePeek & Full-screen Form
	sh.OpenPantryTab()
	settleIPhoneFrames(tt, 160)
	if err := tt.Click("动物房"); err != nil {
		t.Fatalf("click 动物房 tab: %v", err)
	}
	settleIPhoneFrames(tt, 160)
	saveAuditSnapshot(t, tt, "01_vivarium_rack.png")
	assertNoHorizontalOverflow402(t, tt, "VivariumRack")

	if err := tt.Click("动物台账"); err != nil {
		t.Fatalf("click 动物台账 tab: %v", err)
	}
	settleIPhoneFrames(tt, 160)
	saveAuditSnapshot(t, tt, "02_vivarium_ledger.png")
	assertNoHorizontalOverflow402(t, tt, "VivariumLedger")

	a.tools.OpenVivariumPeek(a.tools.VivariumCages[0])
	settleIPhoneFrames(tt, 160)
	saveAuditSnapshot(t, tt, "03_vivarium_sidepeek_detail.png")
	assertHeaderSafeArea402(t, tt, "VivariumSidePeek", "返回", topInset)
	assertNoHorizontalOverflow402(t, tt, "VivariumSidePeek")
	a.tools.VivariumPeekOpen = false
	settleIPhoneFrames(tt, 120)

	a.tools.OpenNewVivariumDialog()
	settleIPhoneFrames(tt, 160)
	saveAuditSnapshot(t, tt, "04_vivarium_new_cage_page.png")
	assertHeaderSafeArea402(t, tt, "VivariumNewCage", "取消", topInset)
	assertHeaderSafeArea402(t, tt, "VivariumNewCage", "保存笼卡", topInset)
	assertNoHorizontalOverflow402(t, tt, "VivariumNewCage")
	a.tools.VivariumDialogOpen = false
	settleIPhoneFrames(tt, 120)

	// 3. Global Search Full-Screen Page
	sh.GlobalSearchOpen = true
	sh.GlobalSearchQuery = "Audit"
	settleIPhoneFrames(tt, 160)
	saveAuditSnapshot(t, tt, "05_global_search_page.png")
	assertHeaderSafeArea402(t, tt, "GlobalSearch", "取消", topInset)
	assertNoHorizontalOverflow402(t, tt, "GlobalSearch")
	sh.GlobalSearchOpen = false
	settleIPhoneFrames(tt, 120)

	// 4. New Folder Full-Screen Page (opened via Left Drawer bottom island)
	sh.MobileDrawer = "tree"
	settleIPhoneFrames(tt, 160)
	if err := tt.Click("新建文件夹"); err != nil {
		t.Fatalf("click 新建文件夹 in mobile drawer: %v", err)
	}
	settleIPhoneFrames(tt, 160)
	saveAuditSnapshot(t, tt, "06_new_folder_page.png")
	assertHeaderSafeArea402(t, tt, "NewFolder", "取消", topInset)
	assertHeaderSafeArea402(t, tt, "NewFolder", "创建", topInset)
	assertNoHorizontalOverflow402(t, tt, "NewFolder")
	sh.HandleMobileBack()
	sh.MobileDrawer = ""
	settleIPhoneFrames(tt, 120)

	// 5. Feedback Full-Screen Page
	sh.FeedbackOpen = true
	settleIPhoneFrames(tt, 160)
	saveAuditSnapshot(t, tt, "07_feedback_page.png")
	assertHeaderSafeArea402(t, tt, "Feedback", "取消", topInset)
	assertNoHorizontalOverflow402(t, tt, "Feedback")
	sh.FeedbackOpen = false
	settleIPhoneFrames(tt, 120)

	// 6. Welcome Full-Screen Page
	sh.WelcomeOpen = true
	settleIPhoneFrames(tt, 160)
	saveAuditSnapshot(t, tt, "08_welcome_page.png")
	assertHeaderSafeArea402(t, tt, "Welcome", "稍后探索", topInset)
	assertNoHorizontalOverflow402(t, tt, "Welcome")
	sh.WelcomeOpen = false
	settleIPhoneFrames(tt, 120)

	// 7. Spotlight Tour Mobile Overlay
	sh.TourOpen = true
	sh.TourStep = 0
	settleIPhoneFrames(tt, 160)
	saveAuditSnapshot(t, tt, "09_spotlight_tour_mobile.png")
	assertNoHorizontalOverflow402(t, tt, "SpotlightTour")
	sh.TourOpen = false
	settleIPhoneFrames(tt, 120)

	// 8. Obsidian Settings Level 1 + All 6 Level 2 Subpages
	sh.VaultSettingsOpen = true
	sh.SettingsView.Category = ""
	settleIPhoneFrames(tt, 160)
	assertHeaderSafeArea402(t, tt, "SettingsLevel1", "完成", topInset)
	assertNoHorizontalOverflow402(t, tt, "SettingsLevel1")

	for _, cat := range []struct {
		title string
		file  string
	}{
		{"文件与库", "10a_settings_vault.png"},
		{"多端同步", "10b_settings_sync.png"},
		{"外观", "10c_settings_appearance.png"},
		{"编辑器", "10d_settings_editor.png"},
		{"快捷键", "10e_settings_hotkeys.png"},
		{"关于", "10f_settings_about.png"},
	} {
		if err := tt.Click(cat.title); err != nil {
			t.Fatalf("open mobile settings category %q: %v", cat.title, err)
		}
		settleIPhoneFrames(tt, 160)
		saveAuditSnapshot(t, tt, cat.file)
		assertHeaderSafeArea402(t, tt, "Settings_"+cat.title, "返回设置", topInset)
		assertNoHorizontalOverflow402(t, tt, "Settings_"+cat.title)
		if err := tt.Click("返回设置"); err != nil {
			t.Fatalf("return from mobile settings category %q: %v", cat.title, err)
		}
		settleIPhoneFrames(tt, 140)
	}
	sh.VaultSettingsOpen = false
	settleIPhoneFrames(tt, 120)

	// 9. Experiment Document Page (group_table, param_table, calculator, data_table, figure, ---)
	a.openVaultFileInActivePane(richExpRel, "Mobile Overflow Full Audit")
	settleIPhoneFrames(tt, 200)
	saveAuditSnapshot(t, tt, "11_experiment_doc_mobile.png")
	assertNoHorizontalOverflow402(t, tt, "ExperimentDocTop")

	// Scroll down through modules and verify no element overflows horizontally anywhere
	for step := 1; step <= 4; step++ {
		tt.Scroll(200, 480, 0, 320)
		settleIPhoneFrames(tt, 120)
		assertNoHorizontalOverflow402(t, tt, fmt.Sprintf("ExperimentDocScroll%d", step))
		if step == 4 {
			saveAuditSnapshot(t, tt, "11b_experiment_doc_modules_bottom.png")
		}
	}

	// 10. Standalone AsterDoc Lightbox full-screen check at 402x874
	doc, err := asterdoc.Parse(richExpSrc)
	if err != nil {
		t.Fatalf("parse richExpSrc: %v", err)
	}
	docView := &asterdoc.View{
		Doc:           doc,
		Mode:          asterdoc.ModeDesign,
		Allowed:       map[string]bool{asterdoc.PermDocumentEdit: true},
		UnifiedLive:   true,
		MobileCompact: true,
	}
	docTester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		ui.Scroll(c.Key("aud-doc-scroll")).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(16).Gap(12).Children(func() {
				docView.Render(c)
			})
		})
	}, 402, 874)
	docTester.SetInsets(topInset, 0, 34, 0)
	settleIPhoneFrames(docTester, 160)
	for step := 0; step < 12; step++ {
		if r, ok := docTester.Find("放大查看"); ok && r.Y >= topInset && r.Y+r.H <= 840 {
			break
		}
		docTester.Scroll(200, 350, 0, 260)
		settleIPhoneFrames(docTester, 100)
	}
	if err := docTester.Click("放大查看"); err != nil {
		t.Fatalf("click 放大查看 on mobile figure: %v (texts=%v)", err, docTester.Texts())
	}
	settleIPhoneFrames(docTester, 160)
	saveAuditSnapshot(t, docTester, "12_figure_lightbox_mobile.png")
	assertHeaderSafeArea402(t, docTester, "FigureLightbox", "关闭", topInset)
	assertNoHorizontalOverflow402(t, docTester, "FigureLightbox")
}

func isHorizontalScrollChip(name string) bool {
	for _, prefix := range []string{"全部 (", "-80°C (", "-20°C (", "4°C (", "常温避光 (", "临期 ("} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func assertBoundsWithinWidth(t *testing.T, tt *ui.Tester, screen string, maxX float32) {
	t.Helper()
	var offenders []string
	for _, info := range tt.AccessAll() {
		if info.Offscreen || info.Bounds.W <= 0 || info.Bounds.H <= 0 {
			continue
		}
		if isHorizontalScrollChip(info.Name) {
			continue
		}
		if info.Bounds.X < -1.5 || info.Bounds.X+info.Bounds.W > maxX+1.0 {
			offenders = append(offenders, fmt.Sprintf("role=%v name=%q value=%q bounds=%+v",
				info.Role, info.Name, info.Value, info.Bounds))
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("[%s] found %d horizontal overflow element(s) on %.0fdp viewport:\n  %s",
			screen, len(offenders), maxX, strings.Join(offenders, "\n  "))
	}
}

func TestMultiDeviceSafeAreaMatrix(t *testing.T) {
	defer mobile.SetLook(mobile.Automatic)

	type deviceCase struct {
		name                             string
		look                             mobile.Look
		w, h                             int
		top, right, bottom, left         float32
		forceMobilePreview               bool
		expectDockHidden                 bool
		expectOrbMinY, expectOrbMaxY     float32
		expectDockMaxBottom              float32
		expectDockMinBottom              float32
		snapshot                         string
		openPantryToo                    bool
	}

	cases := []deviceCase{
		{
			name:                "iPhone16Pro_402x874",
			look:                mobile.Cupertino,
			w:                   402,
			h:                   874,
			top:                 59,
			bottom:              34,
			forceMobilePreview:  true,
			expectOrbMinY:       59,
			expectOrbMaxY:       72,
			expectDockMinBottom: 822,
			expectDockMaxBottom: 836,
			snapshot:            "device_iphone16pro_402x874.png",
			openPantryToo:       true,
		},
		{
			name:                "iPhoneSE_375x667",
			look:                mobile.Cupertino,
			w:                   375,
			h:                   667,
			top:                 20,
			forceMobilePreview:  true,
			expectOrbMinY:       20,
			expectOrbMaxY:       32,
			expectDockMinBottom: 647,
			expectDockMaxBottom: 663,
			snapshot:            "device_iphonese_375x667.png",
			openPantryToo:       true,
		},
		{
			name:                "iPhoneLandscape_874x402",
			look:                mobile.Cupertino,
			w:                   874,
			h:                   402,
			right:               59,
			bottom:              21,
			left:                59,
			forceMobilePreview:  true,
			expectOrbMinY:       0,
			expectOrbMaxY:       14,
			expectDockMinBottom: 361,
			expectDockMaxBottom: 381,
			snapshot:            "device_iphone_landscape_874x402.png",
		},
		{
			name:                "AndroidGesture_360x800",
			look:                mobile.Material,
			w:                   360,
			h:                   800,
			top:                 36,
			bottom:              24,
			forceMobilePreview:  true,
			expectOrbMinY:       36,
			expectOrbMaxY:       48,
			expectDockMinBottom: 752,
			expectDockMaxBottom: 770,
			snapshot:            "device_android_gesture_360x800.png",
			openPantryToo:       true,
		},
		{
			name:                "Android3Button_412x915",
			look:                mobile.Material,
			w:                   412,
			h:                   915,
			top:                 32,
			bottom:              48,
			forceMobilePreview:  true,
			expectOrbMinY:       32,
			expectOrbMaxY:       44,
			expectDockMinBottom: 843,
			expectDockMaxBottom: 863,
			snapshot:            "device_android_3btn_412x915.png",
		},
		{
			name:               "AndroidKeyboardOpen_412x915",
			look:               mobile.Material,
			w:                  412,
			h:                  915,
			top:                32,
			bottom:             310,
			forceMobilePreview: true,
			expectDockHidden:   true,
			expectOrbMinY:      32,
			expectOrbMaxY:      44,
		},
	}

	origLook := mobile.Current()
	t.Cleanup(func() { mobile.SetLook(origLook) })

	for _, dc := range cases {
		dc := dc
		t.Run(dc.name, func(t *testing.T) {
			mobile.SetLook(dc.look)
			vaultRoot := filepath.Join(t.TempDir(), "DeviceMatrix_"+dc.name)
			a, sh := newWiredFSMTestApp(t, vaultRoot)
			sh.ForceMobilePreview = dc.forceMobilePreview
			_ = a
			sh.OpenCalendarTab()

			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.LightTheme())
				sh.RenderSplitView(c)
			}, dc.w, dc.h)
			tt.SetInsets(dc.top, dc.right, dc.bottom, dc.left)
			settleIPhoneFrames(tt, 450)

			orbR, ok := tt.Find("打开实验库抽屉")
			if !ok {
				t.Fatalf("[%s] missing left orb", dc.name)
			}
			if orbR.Y < dc.expectOrbMinY || orbR.Y > dc.expectOrbMaxY {
				t.Fatalf("[%s] top orb Y=%.1f out of 1x SafeArea range [%.1f, %.1f]",
					dc.name, orbR.Y, dc.expectOrbMinY, dc.expectOrbMaxY)
			}
			if orbR.X < dc.left {
				t.Fatalf("[%s] top orb X=%.1f inside left SafeArea=%.1f", dc.name, orbR.X, dc.left)
			}
			if sr, ok := tt.Find("快速打开 (Ctrl+K)"); ok && sr.X+sr.W > float32(dc.w)-dc.right+1 {
				t.Fatalf("[%s] top-right search button overflows right SafeArea: %+v", dc.name, sr)
			}

			tabR, hasTabBtn := tt.Find("标签")
			if dc.expectDockHidden {
				if hasTabBtn {
					t.Fatalf("[%s] expected floating nav dock hidden when software keyboard inset=%.0f", dc.name, dc.bottom)
				}
			} else {
				if !hasTabBtn {
					t.Fatalf("[%s] missing bottom nav dock", dc.name)
				}
				dockBottom := tabR.Y + tabR.H
				if dockBottom < dc.expectDockMinBottom || dockBottom > dc.expectDockMaxBottom {
					t.Fatalf("[%s] bottom dock bottom=%.1f out of range [%.1f, %.1f]",
						dc.name, dockBottom, dc.expectDockMinBottom, dc.expectDockMaxBottom)
				}
			}

			assertBoundsWithinWidth(t, tt, dc.name+"_Calendar", float32(dc.w)-dc.right)

			if dc.openPantryToo {
				sh.OpenPantryTab()
				settleIPhoneFrames(tt, 180)
				if dc.snapshot != "" {
					saveAuditSnapshot(t, tt, dc.snapshot)
				}
				titleR, ok := tt.Find("储藏室")
				if !ok {
					t.Fatalf("[%s] expected 储藏室 header", dc.name)
				}
				if rectsOverlap(titleR, orbR) {
					t.Fatalf("[%s] top overlap 储藏室 %+v vs orb %+v", dc.name, titleR, orbR)
				}
			assertBoundsWithinWidth(t, tt, dc.name+"_Pantry", float32(dc.w)-dc.right)
			} else if dc.snapshot != "" {
				saveAuditSnapshot(t, tt, dc.snapshot)
			}
		})
	}
}

func TestMobileNativeGesturesScrimAndFootnoteUX402x874(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "NativeUXVault")
	a, sh := newWiredFSMTestApp(t, vaultRoot)
	_ = a
	sh.ForceMobilePreview = true
	sh.OpenCalendarTab()

	darkMode := false
	tt := ui.NewTester(func(c *ui.Context) {
		if darkMode {
			c.SetTheme(theme.DarkTheme())
		} else {
			c.SetTheme(theme.LightTheme())
		}
		sh.RenderSplitView(c)
	}, 402, 874)
	tt.SetInsets(59, 0, 34, 0)
	settleIPhoneFrames(tt, 300)

	// 1. Full-canvas symmetric gestures from center of screen (not restricted to 18dp edges!)
	swipeHorizontal(tt, 100, 320, 460)
	settleIPhoneFrames(tt, 260)
	if sh.MobileDrawer != "tree" {
		t.Fatalf("expected full-canvas right swipe from x=100->320 to open Left Drawer, got %q", sh.MobileDrawer)
	}

	// Verify Dark-mode drawer scrim is pure black translucent (NOT white tok.Text haze!)
	darkMode = true
	settleIPhoneFrames(tt, 180)
	saveAuditSnapshot(t, tt, "mobile_dark_drawer_scrim.png")
	if img := tt.Image(); img != nil {
		b := img.Bounds()
		sx := b.Min.X + (b.Dx()*370)/402
		sy := b.Min.Y + (b.Dy()*420)/874
		r, g, bl, _ := img.At(sx, sy).RGBA()
		r8, g8, b8 := r>>8, g>>8, bl>>8
		if r8 > 45 || g8 > 45 || b8 > 45 {
			t.Fatalf("expected dark black scrim at (370,420), got RGB=(%d,%d,%d)", r8, g8, b8)
		}
	}

	// Swipe left on the backdrop scrim closes Left Drawer cleanly
	swipeHorizontal(tt, 370, 120, 460)
	settleIPhoneFrames(tt, 260)
	if sh.MobileDrawer != "" {
		t.Fatalf("expected scrim left swipe to close Left Drawer, got %q", sh.MobileDrawer)
	}

	// Full-canvas left swipe from middle opens Right Drawer, scrim/drawer right swipe closes it
	swipeHorizontal(tt, 290, 70, 460)
	settleIPhoneFrames(tt, 260)
	if sh.MobileDrawer != "outline" {
		t.Fatalf("expected full-canvas left swipe from x=290->70 to open Right Drawer, got %q", sh.MobileDrawer)
	}
	if tt.HasText("切换实验库") {
		t.Fatalf("expected redundant 切换实验库 button removed from Right Drawer header")
	}
	swipeHorizontal(tt, 40, 280, 460)
	settleIPhoneFrames(tt, 260)
	if sh.MobileDrawer != "" {
		t.Fatalf("expected right swipe to close Right Drawer, got %q", sh.MobileDrawer)
	}

	// 2. Rightmost ≡ button opens Obsidian-style Ribbon Quick Options Bottom Sheet (not Left Drawer)
	if err := tt.Click("功能菜单"); err != nil {
		t.Fatalf("click 功能菜单: %v", err)
	}
	settleIPhoneFrames(tt, 320)
	saveAuditSnapshot(t, tt, "mobile_ribbon_menu_sheet_dark.png")
	if !sh.MobileMenuSheetOpen || !tt.HasText("打开快速切换") || !tt.HasText("储藏室与动物房") {
		t.Fatalf("expected Ribbon Menu Bottom Sheet open, open=%v texts=%v", sh.MobileMenuSheetOpen, tt.Texts())
	}
	assertBoundsWithinWidth(t, tt, "RibbonMenuSheetDark", 402)
	sh.MobileMenuSheetOpen = false
	settleIPhoneFrames(tt, 240)

	// 2b. Verify Obsidian Edit Toolbar opens ONLY when keyboard inset is active & icons are uniform neutral (no scrollbar/icon peek)
	paneID := ""
	if p := sh.ActivePane(); p != nil {
		paneID = p.ID
	}
	sh.OpenFileInPane(paneID, "实验模板/空白实验.asterml", "空白实验", shell.ModeEdit, false)
	sh.MobileDockForceNav = false
	tt.SetInsets(59, 0, 336, 0)
	settleIPhoneFrames(tt, 260)
	saveAuditSnapshot(t, tt, "mobile_edit_toolbar_keyboard_336.png")
	if !tt.HasText("+步骤") {
		t.Fatalf("expected Mobile Edit Toolbar visible when ModeEdit keyboard is open")
	}
	sh.OpenCalendarTab()
	tt.SetInsets(59, 0, 34, 0)
	settleIPhoneFrames(tt, 220)

	// 3. Full-screen Settings two-level swipe-back closure
	darkMode = false
	sh.VaultSettingsOpen = true
	settleIPhoneFrames(tt, 200)
	if err := tt.Click("文件与库"); err != nil {
		t.Fatalf("enter Settings Level 2 文件与库: %v", err)
	}
	settleIPhoneFrames(tt, 200)
	if !tt.HasText("当前库路径") {
		t.Fatalf("expected Level 2 文件与库 detail")
	}
	swipeHorizontal(tt, 40, 320, 420)
	settleIPhoneFrames(tt, 260)
	if tt.HasText("当前库路径") || !tt.HasText("工作区") || !tt.HasText("多端同步") {
		t.Fatalf("expected right swipe-back on Settings Level 2 to pop to Level 1, texts=%v", tt.Texts())
	}
	swipeHorizontal(tt, 40, 320, 420)
	settleIPhoneFrames(tt, 260)
	if sh.VaultSettingsOpen {
		t.Fatalf("expected second right swipe-back to close Settings")
	}

	// 4. Mobile inline Tufte footnote accordion attached directly beneath paragraph 1 (above paragraph 2) + keyboard-docked composer check
	fnSrc := "---\ntitle: 批注测试实验\ndate: 2026-11-26\n---\n\n#@ step-1 细胞润洗\n第一段：按 1:10 比例配制 PBS 缓冲液，沿孔壁缓慢加液[^1]，避免直接冲击细胞层。\n\n第二段跟随文字：静置 2 分钟后观察贴壁完整度。\n\n[^1]: 建议先 37°C 预热 15 min，冷液直冲容易造成贴壁细胞整片起皱。\n"
	fnDoc, err := asterdoc.Parse(fnSrc)
	if err != nil {
		t.Fatalf("parse footnote sample: %v", err)
	}
	var fnView *asterdoc.View
	fnView = &asterdoc.View{
		Doc:           fnDoc,
		Mode:          asterdoc.ModeDesign,
		Allowed:       map[string]bool{asterdoc.PermDocumentEdit: true},
		UnifiedLive:   true,
		MobileCompact: true,
		OnChange: func(next string) {
			if p, pErr := asterdoc.Parse(next); pErr == nil {
				fnView.Doc = p
			}
		},
	}
	fnTester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		ui.Scroll(c.Key("fn-ux-scroll")).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(float32(theme.LightTokens().Spacing16)).Children(func() {
				fnView.Render(c)
			})
		})
	}, 402, 874)
	fnTester.SetInsets(59, 0, 34, 0)
	settleIPhoneFrames(fnTester, 160)

	fnView.ToggleMobileInlineFootnote("1")
	settleIPhoneFrames(fnTester, 360)
	saveAuditSnapshot(t, fnTester, "mobile_footnote_inline_expanded.png")
	if !fnTester.HasText("建议先 37°C 预热 15 min") || !fnTester.HasText("编辑") || !fnTester.HasText("收起") {
		t.Fatalf("expected inline Tufte footnote expanded, texts=%v", fnTester.Texts())
	}
	if fnR, ok1 := fnTester.Find("编辑批注"); ok1 {
		if p2R, ok2 := fnTester.Find("第二段跟随文字：静置 2 分钟后观察贴壁完整度。"); ok2 {
			if fnR.Y >= p2R.Y {
				t.Fatalf("expected footnote accordion below paragraph 1 (above paragraph 2), fnR.Y=%v >= p2R.Y=%v", fnR.Y, p2R.Y)
			}
		}
	}
	assertBoundsWithinWidth(t, fnTester, "InlineFootnoteExpanded", 402)

	if err := fnTester.Click("编辑批注"); err != nil {
		t.Fatalf("click 编辑批注: %v", err)
	}
	fnTester.SetInsets(59, 0, 336, 0)
	settleIPhoneFrames(fnTester, 240)
	saveAuditSnapshot(t, fnTester, "mobile_footnote_composer_keyboard_336.png")
	saveR, ok := fnTester.Find("保存批注")
	if !ok {
		t.Fatalf("expected 保存批注 in composer, texts=%v", fnTester.Texts())
	}
	if saveR.Y < 59 || saveR.Y+saveR.H > 874-336 {
		t.Fatalf("composer header overlaps SafeArea or 336dp keyboard: %+v", saveR)
	}
	assertBoundsWithinWidth(t, fnTester, "FootnoteComposerKeyboard", 402)
}

func TestPhotoRouteOptionsAndSafeWikiEmbed(t *testing.T) {
	src := "---\ntitle: 拍照路由测试\n---\n\n#@ step-1 细胞成像\n观察贴壁状态。\n\n```asterlab\nkind: figure\nid: fig-slot-1\nschema: 1\ntitle: 显微镜 10x 视野\nsvg: \"<svg viewBox='0 0 360 160'></svg>\"\n```\n\n```asterlab\nkind: group_table\nid: gt-1\nschema: 1\nrows:\n  - {id: ctrl, name: 对照组, role: control}\n```\n\n```asterlab\nkind: plate\nid: plate-1\nschema: 1\ntitle: 96孔板A\nformat: 96\ngroups:\n  ctrl: [A1, A2]\n```\n"
	doc, err := asterdoc.Parse(src)
	if err != nil {
		t.Fatalf("parse doc: %v", err)
	}
	opts := collectPhotoRouteOptions(doc)
	if len(opts) < 3 {
		t.Fatalf("expected slot + plate + inline options, got %+v", opts)
	}
	if opts[0].Kind != "figure_slot" || opts[0].ModuleID != "fig-slot-1" {
		t.Fatalf("expected first option to be empty figure slot, got %+v", opts[0])
	}

	safe := sanitizeWikiAssetPath(`assets\photo[1]|test.png`)
	if safe != "./assets/photo1test.png" {
		t.Fatalf("unexpected sanitized asset path %q", safe)
	}

	// Verify inline ![[...]] insertion & figure_slot backfill both produce valid ASTs.
	resInline := executePhotoRouteChoice(src, "edit", 96, 10, safe, photoRouteOption{Kind: "inline_embed"})
	if !resInline.OK {
		t.Fatalf("inline embed failed: %+v", resInline.Error)
	}
	if _, err := asterdoc.Parse(resInline.NextSource); err != nil {
		t.Fatalf("inline embed produced invalid AsterML: %v\n%s", err, resInline.NextSource)
	}
	resSlot := executePhotoRouteChoice(src, "edit", 96, 10, safe, opts[0])
	if !resSlot.OK {
		t.Fatalf("slot update failed: %+v", resSlot.Error)
	}
	if _, err := asterdoc.Parse(resSlot.NextSource); err != nil {
		t.Fatalf("slot update produced invalid AsterML: %v\n%s", err, resSlot.NextSource)
	}
}
