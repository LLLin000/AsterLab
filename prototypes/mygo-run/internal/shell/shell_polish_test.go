package shell

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/uikit/kit"
)

func exportPolishShot(t *testing.T, dir, name string, img image.Image) {
	t.Helper()
	if dir == "" || img == nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir shot dir: %v", err)
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("create shot %s: %v", name, err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatalf("encode shot %s: %v", name, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close shot %s: %v", name, err)
	}
}

func TestShellPolishModalsAndQuickPalette(t *testing.T) {
	s := makeSampleV9Shell(true)
	s.ForceMobilePreview = false
	s.RecentVaults = []RecentVaultEntry{
		{Name: "AsterLabVault", Path: `D:\Lab\AsterLabVault`, Active: true},
		{Name: "ProjectB", Path: `D:\Lab\ProjectB`, Active: false},
	}

	tt := ui.NewTesterWith(func(c *ui.Context) {
		s.RenderSplitView(c)
	}, 1280, 820, ui.TesterOptions{Dark: true})
	tt.Frame()

	// 1. Command Palette
	s.PaletteOpen = true
	tt.Frame()
	advanceMobileFrames(tt, 140*time.Millisecond)
	if !tt.HasText("新建实验") {
		t.Fatalf("expected CommandPalette entries, got %v", tt.Texts())
	}
	s.PaletteOpen = false
	advanceMobileFrames(tt, 140*time.Millisecond)

	// 2. Global Search Modal
	s.GlobalSearchOpen = true
	s.GlobalSearchQuery = "HUVEC"
	tt.Frame()
	advanceMobileFrames(tt, 140*time.Millisecond)
	if !tt.HasText("HUVEC 血管新生") {
		t.Fatalf("expected GlobalSearch hit, got %v", tt.Texts())
	}
	s.GlobalSearchOpen = false
	advanceMobileFrames(tt, 140*time.Millisecond)

	// 3. Feedback & Welcome Modals
	s.FeedbackOpen = true
	tt.Frame()
	advanceMobileFrames(tt, 220*time.Millisecond)
	if !tt.HasText("反馈与诊断中心") {
		t.Fatalf("expected Feedback dialog, got %v", tt.Texts())
	}
	s.FeedbackOpen = false
	advanceMobileFrames(tt, 160*time.Millisecond)

	s.WelcomeOpen = true
	tt.Frame()
	advanceMobileFrames(tt, 220*time.Millisecond)
	if !tt.HasText("欢迎使用 AsterLab") {
		t.Fatalf("expected Welcome dialog, got %v", tt.Texts())
	}
	s.WelcomeOpen = false
	advanceMobileFrames(tt, 160*time.Millisecond)

	// 4. Settings Modal
	s.VaultSettingsOpen = true
	tt.Frame()
	advanceMobileFrames(tt, 220*time.Millisecond)
	if !tt.HasText("设置") {
		t.Fatalf("expected Settings dialog, got %v", tt.Texts())
	}
	s.VaultSettingsOpen = false
	advanceMobileFrames(tt, 160*time.Millisecond)
}

func TestShellPolishSheetBackdropAndDrawerDrag(t *testing.T) {
	s := makeSampleV9Shell(true)
	tt := ui.NewTesterWith(func(c *ui.Context) {
		s.RenderSplitView(c)
	}, 402, 874, ui.TesterOptions{Dark: true})
	tt.SetInsets(44, 0, 34, 0)
	tt.Frame()
	advanceMobileFrames(tt, 200*time.Millisecond)

	// Open Ribbon Sheet -> backdrop depth recedes cleanly, then dismiss.
	s.MobileMenuSheetOpen = true
	tt.Frame()
	advanceMobileFrames(tt, 380*time.Millisecond)
	if !tt.HasText("切换实验库") {
		t.Fatalf("expected ribbon sheet items, got %v", tt.Texts())
	}
	s.MobileMenuSheetOpen = false
	advanceMobileFrames(tt, 380*time.Millisecond)

	// Open Tab Switcher -> backdrop recedes cleanly, then dismiss.
	s.MobileTabSwitcherOpen = true
	tt.Frame()
	advanceMobileFrames(tt, 380*time.Millisecond)
	if !tt.HasText("标签页概览 (2)") {
		t.Fatalf("expected tab switcher overview, got %v", tt.Texts())
	}
	s.MobileTabSwitcherOpen = false
	advanceMobileFrames(tt, 380*time.Millisecond)

	// Verify drawer Draggable still works normally after sheets close.
	fingerDragSteps(tt, 24, 420, 290, 420, 10)
	advanceMobileFrames(tt, 380*time.Millisecond)
	if s.MobileDrawer != "tree" {
		t.Fatalf("expected edge drag to open left drawer after sheet dismissal, got %q", s.MobileDrawer)
	}
}

func TestShellPolishMenuButtonDesktopAndMobile(t *testing.T) {
	// Mobile: clicking 文档与更多操作 opens bottom sheet (with 取消 button).
	sMob := makeSampleV9Shell(true)
	ttMob := ui.NewTesterWith(func(c *ui.Context) {
		sMob.RenderSplitView(c)
	}, 402, 874, ui.TesterOptions{Dark: true})
	ttMob.SetInsets(44, 0, 34, 0)
	ttMob.Frame()
	advanceMobileFrames(ttMob, 200*time.Millisecond)

	if err := ttMob.Click("文档与更多操作"); err != nil {
		t.Fatalf("click mobile more doc menu: %v", err)
	}
	advanceMobileFrames(ttMob, 360*time.Millisecond)
	if !ttMob.HasText("步骤大纲与速算") || !ttMob.HasText("取消") {
		t.Fatalf("expected mobile doc menu bottom sheet with 取消, got %v", ttMob.Texts())
	}

	// Desktop: clicking 新手指南与界面导览 opens floating popover.
	sDesk := makeSampleV9Shell(false)
	sDesk.ForceMobilePreview = false
	ttDesk := ui.NewTesterWith(func(c *ui.Context) {
		sDesk.RenderSplitView(c)
	}, 1280, 820, ui.TesterOptions{Dark: false})
	ttDesk.Frame()
	if err := ttDesk.Click("新手指南与界面导览"); err != nil {
		t.Fatalf("click desktop guide menu button: %v", err)
	}
	advanceMobileFrames(ttDesk, 200*time.Millisecond)
	if !ttDesk.HasText("1 分钟界面区域导览") {
		t.Fatalf("expected desktop guide popover menu item, got %v", ttDesk.Texts())
	}
}

func TestShellPolishScreenshots(t *testing.T) {
	shotDir := os.Getenv("ASTERLAB_SHELL_POLISH_SHOT_DIR")
	if shotDir == "" {
		t.Skip("set ASTERLAB_SHELL_POLISH_SHOT_DIR to export verification screenshots")
	}

	// 1. Dark Ribbon Sheet with backdrop recession (402x874)
	{
		s := makeSampleV9Shell(true)
		s.MobileMenuSheetOpen = true
		tt := ui.NewTesterWith(func(c *ui.Context) {
			s.RenderSplitView(c)
		}, 402, 874, ui.TesterOptions{Dark: true})
		tt.SetInsets(44, 0, 34, 0)
		tt.Frame()
		advanceMobileFrames(tt, 420*time.Millisecond)
		exportPolishShot(t, shotDir, "dark_ribbon_sheet_402x874.png", tt.Image())
	}

	// 2. Light & Dark Right Underlay Switcher (402x874)
	for _, dark := range []bool{false, true} {
		s := makeSampleV9Shell(dark)
		s.MobileDrawer = "outline"
		tt := ui.NewTesterWith(func(c *ui.Context) {
			s.RenderSplitView(c)
		}, 402, 874, ui.TesterOptions{Dark: dark})
		tt.SetInsets(44, 0, 34, 0)
		tt.Frame()
		advanceMobileFrames(tt, 380*time.Millisecond)
		if err := tt.Click("切换右侧面板 步骤大纲"); err != nil {
			t.Fatalf("click right switcher pill (dark=%v): %v", dark, err)
		}
		advanceMobileFrames(tt, 240*time.Millisecond)
		name := "light_right_underlay_switcher_402x874.png"
		if dark {
			name = "dark_right_underlay_switcher_402x874.png"
		}
		exportPolishShot(t, shotDir, name, tt.Image())
	}

	// 3. Light & Dark Vault Launcher (402x874)
	for _, dark := range []bool{false, true} {
		s := makeSampleV9Shell(dark)
		s.VaultLauncherOpen = true
		s.RecentVaults = []RecentVaultEntry{
			{Name: "AsterLabVault", Path: "/Users/lab/AsterLabVault", Active: true},
			{Name: "TumorScreen", Path: "/Users/lab/TumorScreen", Active: false},
		}
		tt := ui.NewTesterWith(func(c *ui.Context) {
			s.RenderSplitView(c)
		}, 402, 874, ui.TesterOptions{Dark: dark})
		tt.SetInsets(44, 0, 34, 0)
		tt.Frame()
		advanceMobileFrames(tt, 260*time.Millisecond)
		name := "light_vault_launcher_402x874.png"
		if dark {
			name = "dark_vault_launcher_402x874.png"
		}
		exportPolishShot(t, shotDir, name, tt.Image())
	}

	// 4. Dark Tab Switcher Overview (402x874)
	{
		s := makeSampleV9Shell(true)
		s.OpenCalendarTab()
		s.OpenPantryTab()
		s.MobileTabSwitcherOpen = true
		tt := ui.NewTesterWith(func(c *ui.Context) {
			s.RenderSplitView(c)
		}, 402, 874, ui.TesterOptions{Dark: true})
		tt.SetInsets(44, 0, 34, 0)
		tt.Frame()
		advanceMobileFrames(tt, 400*time.Millisecond)
		exportPolishShot(t, shotDir, "dark_tab_switcher_402x874.png", tt.Image())
	}

	// 5. Desktop Dark Command Palette & Light Vault Launcher (1280x820)
	{
		s := makeSampleV9Shell(true)
		s.ForceMobilePreview = false
		s.PaletteOpen = true
		tt := ui.NewTesterWith(func(c *ui.Context) {
			s.RenderSplitView(c)
		}, 1280, 820, ui.TesterOptions{Dark: true})
		tt.Frame()
		advanceMobileFrames(tt, 200*time.Millisecond)
		exportPolishShot(t, shotDir, "desktop_command_palette_dark_1280x820.png", tt.Image())
	}
	{
		s := makeSampleV9Shell(false)
		s.ForceMobilePreview = false
		s.VaultLauncherOpen = true
		s.RecentVaults = []RecentVaultEntry{
			{Name: "AsterLabVault", Path: `D:\Lab\AsterLabVault`, Active: true},
			{Name: "TumorScreen", Path: `D:\Lab\TumorScreen`, Active: false},
		}
		tt := ui.NewTesterWith(func(c *ui.Context) {
			s.RenderSplitView(c)
		}, 1280, 820, ui.TesterOptions{Dark: false})
		tt.Frame()
		advanceMobileFrames(tt, 240*time.Millisecond)
		exportPolishShot(t, shotDir, "desktop_vault_launcher_light_1280x820.png", tt.Image())
	}
}

func TestShellRolloutCraftPrimitives(t *testing.T) {
	longTitle := "2026-11-27_HAMA-GelMA_CCK8_Plate03_FOV12_48h_raw.asterml"
	truncTitle := kit.MiddleTruncate(longTitle, 24)
	if truncTitle == longTitle || !strings.Contains(truncTitle, "…") || !strings.HasSuffix(truncTitle, ".asterml") {
		t.Fatalf("expected MiddleTruncate to preserve .asterml suffix around ellipsis, got %q", truncTitle)
	}

	s := makeSampleV9Shell(true)
	s.ForceMobilePreview = false
	s.RightOpen = true
	s.RightTab = 0
	s.OpenFileInPane(defaultPaneID, "2026/long-raw.asterml", longTitle, ModeEdit, true)

	tt := ui.NewTesterWith(func(c *ui.Context) {
		s.RenderSplitView(c)
	}, 1280, 820, ui.TesterOptions{Dark: true})
	tt.Frame()

	// 1. Desktop tab MiddleTruncate keeps full label accessible and short names untouched.
	if !tt.HasText(truncTitle) {
		t.Fatalf("expected middle-truncated desktop tab title %q, got %v", truncTitle, tt.Texts())
	}
	if _, ok := tt.Find("标签:" + longTitle); !ok {
		t.Fatalf("expected full accessible tab label %q", "标签:"+longTitle)
	}
	if !tt.HasText("HUVEC 血管新生") {
		t.Fatalf("expected short tab title HUVEC 血管新生 untouched, got %v", tt.Texts())
	}
	if !tt.HasText("3 项") {
		t.Fatalf("expected outline step count in right drawer, got %v", tt.Texts())
	}

	// 2. Right drawer calculator + timer NumericRoll readouts.
	if err := tt.Click("速算计时"); err != nil {
		t.Fatalf("switch right drawer to 速算计时: %v", err)
	}
	s.RightCalc.Stock = "100"
	s.RightCalc.Target = "50"
	s.RightCalc.FinalVol = "4"
	tt.Frame()
	if !tt.HasText("2.00 µL") || !tt.HasText("15:00") {
		t.Fatalf("expected live calc & timer readouts in right drawer, got %v", tt.Texts())
	}
	if err := tt.Click("30分"); err != nil {
		t.Fatalf("click 30分 timer preset: %v", err)
	}
	tt.Frame()
	if !tt.HasText("30:00") {
		t.Fatalf("expected timer countdown 30:00 after preset click, got %v", tt.Texts())
	}

	// 3. Command Palette EmptyStateCard + ShortcutKeycaps + 清空搜索 action.
	s.PaletteOpen = true
	s.PaletteQuery = "no-such-rollout-match"
	tt.Frame()
	advanceMobileFrames(tt, 140*time.Millisecond)
	if !tt.HasText("未找到匹配实验、试剂或命令") || !tt.HasText("清空搜索") {
		t.Fatalf("expected CommandPalette EmptyStateCard, got %v", tt.Texts())
	}
	if !tt.HasText("Ctrl") || !tt.HasText("K") {
		t.Fatalf("expected ShortcutKeycaps in CommandPalette EmptyStateCard, got %v", tt.Texts())
	}
	if err := tt.Click("清空搜索"); err != nil {
		t.Fatalf("click CommandPalette 清空搜索: %v", err)
	}
	tt.Frame()
	if s.PaletteQuery != "" || !tt.HasText("新建实验") {
		t.Fatalf("expected PaletteQuery cleared and entries restored, query=%q texts=%v", s.PaletteQuery, tt.Texts())
	}
	s.PaletteOpen = false
	advanceMobileFrames(tt, 120*time.Millisecond)

	// 4. Global Search Modal EmptyStateCard (idle & empty hits) + ShortcutKeycaps.
	s.GlobalSearchOpen = true
	s.GlobalSearchQuery = ""
	tt.Frame()
	advanceMobileFrames(tt, 140*time.Millisecond)
	if !tt.HasText("输入关键词跨 Vault 全文检索") || !tt.HasText("Shift") {
		t.Fatalf("expected GlobalSearch idle EmptyStateCard with shortcut keycaps, got %v", tt.Texts())
	}
	s.GlobalSearchQuery = "no-such-rollout-hit"
	tt.Frame()
	if !tt.HasText("未找到包含该关键词的实验文档") || !tt.HasText("清空搜索") || !tt.HasText("Esc") {
		t.Fatalf("expected GlobalSearch empty hits EmptyStateCard, got %v", tt.Texts())
	}
	if err := tt.Click("清空搜索"); err != nil {
		t.Fatalf("click GlobalSearch 清空搜索: %v", err)
	}
	tt.Frame()
	if s.GlobalSearchQuery != "" {
		t.Fatalf("expected GlobalSearchQuery cleared, got %q", s.GlobalSearchQuery)
	}
	s.GlobalSearchOpen = false
	advanceMobileFrames(tt, 120*time.Millisecond)

	// 5. Mobile Settings directional parallax sub-page push/pop + recessed sync provider track.
	sMob := makeSampleV9Shell(true)
	sMob.VaultSettingsOpen = true
	ttMob := ui.NewTesterWith(func(c *ui.Context) {
		sMob.RenderSplitView(c)
	}, 402, 874, ui.TesterOptions{Dark: true})
	ttMob.SetInsets(44, 0, 34, 0)
	ttMob.Frame()
	advanceMobileFrames(ttMob, 240*time.Millisecond)

	if err := ttMob.Click("多端同步"); err != nil {
		t.Fatalf("open mobile 多端同步 sub-page: %v", err)
	}
	advanceMobileFrames(ttMob, 360*time.Millisecond)
	if !sMob.mobileExtra().mobileSettingsDetail || !ttMob.HasText("OneDrive") {
		t.Fatalf("expected mobile sync sub-page open with OneDrive pill, texts=%v", ttMob.Texts())
	}
	if err := ttMob.Click("返回设置"); err != nil {
		t.Fatalf("return from mobile settings sub-page: %v", err)
	}
	advanceMobileFrames(ttMob, 360*time.Millisecond)
	if sMob.mobileExtra().mobileSettingsDetail || !ttMob.HasText("工作区") {
		t.Fatalf("expected parent settings home restored after back, texts=%v", ttMob.Texts())
	}
}

func TestShellRolloutScreenshots(t *testing.T) {
	shotDir := os.Getenv("ASTERLAB_SHELL_ROLLOUT_SHOT_DIR")
	if shotDir == "" {
		t.Skip("set ASTERLAB_SHELL_ROLLOUT_SHOT_DIR to export verification screenshots")
	}

	// 1. Desktop Dark (1280x820): edge-faded tabs + MiddleTruncate, Command Palette recessed keycaps,
	//    and Right Drawer with recessed segmented track & NumericRoll calculator/timer readouts.
	{
		s := makeSampleV9Shell(true)
		s.ForceMobilePreview = false
		s.RightOpen = true
		s.RightTab = 1
		s.RightCalc.Stock = "100"
		s.RightCalc.Target = "25"
		s.RightCalc.FinalVol = "4"
		s.BenchTimer.PresetMin = 30
		s.BenchTimer.TotalSec = 30 * 60
		s.BenchTimer.RemainingSec = 30 * 60
		s.BenchTimer.Initialized = true
		s.OpenPantryTab()
		s.OpenFileInPane(defaultPaneID, "2026/hama-raw.asterml", "2026-11-27_HAMA-GelMA_CCK8_Plate03_FOV12_48h_raw.asterml", ModeEdit, true)
		s.OpenFileInPane(defaultPaneID, "2026/qpcr.asterml", "VEGF_qPCR_2026-11-27_Run02_Ct_Normalized.asterml", ModeLab, true)
		s.PaletteOpen = true

		tt := ui.NewTesterWith(func(c *ui.Context) {
			s.RenderSplitView(c)
		}, 1280, 820, ui.TesterOptions{Dark: true})
		tt.Frame()
		advanceMobileFrames(tt, 220*time.Millisecond)
		exportPolishShot(t, shotDir, "rollout_shell_desktop_dark_1280x820.png", tt.Image())
	}

	// 2. Mobile Settings (402x874): directional parallax parent recede + pushed sub-page with recessed controls.
	{
		s := makeSampleV9Shell(true)
		s.VaultSettingsOpen = true
		tt := ui.NewTesterWith(func(c *ui.Context) {
			s.RenderSplitView(c)
		}, 402, 874, ui.TesterOptions{Dark: true})
		tt.SetInsets(44, 0, 34, 0)
		tt.Frame()
		advanceMobileFrames(tt, 240*time.Millisecond)
		if err := tt.Click("多端同步"); err != nil {
			t.Fatalf("click 多端同步 for rollout shot: %v", err)
		}
		advanceMobileFrames(tt, 112*time.Millisecond)
		exportPolishShot(t, shotDir, "rollout_shell_mobile_settings_402x874.png", tt.Image())
	}
}
