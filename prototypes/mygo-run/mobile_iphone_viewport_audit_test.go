package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
	"asterlab-mygo-prototype/modules/runs"
)

func maybeSaveIPhoneSnapshot(t *testing.T, tt *ui.Tester, filename string) {
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
		t.Fatalf("nil tester image for %s", filename)
	}
	outPath := filepath.Join(dir, filename)
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create snapshot %s: %v", outPath, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode snapshot %s: %v", outPath, err)
	}
}

func settleIPhoneFrames(tt *ui.Tester, ms int) {
	steps := ms / 16
	if steps < 1 {
		steps = 1
	}
	for range steps {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
}

func rectsOverlap(a, b ui.Rect) bool {
	return a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

func swipeHorizontal(tt *ui.Tester, x1, x2, y float32) {
	tt.FingerDown(x1, y)
	tt.Advance(16 * time.Millisecond)
	const steps = 12
	for i := 1; i <= steps; i++ {
		t := float32(i) / float32(steps)
		x := x1 + (x2-x1)*t
		tt.FingerMove(x, y)
		tt.Frame()
		tt.Advance(16 * time.Millisecond)
	}
	tt.FingerUp(x2, y)
	settleIPhoneFrames(tt, 420)
}

func TestIPhonePantryTopClearanceAndAddReagent402x874(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "IPhonePantryVault")
	a, sh := newWiredFSMTestApp(t, vaultRoot)
	sh.ForceMobilePreview = true
	sh.OpenPantryTab()

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		sh.RenderSplitView(c)
	}, 402, 874)
	tt.SetInsets(47, 0, 34, 0)
	settleIPhoneFrames(tt, 240)

	titleR, ok := tt.Find("储藏室")
	if !ok || titleR.W < 36 {
		t.Fatalf("expected untruncated 储藏室 title, ok=%v rect=%+v", ok, titleR)
	}
	vivR, ok := tt.Find("动物房")
	if !ok {
		t.Fatalf("expected 动物房 tab on mobile pantry, texts=%v", tt.Texts())
	}
	leftOrbR, ok := tt.Find("打开实验库抽屉")
	if !ok {
		t.Fatalf("expected floating left drawer orb")
	}
	calcR, ok := tt.Find("速算与计时器")
	if !ok {
		t.Fatalf("expected floating calc button")
	}
	searchR, ok := tt.Find("快速打开 (Ctrl+K)")
	if !ok {
		t.Fatalf("expected floating command palette button")
	}

	for _, pair := range []struct {
		name string
		a, b ui.Rect
	}{
		{"储藏室 vs 打开实验库抽屉", titleR, leftOrbR},
		{"动物房 vs 速算与计时器", vivR, calcR},
		{"动物房 vs 快速打开", vivR, searchR},
	} {
		if rectsOverlap(pair.a, pair.b) {
			t.Fatalf("P0 top overlap %s: %+v vs %+v", pair.name, pair.a, pair.b)
		}
	}

	if err := tt.Click("+ 新试剂"); err != nil {
		t.Fatalf("click top + 新试剂 button failed (pull zone may be blocking): %v", err)
	}
	settleIPhoneFrames(tt, 320)
	if !a.tools.PantryDialogOpen {
		t.Fatalf("expected clicking + 新试剂 to open PantryDialogOpen on 402x874")
	}
}

func TestIPhoneDrawerSwipeOpenCloseAndRightPanelNoOverlap402x874(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "IPhoneDrawerVault")
	a, sh := newWiredFSMTestApp(t, vaultRoot)
	_ = a
	sh.ForceMobilePreview = true
	sh.OpenCalendarTab()

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		sh.RenderSplitView(c)
	}, 402, 874)
	tt.SetInsets(47, 0, 34, 0)
	settleIPhoneFrames(tt, 240)

	// 1. Open Left Drawer via top orb -> verify open -> swipe left to close!
	if err := tt.Click("打开实验库抽屉"); err != nil {
		t.Fatalf("click left orb: %v", err)
	}
	settleIPhoneFrames(tt, 320)
	if sh.MobileDrawer != "tree" {
		t.Fatalf("expected MobileDrawer=tree, got %q", sh.MobileDrawer)
	}
	if !tt.HasText("实验库") || !tt.HasText("实验列表") {
		t.Fatalf("expected Left Drawer content, texts=%v", tt.Texts())
	}

	swipeHorizontal(tt, 250, 20, 420)
	if sh.MobileDrawer != "" {
		t.Fatalf("expected swiping left inside Left Drawer to close drawer, still got %q", sh.MobileDrawer)
	}

	// 2. Swipe left from right edge to open Right Drawer -> switch to 速算计时 -> check zero overlap -> swipe right to close!
	swipeHorizontal(tt, 396, 120, 420)
	if sh.MobileDrawer != "outline" {
		t.Fatalf("expected right-edge swipe left to open Right Drawer, got %q", sh.MobileDrawer)
	}
	if tt.HasText("实验列表") || tt.HasText("新建文件夹") {
		t.Fatalf("Right Drawer must not render Left Drawer bottom island, texts=%v", tt.Texts())
	}
	if err := tt.Click("速算计时"); err != nil {
		t.Fatalf("switch Right Drawer tab to 速算计时: %v", err)
	}
	settleIPhoneFrames(tt, 120)
	startR, ok := tt.Find("开始")
	if !ok {
		t.Fatalf("expected 快捷倒计时 开始 button in Right Drawer, texts=%v", tt.Texts())
	}
	if startR.Y+startR.H > 840 {
		t.Fatalf("expected 开始 button clear above bottom safeArea (<=840), got %+v", startR)
	}

	swipeHorizontal(tt, 140, 388, 300)
	if sh.MobileDrawer != "" {
		t.Fatalf("expected swiping right inside Right Drawer to close drawer, still got %q", sh.MobileDrawer)
	}
}

func TestIPhoneMobileSheetsWidthAndLabels402x874(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "IPhoneSheetsVault")
	a, sh := newWiredFSMTestApp(t, vaultRoot)
	sh.ForceMobilePreview = true
	a.tools.Callbacks.ListPantry = func() (runs.PantryLedger, error) {
		return runs.PantryLedger{}, nil
	}

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		sh.RenderSplitView(c)
	}, 402, 874)
	tt.SetInsets(47, 0, 34, 0)
	settleIPhoneFrames(tt, 160)

	// 1. Vault Launcher full-screen page
	sh.VaultLauncherOpen = true
	settleIPhoneFrames(tt, 120)
	maybeSaveIPhoneSnapshot(t, tt, "mobile_402x874_vault_launcher.png")
	for _, lbl := range []string{"选择你的实验库目录", "新建本地实验库", "创建并打开"} {
		r, ok := tt.Find(lbl)
		if !ok {
			t.Fatalf("Vault Launcher mobile page missing %q, texts=%v", lbl, tt.Texts())
		}
		if r.W < 40 || r.X+r.W > 403 {
			t.Fatalf("Vault Launcher %q geometry invalid on 402dp: %+v", lbl, r)
		}
	}
	sh.VaultLauncherOpen = false
	settleIPhoneFrames(tt, 120)

	// 2. Command Palette full-screen page
	sh.PaletteOpen = true
	settleIPhoneFrames(tt, 120)
	maybeSaveIPhoneSnapshot(t, tt, "mobile_402x874_command_palette.png")
	if tt.HasText("Ctrl+Enter") {
		t.Fatalf("Command Palette on mobile should omit desktop Ctrl+Enter footer hints")
	}
	scopeR, ok := tt.Find("文件与储藏室")
	if !ok || scopeR.W < 50 || scopeR.X+scopeR.W > 403 {
		t.Fatalf("Command Palette mobile scope chip invalid: ok=%v rect=%+v", ok, scopeR)
	}
	sh.PaletteOpen = false
	settleIPhoneFrames(tt, 120)

	// 3. Obsidian 2-level full-screen Settings Page (Level 1 Home -> Level 2 Appearance -> Back -> Files)
	sh.VaultSettingsOpen = true
	sh.SettingsView.Category = ""
	settleIPhoneFrames(tt, 120)
	maybeSaveIPhoneSnapshot(t, tt, "mobile_402x874_settings_level1.png")
	for _, lbl := range []string{"工作区", "文件与库", "多端同步", "外观", "系统", "关于"} {
		r, ok := tt.Find(lbl)
		if !ok || r.W < 24 || r.X+r.W > 403 {
			t.Fatalf("Mobile Obsidian settings Level 1 missing or invalid %q: ok=%v rect=%+v texts=%v", lbl, ok, r, tt.Texts())
		}
	}
	if err := tt.Click("外观"); err != nil {
		t.Fatalf("enter Appearance detail page from Settings Level 1: %v", err)
	}
	settleIPhoneFrames(tt, 120)
	if !tt.HasText("返回设置") {
		t.Fatalf("expected back navigation 返回设置 on Level 2 Appearance page, texts=%v", tt.Texts())
	}
	for i := 0; i < 8; i++ {
		if r, ok := tt.Find("标题 H1–H3"); ok && r.Y >= 120 && r.Y+r.H <= 840 {
			break
		}
		tt.Scroll(200, 480, 0, 260)
		tt.Frame()
	}
	maybeSaveIPhoneSnapshot(t, tt, "mobile_402x874_settings_level2_appearance.png")
	for _, lbl := range []string{"标题 H1–H3", "待办清单"} {
		r, ok := tt.Find(lbl)
		if !ok {
			t.Fatalf("Mobile settings missing toolbar item %q, texts=%v", lbl, tt.Texts())
		}
		if r.W < 40 {
			t.Fatalf("Mobile settings toolbar label %q crushed: %+v", lbl, r)
		}
	}
	if err := tt.Click("返回设置"); err != nil {
		t.Fatalf("navigate back from Appearance to Settings Level 1: %v", err)
	}
	settleIPhoneFrames(tt, 120)
	if err := tt.Click("文件与库"); err != nil {
		t.Fatalf("switch to 文件与库 category in Obsidian mobile settings: %v", err)
	}
	settleIPhoneFrames(tt, 120)
	if !tt.HasText("当前库路径") {
		t.Fatalf("expected 当前库路径 in mobile 文件与库 settings, texts=%v", tt.Texts())
	}
	sh.VaultSettingsOpen = false
	settleIPhoneFrames(tt, 120)

	// 4. Pantry New Box & New Reagent full-screen pages
	sh.OpenPantryTab()
	settleIPhoneFrames(tt, 120)
	a.tools.OpenNewBoxDialog("-80")
	settleIPhoneFrames(tt, 120)
	for _, lbl := range []string{"架层补充位置", "保存收纳盒"} {
		r, ok := tt.Find(lbl)
		if !ok || r.W < 40 || r.X+r.W > 403 {
			t.Fatalf("Mobile New Box page %q invalid: ok=%v rect=%+v", lbl, ok, r)
		}
	}
	a.tools.PantryBoxDialogOpen = false
	settleIPhoneFrames(tt, 120)

	a.tools.OpenNewPantryDialog("-80°C")
	settleIPhoneFrames(tt, 120)
	maybeSaveIPhoneSnapshot(t, tt, "mobile_402x874_pantry_new_reagent.png")
	for _, lbl := range []string{"条目类型", "试剂 / 耗材", "生物样本", "保存试剂档案"} {
		r, ok := tt.Find(lbl)
		if !ok || r.W < 36 || r.X+r.W > 403 {
			t.Fatalf("Mobile New Reagent page %q invalid: ok=%v rect=%+v", lbl, ok, r)
		}
	}
	a.tools.PantryDialogOpen = false
	settleIPhoneFrames(tt, 120)

	// 5. New Experiment full-screen page
	a.openNewExperimentDialog("", "2026-11-26")
	settleIPhoneFrames(tt, 120)
	for _, lbl := range []string{"新建实验", "实验名称", "创建实验"} {
		r, ok := tt.Find(lbl)
		if !ok || r.W < 36 || r.X+r.W > 403 {
			t.Fatalf("Mobile New Experiment page %q invalid: ok=%v rect=%+v", lbl, ok, r)
		}
	}
}

func TestIPhoneUIGalleryMobileSection402x874(t *testing.T) {
	sh := &shell.Shell{TodayDate: "2026-11-26"}
	for _, tc := range []struct {
		name          string
		w, h          int
		dark          bool
		mobilePreview bool
		insetTop      float32
		insetBottom   float32
		snapshot      string
	}{
		{"desktop_full_light", 1280, 860, false, false, 0, 0, "gallery_desktop_mobile_section_light.png"},
		{"desktop_402dp_preview_light", 1280, 860, false, true, 0, 0, "gallery_desktop_402dp_preview_light.png"},
		{"iphone_402x874_light", 402, 874, false, true, 47, 34, "gallery_iphone_402x874_light.png"},
		{"iphone_402x874_dark", 402, 874, true, true, 47, 34, "gallery_iphone_402x874_dark.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &kit.GalleryState{
				ActiveSection: "mobile-ios-design",
				MobilePreview: tc.mobilePreview,
			}
			tt := ui.NewTester(func(c *ui.Context) {
				if tc.dark {
					c.SetTheme(theme.DarkTheme())
				} else {
					c.SetTheme(theme.LightTheme())
				}
				kit.RenderComponentGallery(c, st, sh.BuiltinShellStorySection(), sh.BuiltinMobileStorySection())
			}, tc.w, tc.h)
			if tc.insetTop > 0 || tc.insetBottom > 0 {
				tt.SetInsets(tc.insetTop, 0, tc.insetBottom, 0)
			}
			settleIPhoneFrames(tt, 160)
			if !tt.HasText("8A. 402dp 移动工作台壳层、顶/底 SafeArea 避让与手势闭环") {
				t.Fatalf("missing mobile showcase section 8A heading, texts=%v", tt.Texts())
			}
			if !tt.HasText("8B. 全屏移动页面规范（Obsidian 两级设置页 & 全屏表单导航栏）") {
				t.Fatalf("missing mobile showcase 8B heading, texts=%v", tt.Texts())
			}
			maybeSaveIPhoneSnapshot(t, tt, tc.snapshot)
		})
	}
}
