package shell

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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
