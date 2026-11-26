package shell

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/vault"
)

func fingerDragSteps(tt *ui.Tester, x0, y0, x1, y1 float32, steps int) {
	if steps < 4 {
		steps = 4
	}
	tt.FingerDown(x0, y0)
	tt.Frame()
	for i := 1; i <= steps; i++ {
		f := float32(i) / float32(steps)
		x := x0 + (x1-x0)*f
		y := y0 + (y1-y0)*f
		tt.FingerMove(x, y)
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	tt.FingerUp(x1, y1)
	tt.Frame()
}

func makeSampleV9Shell(dark bool) *Shell {
	srcMode := "light"
	if dark {
		srcMode = "dark"
	}
	s := &Shell{
		TodayDate:          "2026-11-27",
		SelectedDate:       "2026-11-27",
		ForceMobilePreview: true,
		ThemeSource:        srcMode,
		VaultPathInput:     "/Users/lab/AsterLabVault",
		VaultFiles: []vault.FileEntry{
			{RelPath: "2026/huvec.asterml", Name: "huvec.asterml", Title: "HUVEC 血管新生", Date: "2026-11-27"},
			{RelPath: "2026/nested/qPCR-VEGF.asterml", Name: "qPCR-VEGF.asterml", Title: "VEGF qPCR 检测", Date: "2026-11-27"},
			{RelPath: "Templates/cck8.asterml", Name: "cck8.asterml", Title: "CCK-8 标准模板", IsTemplate: true},
		},
		VaultTree: &vault.TreeNode{
			IsDir: true,
			Children: []*vault.TreeNode{
				{
					RelPath: "2026",
					Name:    "2026",
					IsDir:   true,
					Children: []*vault.TreeNode{
						{
							RelPath: "2026/huvec.asterml",
							Name:    "huvec.asterml",
							Entry:   &vault.FileEntry{RelPath: "2026/huvec.asterml", Name: "huvec.asterml", Title: "HUVEC 血管新生", Date: "2026-11-27"},
						},
						{
							RelPath: "2026/nested",
							Name:    "nested",
							IsDir:   true,
							Children: []*vault.TreeNode{
								{
									RelPath: "2026/nested/qPCR-VEGF.asterml",
									Name:    "qPCR-VEGF.asterml",
									Entry:   &vault.FileEntry{RelPath: "2026/nested/qPCR-VEGF.asterml", Name: "qPCR-VEGF.asterml", Title: "VEGF qPCR 检测", Date: "2026-11-27"},
								},
							},
						},
					},
				},
				{
					RelPath:  "Templates",
					Name:     "Templates",
					IsDir:    true,
					IsSystem: true,
					Children: []*vault.TreeNode{
						{
							RelPath: "Templates/cck8.asterml",
							Name:    "cck8.asterml",
							Entry:   &vault.FileEntry{RelPath: "Templates/cck8.asterml", Name: "cck8.asterml", Title: "CCK-8 标准模板", IsTemplate: true},
						},
					},
				},
			},
		},
		OutlineSteps: []OutlineItem{
			{ID: "step-1", Title: "1. 准备 96 孔板与细胞悬液", IsStep: true, Number: "1", Meta: "3 模块"},
			{ID: "step-2", Title: "2. 加入 VEGF 处理与孵育", IsStep: true, Number: "2", Meta: "2 模块"},
			{ID: "step-3", Title: "3. CCK-8 显色与 450nm 读数", IsStep: true, Number: "3", Meta: "2 模块"},
		},
	}
	s.OpenFileInPane(defaultPaneID, "2026/huvec.asterml", "HUVEC 血管新生", ModeLab, false)
	return s
}

func TestMobileObsidianUnderlayDrawersAndRightBottomSwitcher402x874(t *testing.T) {
	s := makeSampleV9Shell(false)

	tt := ui.NewTesterWith(func(c *ui.Context) {
		s.RenderSplitView(c)
	}, 402, 874, ui.TesterOptions{Dark: false})
	tt.SetInsets(44, 0, 34, 0)
	tt.Frame()
	advanceMobileFrames(tt, 200*time.Millisecond)

	// 1. Open Left Drawer ("tree") -> main stage shifts right revealing mb-underlay-left underneath.
	s.MobileDrawer = "tree"
	tt.Frame()
	advanceMobileFrames(tt, 380*time.Millisecond)
	maybeExportVerifyPNG(t, "05_left_underlay_reveal_402x874.png", tt.Image())

	veilRect, hasVeil := tt.Find("关闭侧边栏")
	if !hasVeil {
		t.Fatalf("expected peeking main stage veil when left underlay is open, texts=%v", tt.Texts())
	}
	if veilRect.X < 250 {
		t.Fatalf("expected mb-main-stage shifted right (>=250px) to reveal left underlay, veilRect=%+v", veilRect)
	}
	if !tt.HasText("huvec.asterml") {
		t.Fatalf("expected file tree visible inside left underlay, texts=%v", tt.Texts())
	}

	// Swipe left from peeking right edge/stage strip (full-screen swipe-close).
	fingerDragSteps(tt, 380, 420, 120, 420, 10)
	advanceMobileFrames(tt, 380*time.Millisecond)
	if s.MobileDrawer != "" {
		t.Fatalf("expected swiping left from peeking strip to close left drawer, got %q", s.MobileDrawer)
	}

	// Re-open left drawer and verify tapping mb-stage-veil closes it too.
	s.MobileDrawer = "tree"
	tt.Frame()
	advanceMobileFrames(tt, 360*time.Millisecond)
	if err := tt.Click("关闭侧边栏"); err != nil {
		t.Fatalf("tap peeking stage veil to close left drawer: %v", err)
	}
	advanceMobileFrames(tt, 380*time.Millisecond)
	if s.MobileDrawer != "" {
		t.Fatalf("expected tapping stage veil to close left drawer, got %q", s.MobileDrawer)
	}

	// 2. Open Right Drawer ("outline") -> main stage shifts left revealing mb-underlay-right + bottom pill switcher.
	s.MobileDrawer = "outline"
	tt.Frame()
	advanceMobileFrames(tt, 380*time.Millisecond)
	veilR, hasRightVeil := tt.Find("关闭侧边栏")
	if !hasRightVeil {
		t.Fatalf("expected peeking left veil strip when right underlay open, texts=%v", tt.Texts())
	}
	if veilR.W > 110 || veilR.X != 0 {
		t.Fatalf("expected mb-main-stage shifted left so visible left peek strip is ~82px at X=0, veilR=%+v", veilR)
	}

	// Tap Obsidian-style bottom switcher pill ("切换右侧面板 步骤大纲") -> opens upward floating rounded card.
	if err := tt.Click("切换右侧面板 步骤大纲"); err != nil {
		t.Fatalf("click bottom right switcher pill: %v", err)
	}
	tt.Frame()
	advanceMobileFrames(tt, 180*time.Millisecond)
	for _, want := range []string{"步骤大纲", "速算与计时", "关联与诊断"} {
		if !tt.HasText(want) {
			t.Fatalf("expected upward switcher menu option %q, texts=%v", want, tt.Texts())
		}
	}
	maybeExportVerifyPNG(t, "06_right_underlay_switcher_402x874.png", tt.Image())

	if err := tt.Click("速算与计时"); err != nil {
		t.Fatalf("select 速算与计时 from upward switcher menu: %v", err)
	}
	advanceMobileFrames(tt, 160*time.Millisecond)
	if s.RightTab != 1 {
		t.Fatalf("expected RightTab=1 after selecting 速算与计时, got %d", s.RightTab)
	}
	if s.mobileExtra().rightSwitcherOpen {
		t.Fatalf("expected switcher menu to close after selecting tab")
	}

	// Full-screen swipe-close from inside open Right Underlay (middle area drag rightwards).
	fingerDragSteps(tt, 180, 320, 370, 320, 10)
	advanceMobileFrames(tt, 380*time.Millisecond)
	if s.MobileDrawer != "" {
		t.Fatalf("expected swiping right from inside right underlay to close drawer, got %q", s.MobileDrawer)
	}

	// Re-open Right Drawer and swipe right from peeking left workspace strip (X=30 -> 280).
	s.MobileDrawer = "outline"
	tt.Frame()
	advanceMobileFrames(tt, 360*time.Millisecond)
	fingerDragSteps(tt, 30, 420, 280, 420, 10)
	advanceMobileFrames(tt, 380*time.Millisecond)
	if s.MobileDrawer != "" {
		t.Fatalf("expected swiping right from peeking left strip to close right drawer, got %q", s.MobileDrawer)
	}
}

func TestCollapseAllExpandAllToggleMobileAndDesktop(t *testing.T) {
	// Mobile toggle: 折叠全部 <-> 展开全部
	sMob := makeSampleV9Shell(false)
	sMob.MobileDrawer = "tree"
	ttMob := ui.NewTesterWith(func(c *ui.Context) {
		sMob.RenderSplitView(c)
	}, 402, 874, ui.TesterOptions{})
	ttMob.SetInsets(44, 0, 34, 0)
	ttMob.Frame()
	advanceMobileFrames(ttMob, 340*time.Millisecond)

	if !sMob.anyVaultFolderOpen() {
		t.Fatalf("expected at least one vault folder open initially")
	}
	if err := ttMob.Click("折叠全部"); err != nil {
		t.Fatalf("click mobile 折叠全部: %v", err)
	}
	ttMob.Frame()
	if sMob.anyVaultFolderOpen() {
		t.Fatalf("expected all folders collapsed after first click")
	}
	if _, ok := ttMob.Find("展开全部"); !ok {
		t.Fatalf("expected mobile button label to flip to 展开全部 after collapse")
	}
	if err := ttMob.Click("展开全部"); err != nil {
		t.Fatalf("click mobile 展开全部: %v", err)
	}
	ttMob.Frame()
	if !sMob.anyVaultFolderOpen() {
		t.Fatalf("expected folders expanded after second click")
	}
	if _, ok := ttMob.Find("折叠全部"); !ok {
		t.Fatalf("expected mobile button label to flip back to 折叠全部 after expand")
	}

	// Desktop helper state toggle: 折叠全部文件夹 <-> 展开全部文件夹
	sDesk := makeSampleV9Shell(false)
	sDesk.ForceMobilePreview = false
	ttDesk := ui.NewTesterWith(func(c *ui.Context) {
		sDesk.RenderSplitView(c)
	}, 1180, 760, ui.TesterOptions{})
	ttDesk.Frame()
	if !sDesk.anyVaultFolderOpen() {
		t.Fatalf("expected desktop folder open initially")
	}
	if err := ttDesk.Click("折叠全部文件夹"); err != nil {
		t.Fatalf("click desktop 折叠全部文件夹: %v", err)
	}
	ttDesk.Frame()
	if sDesk.anyVaultFolderOpen() {
		t.Fatalf("expected desktop folders collapsed after click")
	}
	if _, ok := ttDesk.Find("展开全部文件夹"); !ok {
		t.Fatalf("expected desktop button label to flip to 展开全部文件夹")
	}
	if err := ttDesk.Click("展开全部文件夹"); err != nil {
		t.Fatalf("click desktop 展开全部文件夹: %v", err)
	}
	ttDesk.Frame()
	if !sDesk.anyVaultFolderOpen() {
		t.Fatalf("expected desktop folders expanded after second click")
	}
}

func TestMobileSettingsCleanupAndSyncProviderSwitcher(t *testing.T) {
	s := makeSampleV9Shell(false)
	s.VaultSettingsOpen = true

	tt := ui.NewTesterWith(func(c *ui.Context) {
		s.RenderSplitView(c)
	}, 402, 874, ui.TesterOptions{})
	tt.SetInsets(44, 0, 34, 0)
	tt.Frame()
	advanceMobileFrames(tt, 240*time.Millisecond)

	// 1. Settings Home omits desktop-only "快捷键" category and shows Vault hint ("AsterLabVault").
	if tt.HasText("快捷键") {
		t.Fatalf("expected 快捷键 category hidden on mobile Settings Home, texts=%v", tt.Texts())
	}
	if !tt.HasText("AsterLabVault") {
		t.Fatalf("expected vault folder hint AsterLabVault on Settings Home, texts=%v", tt.Texts())
	}

	// 2. Check Appearance category hides desktop shortcut hints ("Ctrl+=").
	if err := tt.Click("外观"); err != nil {
		t.Fatalf("open 外观 settings: %v", err)
	}
	advanceMobileFrames(tt, 180*time.Millisecond)
	for _, txt := range tt.Texts() {
		if strings.Contains(txt, "Ctrl+=") || strings.Contains(txt, "Ctrl+Shift+D") {
			t.Fatalf("desktop shortcut hint %q leaked into mobile Appearance settings", txt)
		}
	}
	if err := tt.Click("返回设置"); err != nil {
		t.Fatalf("back to settings home: %v", err)
	}
	advanceMobileFrames(tt, 180*time.Millisecond)

	// 3. Check File & Vault category hides desktop "打开目录" and shows mobile "切换 / 新建实验库".
	if err := tt.Click("文件与库"); err != nil {
		t.Fatalf("open 文件与库 settings: %v", err)
	}
	advanceMobileFrames(tt, 180*time.Millisecond)
	if tt.HasText("打开目录") {
		t.Fatalf("expected desktop 打开目录 hidden on mobile")
	}
	if !tt.HasText("切换 / 新建实验库") {
		t.Fatalf("expected mobile 切换 / 新建实验库 action, texts=%v", tt.Texts())
	}
	if err := tt.Click("返回设置"); err != nil {
		t.Fatalf("back from vault settings: %v", err)
	}
	advanceMobileFrames(tt, 180*time.Millisecond)

	// 4. Open Sync category -> switch WebDAV | OneDrive | Google Drive pills cleanly.
	if err := tt.Click("多端同步"); err != nil {
		t.Fatalf("open 多端同步 settings: %v", err)
	}
	advanceMobileFrames(tt, 200*time.Millisecond)
	for _, p := range []string{"WebDAV", "OneDrive", "Google Drive"} {
		if !tt.HasText(p) {
			t.Fatalf("missing sync provider pill %q, texts=%v", p, tt.Texts())
		}
	}
	if err := tt.Click("OneDrive"); err != nil {
		t.Fatalf("click OneDrive pill: %v", err)
	}
	advanceMobileFrames(tt, 160*time.Millisecond)
	if NormalizeSyncProvider(s.SyncSettings.Provider) != SyncProviderOneDrive {
		t.Fatalf("expected Provider=onedrive, got %q", s.SyncSettings.Provider)
	}
	maybeExportVerifyPNG(t, "07_mobile_settings_sync_providers_402x874.png", tt.Image())

	if err := tt.Click("Google Drive"); err != nil {
		t.Fatalf("click Google Drive pill: %v", err)
	}
	advanceMobileFrames(tt, 160*time.Millisecond)
	if NormalizeSyncProvider(s.SyncSettings.Provider) != SyncProviderGDrive {
		t.Fatalf("expected Provider=gdrive, got %q", s.SyncSettings.Provider)
	}
}

func TestMobileVaultLauncherErrorRetentionAndCloudSyncCTA(t *testing.T) {
	s := makeSampleV9Shell(false)
	s.VaultLauncherOpen = true
	s.VaultLauncherErr = ""
	s.RecentVaults = []RecentVaultEntry{
		{Name: "AsterLabVault", Path: "/Users/lab/AsterLabVault", Active: true},
		{Name: "ReadOnlyVault", Path: "/readonly/ReadOnlyVault", Active: false},
	}
	s.OnSwitchVaultPathErr = func(path string) error {
		if strings.Contains(path, "ReadOnlyVault") {
			return errors.New("实验库目录不可写：permission denied")
		}
		return nil
	}

	tt := ui.NewTesterWith(func(c *ui.Context) {
		s.RenderSplitView(c)
	}, 402, 874, ui.TesterOptions{})
	tt.SetInsets(44, 0, 34, 0)
	tt.Frame()
	advanceMobileFrames(tt, 220*time.Millisecond)

	if !tt.HasText("连接云端同步库") {
		t.Fatalf("expected 连接云端同步库 CTA in mobile Vault Launcher, texts=%v", tt.Texts())
	}

	// Attempt to open unwritable recent vault -> launcher MUST stay open and display error banner.
	if err := tt.Click("打开 ReadOnlyVault"); err != nil {
		t.Fatalf("click 打开 ReadOnlyVault: %v", err)
	}
	advanceMobileFrames(tt, 180*time.Millisecond)
	if !s.VaultLauncherOpen {
		t.Fatalf("expected VaultLauncherOpen to remain true on switch error")
	}
	if !strings.Contains(s.VaultLauncherErr, "实验库目录不可写") {
		t.Fatalf("expected VaultLauncherErr retained, got %q", s.VaultLauncherErr)
	}
	if !tt.HasText(s.VaultLauncherErr) {
		t.Fatalf("expected rendered error banner with %q, texts=%v", s.VaultLauncherErr, tt.Texts())
	}
	maybeExportVerifyPNG(t, "08_mobile_vault_launcher_402x874.png", tt.Image())

	// Clicking 连接云端同步库 navigates directly into Settings -> Sync.
	if err := tt.Click("连接云端同步库"); err != nil {
		t.Fatalf("click 连接云端同步库: %v", err)
	}
	advanceMobileFrames(tt, 220*time.Millisecond)
	if s.VaultLauncherOpen || !s.VaultSettingsOpen || s.SettingsView.Category != "sync" {
		t.Fatalf("expected transition from launcher to sync settings, launcher=%v settings=%v cat=%q",
			s.VaultLauncherOpen, s.VaultSettingsOpen, s.SettingsView.Category)
	}
	_ = components.SettingsState{}
}
